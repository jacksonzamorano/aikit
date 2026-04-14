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

// stream is the unified internal streaming pipeline. It returns a channel
// of StreamEvent values covering block lifecycle, errors, and completion.
func (s *Session) stream(ctx context.Context) <-chan StreamEvent {
	ch := make(chan StreamEvent, 1)
	go func() {
		defer close(ch)

		emit := func(kind StreamEventKind, block *ThreadBlock, err error) {
			ch <- StreamEvent{
				Kind:    kind,
				Block:   block,
				Session: s,
				Error:   err,
			}
		}

		state := s.streamState()
		s.Provider.InitSession(state)
		state.SetCurrentProvider(s.Provider.Name())

		lastBlock := 0
		prevBlockCount := len(s.data.blocks)
		completedSet := make(map[int]bool)
		for i, b := range s.data.blocks {
			if b.Complete {
				completedSet[i] = true
			}
		}

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
				emit(EventError, nil, err)
				return
			}
			if resp.StatusCode >= 300 {
				body, _ := io.ReadAll(resp.Body)
				var apiErr error
				if parsedErr := s.Provider.ParseHttpError(resp.StatusCode, body); parsedErr != nil {
					s.data.setError(parsedErr)
					apiErr = parsedErr
				} else {
					httpErr := &AIError{
						Category: AIErrorCategoryHTTPStatus,
						Message:  fmt.Sprintf("Unhandled error. Received status code %d with body %s", resp.StatusCode, string(body)),
						Provider: s.Provider.Name(),
					}
					s.data.setError(httpErr)
					apiErr = httpErr
				}
				emit(EventError, nil, apiErr)
				return
			}
			if s.Debug {
				log.Printf("[Session] Response status: %s", resp.Status)
			}
			defer resp.Body.Close()
			transport := s.Provider.Transport()
			switch transport {
			case TransportSSE:
				sseErr := readSSE(s.Provider.Name(), resp.Body, func(ev sseEvent) (bool, error) {
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
					updated := state.TakeUpdate()

					// Detect new blocks
					emittedAny := false
					for i := prevBlockCount; i < len(s.data.blocks); i++ {
						emit(EventBlockNew, s.data.blocks[i], nil)
						emittedAny = true
						if s.data.blocks[i].Complete {
							completedSet[i] = true
						}
					}
					prevBlockCount = len(s.data.blocks)

					// Detect newly completed blocks
					for i := range s.data.blocks {
						if s.data.blocks[i].Complete && !completedSet[i] {
							emit(EventBlockCompleted, s.data.blocks[i], nil)
							completedSet[i] = true
							emittedAny = true
						}
					}

					// If something updated but no new/completed events, emit update
					if updated && !emittedAny {
						// Find the last non-complete block as the one being updated
						for i := len(s.data.blocks) - 1; i >= 0; i-- {
							if !s.data.blocks[i].Complete {
								emit(EventBlockUpdated, s.data.blocks[i], nil)
								break
							}
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
				if sseErr != nil {
					s.data.setError(sseErr)
					emit(EventError, nil, sseErr)
					return
				} else if state.IncompleteToolCalls() == 0 {
					s.data.success = true
					emit(EventDone, nil, nil)
					return
				}
			}
		}
	}()
	return ch
}

// StreamAll returns a channel of all streaming events. Callers range over
// the channel and switch on ev.Kind to handle different event types.
func (s *Session) StreamAll(ctx context.Context) <-chan StreamEvent {
	return s.stream(ctx)
}

// Send runs the streaming pipeline to completion.
// It blocks until the stream finishes or an error occurs.
func (s *Session) Send(ctx context.Context) {
	for range s.stream(ctx) {
	}
}

// StreamBlocks returns a channel that emits each ThreadBlock as it completes.
func (s *Session) StreamBlocks(ctx context.Context) <-chan *ThreadBlock {
	ch := make(chan *ThreadBlock, 1)
	go func() {
		defer close(ch)
		for ev := range s.stream(ctx) {
			if ev.Kind == EventBlockCompleted && ev.Block != nil {
				ch <- ev.Block
			}
		}
	}()
	return ch
}
