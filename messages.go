package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// MessagesAPI implements the Messages API shape (Anthropic-style).
// Configurable BaseURL/Endpoint allows pointing at compatible endpoints.
type MessagesAPI struct {
	Config  ProviderConfig
	Request MessagesRequest
	Version string

	lastToolCallID string
}

func (p *MessagesAPI) Transport() InferenceTransport {
	return TransportSSE
}

func (p *MessagesAPI) PrepareForUpdates() {}

func (p *MessagesAPI) InitSession(state *ProviderState) {
	tools := make([]map[string]any, 0)
	for name := range state.Tools {
		toolSpec := map[string]any{}
		toolSpec["description"] = state.Tools[name].Description
		toolSpec["input_schema"] = state.Tools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, toolSpec)
	}

	// if request.MaxWebSearches > 0 {
	// 	tools = append(tools, map[string]any{
	// 		"type":     "web_search_20250305",
	// 		"name":     "web_search",
	// 		"max_uses": request.MaxWebSearches,
	// 	})
	// }

	p.Request = MessagesRequest{
		Messages:  []MessagesMessage{},
		Model:     state.Model,
		Tools:     tools,
		MaxTokens: 10_000,
		Stream:    true,
	}

	if len(state.ReasoningEffort) > 0 {
		if parsed, err := strconv.Atoi(state.ReasoningEffort); err == nil {
			budgetTokens := int64(parsed)
			p.Request.Thinking = &MessagesThinking{
				BudgetTokens: budgetTokens,
				Type:         "enabled",
			}
		}
	}
}

func (p *MessagesAPI) Update(block *InferenceBlock) {
	switch block.Type {
	case InferenceBlockInput:
		p.Request.Messages = append(p.Request.Messages, MessagesMessage{
			Role: "user",
			Content: []MessagesContent{
				{
					Type: "text",
					Text: block.Text,
					CacheControl: &MessagesCacheControl{
						Type: "ephemeral",
					},
				},
			},
		})
	case InferenceBlockText:
		p.Request.Messages = append(p.Request.Messages, MessagesMessage{
			Role: "assistant",
			Content: []MessagesContent{
				{
					Type: "text",
					Text: block.Text,
				},
			},
		})
	case InferenceBlockThinking:
		p.Request.Messages = append(p.Request.Messages, MessagesMessage{
			Role: "assistant",
			Content: []MessagesContent{
				{
					Type:      "thinking",
					Thinking:  block.Text,
					Signature: block.Signature,
				},
			},
		})
	case InferenceBlockEncryptedThinking:
		p.Request.Messages = append(p.Request.Messages, MessagesMessage{
			Role: "assistant",
			Content: []MessagesContent{
				{
					Type: "redacted_thinking",
					Data: block.Text,
				},
			},
		})
	case InferenceBlockToolCall:
		p.Request.Messages = append(p.Request.Messages, MessagesMessage{
			Role: "assistant",
			Content: []MessagesContent{
				{
					Type:  "tool_use",
					Name:  block.ToolCall.Name,
					Id:    block.ToolCall.ID,
					Input: block.ToolCall.Arguments,
				},
			},
		})
	case InferenceBlockToolResult:
		fmt, _ := json.Marshal(block.ToolResult.Output)
		p.Request.Messages = append(p.Request.Messages, MessagesMessage{
			Role: "user",
			Content: []MessagesContent{
				{
					Type:      "tool_result",
					Content:   fmt,
					ToolUseId: block.ToolCall.ID,
				},
			},
		})

	}
}

func (p *MessagesAPI) MakeRequest(state *ProviderState) *http.Request {
	endpoint := p.Config.resolveEndpoint("/v1/messages")
	body, _ := json.Marshal(p.Request)
	providerReq, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	providerReq.Header.Add("Content-Type", "application/json")
	providerReq.Header.Add("Accept", "text/event-stream")
	if p.Version == "" {
		p.Version = "2023-06-01"
	}
	providerReq.Header.Add("anthropic-version", p.Version)
	providerReq.Header.Add("x-api-key", p.Config.APIKey)
	return providerReq
}

