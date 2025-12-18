package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ResponsesAPI implements the Responses API shape (OpenAI-style).
type ResponsesAPI struct {
	Config          ProviderConfig
	GenerateSummary bool
}

func (p ResponsesAPI) Infer(request *InferenceRequest, onPartial func(*ProviderState)) *ProviderState {
	endpoint, err := p.Config.resolveEndpoint("/v1/responses")
	if err != nil {
		state := NewProviderState()
		state.Success = false
		state.Error = &ProviderError{
			Cause: err.Error(),
		}

		return state
	}

	state := NewProviderState()
	state.Provider = "responses"
	state.Model = request.Model

	state.System(request.SystemPrompt)
	state.Input(request.Prompt)

	payload := map[string]any{}
	if v, ok := reasoningEffortValue(request.ReasoningEffort); ok {
		payload["reasoning"] = map[string]any{
			"effort": v,
		}
		if p.GenerateSummary {
			payload["reasoning"].(map[string]any)["generate_summary"] = true
		}
	}
	payload["model"] = request.Model
	payload["instructions"] = request.SystemPrompt
	payload["stream"] = true

	tools := []map[string]any{}
	for k := range request.Tools {
		tool := map[string]any{}
		tool["description"] = request.Tools[k].Description
		tool["parameters"] = request.Tools[k].Parameters
		tool["type"] = "function"
		tool["name"] = k
		tool["strict"] = false
		tools = append(tools, tool)
	}
	if request.MaxWebSearches > 0 {
		tools = append(tools, map[string]any{
			"type": "web_search",
		})
	}
	payload["tools"] = tools

	messages := []any{
		ResponsesInput{
			Role: "user",
			Content: []ResponsesContent{
				{
					Text: request.Prompt,
					Typ:  "input_text",
				},
			},
		},
	}

	var previousId string

	for {
		payload["input"] = messages
		if previousId != "" {
			payload["previous_response_id"] = previousId
		} else {
			delete(payload, "previous_response_id")
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
				Cause: fmt.Sprintf("Status code %d", providerResp.StatusCode),
				Data:  string(raw),
			}
			return state
		}

		toolCalsThisTurn := []string{}

		streamErr := readSSE(providerResp.Body, func(ev sseEvent) error {
			if bytes.Equal(ev.data, []byte("[DONE]")) {
				return nil
			}
			if len(ev.data) == 0 {
				return nil
			}

			var data ResponsesStreamEvent
			if err := json.Unmarshal(ev.data, &data); err != nil {
				return err
			}
			data.Raw = bytes.Clone(ev.data)

			switch data.Type {
			case "response.output_text.delta":
				state.Text(data.ItemId, data.Delta)
			case "response.content_part.added":
				switch data.Type {
				case "output_text":
					state.Text(data.ItemId, data.Delta)
					for annotation := range data.Part.Annotations {
						state.Cite(data.ItemId, data.Part.Annotations[annotation].Url)
					}
				case "reasoning_text":
					state.Thinking(data.ItemId, data.Part.Text, "")
				default:
					return nil
				}
			case "response.output_item.done":
				switch data.Item.Type {
				case "function_call":
					state.ToolCall(data.Item.Id, data.Item.CallId, data.Item.Name, data.Item.Arguments)
					toolCalsThisTurn = append(toolCalsThisTurn, data.Item.Id)
				case "reasoning":
					for s := range data.Summary {
						state.Thinking(data.ItemId, data.Summary[s].Text, "")
					}
				default:
					return nil
				}
			case "response.reasoning_text.delta":
				state.Thinking(data.ItemId, data.Delta, "")
			case "response.completed":
				usage := data.Response.Usage
				state.Result.CacheReadTokens += usage.InputDetails.CachedTokens
				state.Result.InputTokens += (usage.InputTokens - usage.InputDetails.CachedTokens)
				state.Result.OutputTokens += data.Response.Usage.OutputTokens
				previousId = data.Response.Id
				return nil
			case "error":
				msg := ""
				if data.Error != nil {
					msg = data.Error.Message
				}
				if msg == "" {
					msg = string(data.Raw)
				}
				return fmt.Errorf("provider stream error: %s", msg)
			default:
				return nil
			}
			if onPartial != nil {
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

		if len(toolCalsThisTurn) > 0 {
			for _, id := range toolCalsThisTurn {
				call := *state.Get(id)
				out := request.ToolHandler(call.ToolCall.Name, call.ToolCall.Arguments)
				enc, _ := json.Marshal(out)
				state.ToolResult(call.ToolCall.ID+"-result", enc)
				if onPartial != nil {
					onPartial(state)
				}
				messages = append(messages, ResponsesToolCallResult{
					ToolCallId: call.ToolCall.ID,
					Output:     enc,
					Type:       "function_call_output",
				})
			}
			continue
		}
		state.Success = true
		return state
	}
}
