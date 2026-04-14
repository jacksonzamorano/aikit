package aikit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type GatewayTransport string

const (
	TransportSSE GatewayTransport = "sse"
)

type ChunkResult struct {
	Done  bool
	Error error
}

func AcceptedResult() ChunkResult {
	return ChunkResult{
		Error: nil,
		Done:  false,
	}
}
func DoneChunkResult() ChunkResult {
	return ChunkResult{
		Done:  true,
		Error: nil,
	}
}
func ErrorChunkResult(err *AIError) ChunkResult {
	return ChunkResult{
		Error: err,
	}
}

// Session is the consumer-facing type for interacting with an AI provider.
// It provides methods for setting up conversations, reading results,
// and streaming responses.
type Session struct {
	data     *thread
	Provider APIRequest
	Debug    bool
}

func (s *Session) streamState() *StreamState {
	return &StreamState{data: s.data}
}

// --- Config setters ---

func (s *Session) SetModel(model string) {
	s.data.model = model
}

func (s *Session) GetModel() string {
	return s.data.model
}

func (s *Session) SetReasoning(config ReasoningConfig) {
	s.data.reasoning = config
}

func (s *Session) SetMaxWebSearches(n int) {
	s.data.maxWebSearches = n
}

func (s *Session) SetWebFetchEnabled(enabled bool) {
	s.data.webFetchEnabled = enabled
}

func (s *Session) SetCoalesceTextBlocks(enabled bool) {
	s.data.coalesceTextBlocks = enabled
}

func (s *Session) SetUpdateOnFinalize(enabled bool) {
	s.data.updateOnFinalize = enabled
}

func (s *Session) SetTools(tools map[string]ToolDefinition) {
	s.data.tools = tools
}

func (s *Session) SetHandleToolFunction(fn func(name string, args string) string) {
	s.data.handleToolFunction = fn
}

// --- Consumer input methods ---

// System adds a system prompt block.
func (s *Session) System(text string) {
	b := s.data.create("", InferenceBlockSystem)
	b.Text = text
	b.Complete = true
}

// Input adds a user input text block.
func (s *Session) Input(text string) {
	b := s.data.create("", InferenceBlockInput)
	b.Text = text
	b.Complete = true
}

// InputImage adds an image using raw bytes.
func (s *Session) InputImage(imageData []byte, mediaType string) {
	b := s.data.create("", InferenceBlockInputImage)
	b.Image = &ThreadImage{
		Base64:    base64.StdEncoding.EncodeToString(imageData),
		MediaType: mediaType,
	}
	b.Complete = true
}

// InputImageBase64 adds an image using a pre-encoded base64 string.
func (s *Session) InputImageBase64(base64Data string, mediaType string) {
	b := s.data.create("", InferenceBlockInputImage)
	b.Image = &ThreadImage{
		Base64:    base64Data,
		MediaType: mediaType,
	}
	b.Complete = true
}

// --- Consumer result methods ---

// Output returns the text of the last complete text block.
func (s *Session) Output() string {
	for i := len(s.data.blocks) - 1; i >= 0; i-- {
		if s.data.blocks[i].Type == InferenceBlockText && s.data.blocks[i].Complete {
			return s.data.blocks[i].Text
		}
	}
	return ""
}

// Succeeded returns whether the session completed successfully.
func (s *Session) Succeeded() bool {
	return s.data.success
}

// ErrorMessage returns the error message if the session failed.
func (s *Session) ErrorMessage() string {
	return s.data.err
}

// Usage returns token usage statistics.
func (s *Session) Usage() ThreadUsage {
	return s.data.result
}

// GetBlocks returns the blocks in the thread.
func (s *Session) GetBlocks() []*ThreadBlock {
	return s.data.blocks
}

// IncompleteToolCalls returns the count of tool call blocks that are not yet complete.
func (s *Session) IncompleteToolCalls() int {
	count := 0
	for _, b := range s.data.blocks {
		if b.Type == InferenceBlockToolCall && !b.Complete {
			count++
		}
	}
	return count
}

// --- Tool registration ---

// RegisterTool registers a tool definition alongside its handler.
func (s *Session) RegisterTool(name string, def ToolDefinition, handler func(args string) string) {
	if s.data.tools == nil {
		s.data.tools = make(map[string]ToolDefinition)
	}
	s.data.tools[name] = def

	prev := s.data.handleToolFunction
	s.data.handleToolFunction = func(callName string, args string) string {
		if callName == name {
			return handler(args)
		}
		if prev != nil {
			return prev(callName, args)
		}
		return ""
	}
}

// --- Structured output ---

// SetStructuredOutput configures structured output for the session.
func (s *Session) SetStructuredOutput(schema *JsonSchema, strict bool) {
	s.data.structuredOutputSchema = schema
	s.data.structuredOutputStrict = &strict
}

// --- Streaming ---

