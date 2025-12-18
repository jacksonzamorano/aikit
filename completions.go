package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type CompletionsAPI struct {
	Config ProviderConfig
}

func (p CompletionsAPI) Infer(request *InferenceRequest, onPartial func(*ProviderState)) *ProviderState {
	endpoint, err := p.Config.resolveEndpoint("/v1/chat/completions")
	if err != nil {
		state := NewProviderState()
		state.Success = false
		state.Error = &ProviderError{
			Cause: err.Error(),
		}
		return state
	}
	state := NewProviderState()
	state.Provider = "completions"
	state.Model = request.Model

	state.System(request.SystemPrompt)
	state.Input(request.Prompt)

	tools := make([]map[string]any, 0, len(request.Tools)+1)
	for name := range request.Tools {
		toolSpec := map[string]any{}
		toolSpec["description"] = request.Tools[name].Description
		toolSpec["parameters"] = request.Tools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, map[string]any{
			"type":     "function",
			"function": toolSpec,
		})
	}
	if request.MaxWebSearches > 0 {
		tools = append(tools, map[string]any{"type": "web_search"})
	}

	messages := []CompletionsMessage{}
	if request.SystemPrompt != "" {
		messages = append(messages, CompletionsMessage{
			Role: "system",
			Content: []CompletionTextBlock{
				{Type: "text", Text: request.SystemPrompt},
			},
		})
	}
	messages = append(messages, CompletionsMessage{
		Role: "user",
		Content: []CompletionTextBlock{
			{Type: "text", Text: request.Prompt},
		},
	})

	turn := 0
	lastTurnEndsAt := 0
	for {
		turn++

		payload := map[string]any{}
		if v, ok := reasoningEffortValue(request.ReasoningEffort); ok {
			payload["reasoning_effort"] = v
		}
		payload["model"] = request.Model
		payload["messages"] = messages
		payload["stream"] = true
		payload["stream_options"] = map[string]any{
			"include_usage": true,
		}

		if len(tools) > 0 {
			payload["tools"] = tools
		}

		body, _ := json.Marshal(payload)
		providerReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
		if err != nil {
			state.Success = false
			state.Error = &ProviderError{
				Cause: err.Error(),
			}
			return state
		}
		providerReq.Header.Add("Content-Type", "application/json")
		providerReq.Header.Add("Accept", "text/event-stream")
		providerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.Config.APIKey))

		providerResp, err := p.Config.httpClient().Do(providerReq)
		if err != nil {
			state.Success = false
			state.Error = &ProviderError{
				Cause: err.Error(),
			}
			return state
		}
		if providerResp.StatusCode < 200 || providerResp.StatusCode > 299 {
			raw, _ := io.ReadAll(providerResp.Body)
			providerResp.Body.Close()
			state.Success = false
			state.Error = &ProviderError{
				Data:  string(raw),
				Cause: fmt.Sprintf("Status code %d", providerResp.StatusCode),
			}
			return state
		}

		var toolCalls []string = []string{}
		var lastTool string = ""

		streamErr := readSSE(providerResp.Body, func(ev sseEvent) error {
			if bytes.Equal(ev.data, []byte("[DONE]")) {
				return nil
			}
			if len(ev.data) == 0 {
				return nil
			}

			dirty := false

			var chunk CompletionsStreamChunk
			if err := json.Unmarshal(ev.data, &chunk); err != nil {

			}

			if chunk.Usage != nil {
				nonCachedInput := max(chunk.Usage.PromptTokens-chunk.Usage.PromptTokenDetails.CachedTokens, 0)
				state.Result.InputTokens += nonCachedInput
				state.Result.OutputTokens += chunk.Usage.CompletionTokens
				state.Result.CacheReadTokens += chunk.Usage.PromptTokenDetails.CachedTokens
			}

			if len(chunk.Choices) == 0 {
				return nil
			}

			choice := chunk.Choices[0]

			if choice.Delta.ReasoningContent != "" {
				state.Thinking(fmt.Sprintf("thinking-%d", turn), choice.Delta.ReasoningContent, "")
				dirty = true
			}
			if choice.Delta.Content != "" {
				state.Text(fmt.Sprintf("text-%d", turn), choice.Delta.Content)
				dirty = true
			}

			for i := range choice.Delta.ToolCalls {
				tc := choice.Delta.ToolCalls[i]
				id := tc.Function.Name
				if len(id) == 0 {
					id = lastTool
				} else {
					// New tool call
					lastTool = id
					toolCalls = append(toolCalls, id)
				}

				state.ToolCall(id, id, tc.Function.Name, tc.Function.Arguments)
				dirty = true
			}
			if dirty && onPartial != nil {
				onPartial(state)
			}
			return nil
		})
		providerResp.Body.Close()
		if streamErr != nil {
			state.Success = false
			state.Error = streamErr
			return state
		}

		for lastTurnEndsAt < len(state.Blocks) {
			block := state.Blocks[lastTurnEndsAt]
			switch block.Type {
			case InferenceBlockText:
				messages = append(messages, CompletionsMessage{
					Role: "assistant",
					Content: []CompletionTextBlock{
						{Type: "text", Text: block.Text},
					},
				})
			}
			lastTurnEndsAt++
		}

		if len(toolCalls) > 0 {
			for i := range toolCalls {
				tc := *state.Get(toolCalls[i])
				messages = append(messages, CompletionsMessage{
					Role: "assistant",
					ToolCalls: []CompletionsToolCall{{
						Id:   tc.ID,
						Type: "function",
						Function: &CompletionsToolCallFunction{
							Name:      tc.ToolCall.Name,
							Arguments: tc.ToolCall.Arguments,
						},
					}},
				})
				out := request.ToolHandler(tc.ToolCall.Name, tc.ToolCall.Arguments)
				enc, _ := json.Marshal(out)
				state.ToolResult(tc.ToolCall.ID, enc)
				if onPartial != nil {
					onPartial(state)
				}
				messages = append(messages, CompletionsMessage{
					Role: "tool",
					Content: []CompletionTextBlock{
						{Type: "text", Text: string(enc)},
					},
					ToolCallId: tc.ID,
				})
			}
			continue
		}

		state.Success = true
		return state
	}
}