func (p *MessagesAPI) OnChunk(data []byte, state *ProviderState) ChunkResult {
	dirty := false

	var env MessagesStreamEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return ErrorChunkResult(err)
	}

	switch env.Type {
	case "ping":
		return EmptyChunkResult()
	case "error":
		var e MessagesStreamErrorEvent
		if err := json.Unmarshal(data, &e); err == nil && e.Error != nil && e.Error.Message != "" {
			return ErrorChunkResult(fmt.Errorf("provider stream error: %s", e.Error.Message))
		} else {
			return ErrorChunkResult(fmt.Errorf("provider stream error: %s", string(data)))
		}
	case "message_start":
		var ms MessagesStreamMessageStart
		if err := json.Unmarshal(data, &ms); err == nil {
			if state.ResponseID == "" && ms.Message.ID != "" {
				state.ResponseID = ms.Message.ID
			}
			usage := ms.Message.Usage
			state.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
			state.Result.CacheReadTokens += usage.CacheReadInputTokens
			state.Result.CacheWriteTokens += usage.CacheCreationInputTokens
			state.Result.OutputTokens += usage.OutputTokens
			dirty = true
		}
	case "message_delta":
		var md MessagesStreamMessageDelta
		if err := json.Unmarshal(data, &md); err == nil {
			usage := md.Usage
			state.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
			state.Result.OutputTokens += usage.OutputTokens
			state.Result.CacheReadTokens += usage.CacheReadInputTokens
			state.Result.CacheWriteTokens += usage.CacheCreationInputTokens
			dirty = true
		}
	case "content_block_start":
		var cbs MessagesStreamContentBlockStart
		if err := json.Unmarshal(data, &cbs); err != nil {
			return ErrorChunkResult(err)
		}
		switch cbs.ContentBlock.Type {
		case "thinking":
			state.Thinking(cbs.ContentBlock.Thinking, cbs.ContentBlock.Signature)
		case "redacted_thinking":
			state.EncryptedThinking(cbs.ContentBlock.Data)
		case "tool_use":
			state.ToolCall(cbs.ContentBlock.ID, cbs.ContentBlock.ID, cbs.ContentBlock.Name, cbs.ContentBlock.Input)
			p.lastToolCallID = cbs.ContentBlock.ID
		case "text":
			state.Text(cbs.ContentBlock.Text)
		}
		dirty = true
	case "content_block_delta":
		var cbd MessagesStreamContentDelta
		if err := json.Unmarshal(data, &cbd); err != nil {
			return ErrorChunkResult(err)
		}

		switch cbd.Delta.Type {
		case "text_delta":
			state.Text(cbd.Delta.Text)
		case "thinking_delta":
			state.Thinking(cbd.Delta.Thinking, "")
		case "signature_delta":
			state.Thinking("", cbd.Delta.Signature)
		case "input_json_delta":
			state.ToolCall(p.lastToolCallID, p.lastToolCallID, "", nil)
		}
		dirty = true
	case "content_block_stop":
		var cbst MessagesStreamContentBlockStop
		if err := json.Unmarshal(data, &cbst); err != nil {
			return ErrorChunkResult(err)
		}
		dirty = true
	case "message_stop":
		return DoneChunkResult()
	}
	if dirty {
		return UpdateChunkResult()
	}
	return EmptyChunkResult()
}