func (s *Session) StreamBlocks(ctx context.Context) <-chan *ThreadBlock {
	ch := make(chan *ThreadBlock, 1)
	go func() {
		defer close(ch)

		state := s.streamState()
		s.Provider.InitSession(state)
		state.SetCurrentProvider(s.Provider.Name())

		lastBlock := 0
		completedUpTo := 0
		for {
			s.Provider.PrepareForUpdates()
			for lastBlock < len(s.data.blocks) {
				block := s.data.blocks[lastBlock]

				if block.Type == InferenceBlockThinking || block.Type == InferenceBlockEncryptedThinking {
					if block.ProviderID != "" && block.ProviderID != s.Provider.Name() {
						lastBlock++
						continue
					}
				}

				switch block.Type {
				case InferenceBlockToolCall:
					if block.ToolResult == nil {
						res := s.data.handleToolFunction(block.ToolCall.Name, block.ToolCall.Arguments)
						state.AppendToolResult(block.ToolCall, res)
					}
				}
				s.Provider.Update(block)
				lastBlock++
			}

			req := s.Provider.MakeRequest(state).WithContext(ctx)
			resp, err := http.DefaultClient.Do(req)
			if s.Debug {
				log.Printf("[Session] Request made to %s", req.URL.String())
			}
			if err != nil {
				s.data.setError(err)
				return
			}
			if resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				if parsedErr := s.Provider.ParseHttpError(resp.StatusCode, body); parsedErr != nil {
					s.data.setError(parsedErr)
				} else {
					s.data.setError(&AIError{
						Category: AIErrorCategoryHTTPStatus,
						Message:  fmt.Sprintf("Unhandled error. Received status code %d with body %s", resp.StatusCode, string(body)),
						Provider: s.Provider.Name(),
					})
				}
				return
			}
			if s.Debug {
				log.Printf("[Session] Response status: %s", resp.Status)
			}
			defer resp.Body.Close()
			transport := s.Provider.Transport()
			switch transport {
			case TransportSSE:
				err := readSSE(s.Provider.Name(), resp.Body, func(ev sseEvent) (bool, error) {
					if len(ev.data) == 0 {
						return true, nil
					}
					if string(ev.data) == "[DONE]" {
						return false, nil
					}
					if s.Debug {
						log.Printf("[Session] SSE Event: %s", string(ev.data))
					}
					result := s.Provider.OnChunk(ev.data, state)
					state.TakeUpdate()
					for completedUpTo < len(s.data.blocks) {
						if s.data.blocks[completedUpTo].Complete {
							ch <- s.data.blocks[completedUpTo]
							completedUpTo++
						} else {
							break
						}
					}
					if result.Error != nil {
						return false, result.Error
					}
					if result.Done {
						return false, nil
					}
					return true, nil
				})
				if s.Debug {
					dbg, _ := json.MarshalIndent(s.data, "", "  ")
					log.Printf("[Session] %s", string(dbg))
				}
				if err != nil {
					s.data.setError(err)
					return
				} else if state.IncompleteToolCalls() == 0 {
					s.data.success = true
					return
				}
			}
		}
	}()
	return ch
}

func (s *Session) Stream(ctx context.Context) <-chan *Session {
	ch := make(chan *Session, 1)
	go func() {
		defer close(ch)

		state := s.streamState()
		s.Provider.InitSession(state)
		state.SetCurrentProvider(s.Provider.Name())

		lastBlock := 0
		for {
			s.Provider.PrepareForUpdates()
			for lastBlock < len(s.data.blocks) {
				block := s.data.blocks[lastBlock]

				if block.Type == InferenceBlockThinking || block.Type == InferenceBlockEncryptedThinking {
					if block.ProviderID != "" && block.ProviderID != s.Provider.Name() {
						lastBlock++
						continue
					}
				}

				switch block.Type {
				case InferenceBlockToolCall:
					if block.ToolResult == nil {
						res := s.data.handleToolFunction(block.ToolCall.Name, block.ToolCall.Arguments)
						state.AppendToolResult(block.ToolCall, res)
					}
				}
				s.Provider.Update(block)
				lastBlock++
			}

			req := s.Provider.MakeRequest(state).WithContext(ctx)
			resp, err := http.DefaultClient.Do(req)
			if s.Debug {
				log.Printf("[Session] Request made to %s", req.URL.String())
			}
			if err != nil {
				s.data.setError(err)
				ch <- s
				return
			}
			if resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				if parsedErr := s.Provider.ParseHttpError(resp.StatusCode, body); parsedErr != nil {
					s.data.setError(parsedErr)
				} else {
					s.data.setError(&AIError{
						Category: AIErrorCategoryHTTPStatus,
						Message:  fmt.Sprintf("Unhandled error. Received status code %d with body %s", resp.StatusCode, string(body)),
						Provider: s.Provider.Name(),
					})
				}
				ch <- s
				return
			}
			if s.Debug {
				log.Printf("[Session] Response status: %s", resp.Status)
			}
			defer resp.Body.Close()
			transport := s.Provider.Transport()
			switch transport {
			case TransportSSE:
				err := readSSE(s.Provider.Name(), resp.Body, func(ev sseEvent) (bool, error) {
					if len(ev.data) == 0 {
						return true, nil
					}
					if string(ev.data) == "[DONE]" {
						return false, nil
					}
					if s.Debug {
						log.Printf("[Session] SSE Event: %s", string(ev.data))
					}
					result := s.Provider.OnChunk(ev.data, state)
					if state.TakeUpdate() {
						ch <- s
					}
					if result.Error != nil {
						return false, result.Error
					}
					if result.Done {
						return false, nil
					}
					return true, nil
				})
				if s.Debug {
					dbg, _ := json.MarshalIndent(s.data, "", "  ")
					log.Printf("[Session] %s", string(dbg))
				}
				if err != nil {
					s.data.setError(err)
					ch <- s
					return
				} else if state.IncompleteToolCalls() == 0 {
					s.data.success = true
					ch <- s
					return
				}
			}
		}
	}()
	return ch
}
