package aikit

import (
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

type Session struct {
	Provider APIRequest
	Thread    *Thread
	Debug    bool
}

func CreateResponsesSession(config *ProviderConfig) *Session {
	return &Session{
		Thread: NewProviderState(),
		Provider: &ResponsesAPIRequest{
			Config: config,
		},
	}
}
func CreateMessagesSession(config *ProviderConfig) *Session {
	return &Session{
		Thread: NewProviderState(),
		Provider: &MessagesAPIRequest{
			Config: config,
		},
	}
}
func CreateCompletionsSession(config *ProviderConfig) *Session {
	return &Session{
		Thread: NewProviderState(),
		Provider: &CompletionsAPIRequest{
			Config: config,
		},
	}
}
func CreateAIStudioSession(config *ProviderConfig) *Session {
	return &Session{
		Thread: NewProviderState(),
		Provider: &AIStudioAPIRequest{
			Config: config,
		},
	}
}

func (s *Session) Stream(onPartial func(*Thread)) *Thread {
	// Perform one-off initialization
	s.Provider.InitSession(s.Thread)
	s.Thread.CurrentProvider = s.Provider.Name()

	// Keep track of changed blocks.
	lastBlock := 0
	for {
		s.Provider.PrepareForUpdates()
		// Update blocks from last turn.
		// Will also handle tool calls synchronously.
		for lastBlock < len(s.Thread.Blocks) {
			block := s.Thread.Blocks[lastBlock]

			// Skip thinking blocks from different providers
			if block.Type == InferenceBlockThinking || block.Type == InferenceBlockEncryptedThinking {
				if block.ProviderID != "" && block.ProviderID != s.Provider.Name() {
					lastBlock++
					continue
				}
			}

			switch block.Type {
			case InferenceBlockToolCall:
				// Only execute tool if it doesn't already have a result (for restored sessions)
				if block.ToolResult == nil {
					res := s.Thread.HandleToolFunction(block.ToolCall.Name, block.ToolCall.Arguments)
					s.Thread.ToolResult(block.ToolCall, res)
				}
			}
			s.Provider.Update(block)
			lastBlock++
		}

		req := s.Provider.MakeRequest(s.Thread)
		resp, err := http.DefaultClient.Do(req)
		if s.Debug {
			log.Printf("[Session] Request made to %s", req.URL.String())
		}
		if err != nil {
			s.Thread.SetError(err)
			return s.Thread
		}
		if resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			if parsedErr := s.Provider.ParseHttpError(resp.StatusCode, body); parsedErr != nil {
				s.Thread.SetError(parsedErr)
			} else {
				s.Thread.SetError(&AIError{
					Category: AIErrorCategoryHTTPStatus,
					Message:  fmt.Sprintf("Unhandled error. Received status code %d with body %s", resp.StatusCode, string(body)),
					Provider: s.Provider.Name(),
				})
			}
			return s.Thread
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
				result := s.Provider.OnChunk(ev.data, s.Thread)
				if s.Thread.TakeUpdate() {
					onPartial(s.Thread)
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
				dbg, _ := json.MarshalIndent(s.Thread, "", "  ")
				log.Printf("[Session] %s", string(dbg))
			}
			if err != nil {
				s.Thread.SetError(err)
				return s.Thread
			} else if s.Thread.IncompleteToolCalls() == 0 {
				s.Thread.Success = true
				return s.Thread
			}
		}
	}
}