// func (p MessagesAPI) Infer(request *InferenceRequest, onPartial func(*ProviderState)) *ProviderState {
// 	endpoint, err := p.Config.resolveEndpoint("/v1/messages")
// 	if err != nil {
// 		state := NewProviderState()
// 		state.Success = false
// 		state.Error = &ProviderError{
// 			Cause: err.Error(),
// 		}
// 		return state
// 	}
// 	state := NewProviderState()
// 	state.Provider = "messages"
// 	state.Model = request.Model
//
// 	state.System(request.SystemPrompt)
// 	state.Input(request.Prompt)
//
// 	var payload MessagesRequest
// 	payload.MaxTokens = 10_000
// 	payload.Model = request.Model
// 	payload.System = request.SystemPrompt
//
// 	tools := make([]map[string]any, 0, len(request.Tools)+1)
// 	for k := range request.Tools {
// 		tool := map[string]any{}
// 		tool["name"] = k
// 		tool["input_schema"] = request.Tools[k].Parameters
// 		tool["description"] = request.Tools[k].Description
// 		delete(tool, "parameters")
// 		tools = append(tools, tool)
// 	}
// 	if request.MaxWebSearches > 0 {
// 		tools = append(tools, map[string]any{
// 			"type":     "web_search_20250305",
// 			"name":     "web_search",
// 			"max_uses": request.MaxWebSearches,
// 		})
// 	}
// 	payload.Tools = tools
//
// 	if request.ReasoningEffort != nil {
// 		if v, ok := reasoningEffortValue(request.ReasoningEffort); ok {
// 			budgetTokens := int64(2048)
// 			if parsed, err := strconv.Atoi(v); err == nil {
// 				budgetTokens = int64(parsed)
// 			}
// 			payload.Thinking = MessagesThinking{
// 				BudgetTokens: budgetTokens,
// 				Type:         "enabled",
// 			}
// 		} else {
// 			payload.Thinking = MessagesThinking{Type: "disabled"}
// 		}
// 	} else {
// 		payload.Thinking = MessagesThinking{Type: "disabled"}
// 	}
//
// 	messages := []MessagesMessage{
// 		{
// 			Role: "user",
// 			Content: []MessagesContent{
// 				{
// 					Type: "text",
// 					Text: request.Prompt,
// 					CacheControl: &MessagesCacheControl{
// 						Type: "ephemeral",
// 					},
// 				},
// 			},
// 		},
// 	}
//
// 	version := p.Version
// 	if version == "" {
// 		version = "2023-06-01"
// 	}
//
// 	count := 0
// 	for {
// 		toolsThisTurn := false
// 		count += 1
// 		payload.Messages = messages
// 		streamPayload := messagesStreamRequest{MessagesRequest: payload, Stream: true}
// 		body, _ := json.Marshal(streamPayload)
// 		providerReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
// 		if err != nil {
// 			state.Success = false
// 			state.Error = &ProviderError{
// 				Cause: err.Error(),
// 			}
// 			return state
// 		}
// 		providerReq.Header.Add("Content-Type", "application/json")
// 		providerReq.Header.Add("Accept", "text/event-stream")
// 		providerReq.Header.Add("anthropic-version", version)
// 		providerReq.Header.Add("x-api-key", p.Config.APIKey)
//
// 		providerResp, err := p.Config.httpClient().Do(providerReq)
// 		if err != nil {
// 			state.Success = false
// 			state.Error = &ProviderError{
// 				Cause: err.Error(),
// 			}
// 			return state
// 		}
// 		if providerResp.StatusCode < 200 || providerResp.StatusCode > 299 {
// 			raw, _ := io.ReadAll(providerResp.Body)
// 			providerResp.Body.Close()
// 			state.Success = false
// 			state.Error = &ProviderError{
// 				Cause: fmt.Sprintf("Status code %d", providerResp.StatusCode),
// 				Data:  string(raw),
// 			}
// 			return state
// 		}
//
// 		streamErr := readSSE(providerResp.Body, func(ev sseEvent) error {
// 			if len(ev.data) == 0 {
// 				return nil
// 			}
//
// 			var env MessagesStreamEnvelope
// 			if err := json.Unmarshal(ev.data, &env); err != nil {
// 				return err
// 			}
//
// 			switch env.Type {
// 			case "ping":
// 				return nil
// 			case "error":
// 				var e MessagesStreamErrorEvent
// 				if err := json.Unmarshal(ev.data, &e); err == nil && e.Error != nil && e.Error.Message != "" {
// 					return fmt.Errorf("provider stream error: %s", e.Error.Message)
// 				} else {
// 					return fmt.Errorf("provider stream error: %s", string(ev.data))
// 				}
// 			case "message_start":
// 				var ms MessagesStreamMessageStart
// 				if err := json.Unmarshal(ev.data, &ms); err == nil {
// 					if state.ResponseID == "" && ms.Message.ID != "" {
// 						state.ResponseID = ms.Message.ID
// 					}
// 					usage := ms.Message.Usage
// 					state.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
// 					state.Result.CacheReadTokens += usage.CacheReadInputTokens
// 					state.Result.CacheWriteTokens += usage.CacheCreationInputTokens
// 					state.Result.OutputTokens += usage.OutputTokens
// 					if onPartial != nil {
// 						onPartial(state)
// 					}
// 				}
// 			case "message_delta":
// 				var md MessagesStreamMessageDelta
// 				if err := json.Unmarshal(ev.data, &md); err == nil {
// 					usage := md.Usage
// 					state.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
// 					state.Result.OutputTokens += usage.OutputTokens
// 					state.Result.CacheReadTokens += usage.CacheReadInputTokens
// 					state.Result.CacheWriteTokens += usage.CacheCreationInputTokens
// 					if onPartial != nil {
// 						onPartial(state)
// 					}
// 				}
// 			case "content_block_start":
// 				var cbs MessagesStreamContentBlockStart
// 				if err := json.Unmarshal(ev.data, &cbs); err != nil {
// 					return err
// 				}
// 				idx := cbs.Index
// 				switch cbs.ContentBlock.Type {
// 				case "thinking":
// 					state.Thinking(messagesBlockId(count, idx), cbs.ContentBlock.Thinking, cbs.ContentBlock.Signature)
// 				case "redacted_thinking":
// 					state.Thinking(messagesBlockId(count, idx), cbs.ContentBlock.Data, "")
// 				case "tool_use":
// 					state.ToolCall(messagesBlockId(count, idx), cbs.ContentBlock.ID, cbs.ContentBlock.Name, cbs.ContentBlock.Input)
// 				case "text":
// 					state.Text(messagesBlockId(count, idx), cbs.ContentBlock.Text)
// 				}
// 				if onPartial != nil {
// 					onPartial(state)
// 				}
// 			case "content_block_delta":
// 				var cbd MessagesStreamContentBlockDelta
// 				if err := json.Unmarshal(ev.data, &cbd); err != nil {
// 					return err
// 				}
//
// 				idx := cbd.Index
// 				switch cbd.Type {
// 				case "thinking":
// 					state.Thinking(messagesBlockId(count, idx), cbd.Delta.Thinking, cbd.Delta.Signature)
// 				case "text":
// 					state.Text(messagesBlockId(count, idx), cbd.Delta.Text)
// 				}
// 				if onPartial != nil {
// 					onPartial(state)
// 				}
// 			case "content_block_stop":
// 				var cbst MessagesStreamContentBlockStop
// 				if err := json.Unmarshal(ev.data, &cbst); err != nil {
// 					return err
// 				}
// 				block := *state.Get(messagesBlockId(count, cbst.Index))
// 				switch block.Type {
// 				case "tool_use":
// 					toolsThisTurn = true
// 					messages = append(messages, MessagesMessage{
// 						Role: "assistant",
// 						Content: []MessagesContent{
// 							{
// 								Type:  "tool_use",
// 								Name:  block.ToolCall.Name,
// 								Id:    block.ToolCall.ID,
// 								Input: block.ToolCall.Arguments,
// 							},
// 						},
// 					})
// 					if onPartial != nil {
// 						onPartial(state)
// 					}
// 					returnValue := request.ToolHandler(block.ToolCall.Name, block.ToolCall.Arguments)
// 					enc, _ := json.Marshal(returnValue)
// 					messages = append(messages, MessagesMessage{
// 						Role: "user",
// 						Content: []MessagesContent{
// 							{
// 								Type:      "tool_result",
// 								Content:   enc,
// 								ToolUseId: block.ID,
// 							},
// 						},
// 					})
// 					state.ToolResult(
// 						block.ID+"-result",
// 						enc,
// 					)
// 					if onPartial != nil {
// 						onPartial(state)
// 					}
// 				case "text":
// 					messages = append(messages, MessagesMessage{
// 						Role: "assistant",
// 						Content: []MessagesContent{
// 							{
// 								Type: "text",
// 								Text: block.Text,
// 							},
// 						},
// 					})
// 				case "thinking":
// 					if len(block.Text) > 0 {
// 						messages = append(messages, MessagesMessage{
// 							Role: "assistant",
// 							Content: []MessagesContent{
// 								{
// 									Type:      "thinking",
// 									Thinking:  block.Text,
// 									Signature: block.Signature,
// 								},
// 							},
// 						})
// 					}
// 				case "redacted_thinking":
// 					messages = append(messages, MessagesMessage{
// 						Role: "assistant",
// 						Content: []MessagesContent{
// 							{
// 								Type: "redacted_thinking",
// 								Data: block.Text,
// 							},
// 						},
// 					})
// 				}
// 			case "message_stop":
// 				return nil
// 			}
// 			return nil
// 		})
// 		providerResp.Body.Close()
// 		if streamErr != nil {
// 			state.Success = false
// 			state.Error = streamErr
// 			return state
// 		}
// 		if !toolsThisTurn {
// 			state.Success = true
// 			return state
// 		}
// 	}
// }
