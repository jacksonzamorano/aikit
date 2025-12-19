package aikit

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type InferenceTransport string

const (
	TransportSSE InferenceTransport = "sse"
)

type ChunkResult struct {
	Done    bool
	Updated bool
	Error   error
}

func EmptyChunkResult() ChunkResult {
	return ChunkResult{
		Updated: false,
		Error:   nil,
	}
}
func DoneChunkResult() ChunkResult {
	return ChunkResult{
		Done:    true,
		Updated: false,
		Error:   nil,
	}
}
func ErrorChunkResult(err error) ChunkResult {
	return ChunkResult{
		Updated: false,
		Error:   err,
	}
}
func UpdateChunkResult() ChunkResult {
	return ChunkResult{
		Updated: true,
		Error:   nil,
	}
}

type Session struct {
	Provider InferenceProvider
	State    *ProviderState
	Debug    bool
}

func (s *Session) Stream(onPartial func(*ProviderState)) *ProviderState {
	// Perform one-off initialization
	s.Provider.InitSession(s.State)

	// Keep track of changed blocks.
	lastBlock := 0
	for {
		s.Provider.PrepareForUpdates()
		// Update blocks from last turn.
		// Will also handle tool calls synchronously.
		for lastBlock < len(s.State.Blocks) {
			switch s.State.Blocks[lastBlock].Type {
			case InferenceBlockToolCall:
				block := s.State.Blocks[lastBlock]
				res := s.State.HandleToolFunction(block.ToolCall.Name, block.ToolCall.Arguments)
				resBytes, err := json.Marshal(res)
				if err != nil {
					s.State.Success = false
					s.State.Error = &ProviderError{
						Cause: err.Error(),
					}
					return s.State
				}
				s.State.ToolResult(block.ToolCall, resBytes)
			}
			s.Provider.Update(s.State.Blocks[lastBlock])
			lastBlock++
		}

		req := s.Provider.MakeRequest(s.State)
		resp, err := http.DefaultClient.Do(req)
		if s.Debug {
			log.Printf("[Session] Request made to %s", req.URL.String())
		}
		if err != nil {
			s.State.Success = false
			s.State.Error = &ProviderError{
				Cause: err.Error(),
			}
			return s.State
		}
		if resp.StatusCode >= 300 {
			err, _ := io.ReadAll(resp.Body)
			s.State.Success = false
			s.State.Error = &ProviderError{
				Cause: fmt.Sprintf("Status %d", resp.StatusCode),
				Data:  string(err),
			}
			return s.State
		}
		if s.Debug {
			log.Printf("[Session] Response status: %s", resp.Status)
		}
		defer resp.Body.Close()
		transport := s.Provider.Transport()
		switch transport {
		case TransportSSE:
			err := readSSE(resp.Body, func(ev sseEvent) (bool, error) {
				if len(ev.data) == 0 {
					return true, nil
				}
				if string(ev.data) == "[DONE]" {
					return false, nil
				}
				if s.Debug {
					log.Printf("[Session] SSE Event: %s", string(ev.data))
				}
				result := s.Provider.OnChunk(ev.data, s.State)
				if result.Updated {
					onPartial(s.State)
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
				dbg, _ := json.MarshalIndent(s.State, "", "  ")
				log.Printf("[Session] %s", string(dbg))
			}
			if err != nil {
				s.State.Success = false
				s.State.Error = &ProviderError{
					Cause: err.Error(),
				}
				return s.State
			} else if s.State.incompleteToolCalls == 0 {
				s.State.Success = true
				return s.State
			}
		}
	}
}
