package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type messagesLastToolCall struct {
	ID       string
	IsServer bool
	Buffer   string
	ToolName string
}

// MessagesAPIRequest implements the Messages API shape (Anthropic-style).
// Configurable BaseURL/Endpoint allows pointing at compatible endpoints.
type MessagesAPIRequest struct {
	Config *ProviderConfig

	request      MessagesRequest
	lastToolCall messagesLastToolCall
}

func (p *MessagesAPIRequest) blockId(thread *Thread, index int) string {
	return fmt.Sprintf("%s.%d", thread.ThreadId, index)
}

func (p *MessagesAPIRequest) Name() string {
	return fmt.Sprintf("messages.%s", p.Config.Name)
}

func (p *MessagesAPIRequest) Transport() GatewayTransport {
	return TransportSSE
}

func (p *MessagesAPIRequest) PrepareForUpdates() {}

func (p *MessagesAPIRequest) InitSession(thread *Thread) {
	tools := make([]map[string]any, 0)
	for name := range thread.Tools {
		toolSpec := map[string]any{}
		toolSpec["description"] = thread.Tools[name].Description
		toolSpec["input_schema"] = thread.Tools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, toolSpec)
	}

	if thread.MaxWebSearches > 0 && p.Config.WebSearchToolName != "" {
		tools = append(tools, map[string]any{
			"type":     p.Config.WebSearchToolName,
			"name":     "web_search",
			"max_uses": thread.MaxWebSearches,
		})
	}
	if thread.WebFetchEnabled && p.Config.WebFetchToolName != "" {
		tools = append(tools, map[string]any{
			"type": p.Config.WebFetchToolName,
			"name": "web_fetch",
		})
	}

	p.request = MessagesRequest{
		Messages:  []MessagesMessage{},
		Model:     thread.Model,
		Tools:     tools,
		MaxTokens: 10_000,
		Stream:    true,
	}

	if len(thread.ReasoningEffort) > 0 {
		if parsed, err := strconv.Atoi(thread.ReasoningEffort); err == nil {
			budgetTokens := int64(parsed)
			p.request.Thinking = &MessagesThinking{
				BudgetTokens: budgetTokens,
				Type:         "enabled",
			}
		}
	}
}

func (p *MessagesAPIRequest) Update(block *ThreadBlock) {
	switch block.Type {
	case InferenceBlockInput:
		p.request.Messages = append(p.request.Messages, MessagesMessage{
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
		p.request.Messages = append(p.request.Messages, MessagesMessage{
			Role: "assistant",
			Content: []MessagesContent{
				{
					Type: "text",
					Text: block.Text,
				},
			},
		})
	case InferenceBlockThinking:
		p.request.Messages = append(p.request.Messages, MessagesMessage{
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
		p.request.Messages = append(p.request.Messages, MessagesMessage{
			Role: "assistant",
			Content: []MessagesContent{
				{
					Type: "redacted_thinking",
					Data: block.Text,
				},
			},
		})
	case InferenceBlockToolCall:
		p.request.Messages = append(p.request.Messages, MessagesMessage{
			Role: "assistant",
			Content: []MessagesContent{
				{
					Type:  "tool_use",
					Name:  block.ToolCall.Name,
					Id:    block.ToolCall.ID,
					Input: []byte(block.ToolCall.Arguments),
				},
			},
		})
		if block.ToolResult != nil {
			p.request.Messages = append(p.request.Messages, MessagesMessage{
				Role: "user",
				Content: []MessagesContent{
					{
						Type:      "tool_result",
						Content:   []byte(block.ToolResult.Output),
						ToolUseId: block.ToolCall.ID,
					},
				},
			})
		}
	}
}

func (p *MessagesAPIRequest) MakeRequest(thread *Thread) *http.Request {
	endpoint := p.Config.resolveEndpoint("/v1/messages")
	body, _ := json.Marshal(p.request)
	providerReq, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	providerReq.Header.Add("Content-Type", "application/json")
	providerReq.Header.Add("Accept", "text/event-stream")
	if p.Config.APIVersion == "" {
		providerReq.Header.Add("anthropic-version", "2023-06-11")
	} else {
		providerReq.Header.Add("anthropic-version", p.Config.APIVersion)
	}

	providerReq.Header.Add("x-api-key", p.Config.APIKey)
	if len(p.Config.BetaFeatures) > 0 {
		providerReq.Header.Add("x-beta-features", fmt.Sprintf("%s", p.Config.BetaFeatures))
	}
	return providerReq
}

func (p *MessagesAPIRequest) OnChunk(data []byte, thread *Thread) ChunkResult {
	var env MessagesStreamEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
	}

	switch env.Type {
	case "error":
		var e MessagesStreamErrorEvent
		if err := json.Unmarshal(data, &e); err == nil && e.Error != nil && e.Error.Message != "" {
			switch e.Error.Type {
			case "authentication_error", "permission_error":
				return ErrorChunkResult(AuthenticationError(p.Name(), e.Error.Message))
			case "not_found_error", "request_too_large":
				return ErrorChunkResult(ConfigurationError(p.Name(), e.Error.Message))
			case "rate_limit_exceeded", "rate_limit_error":
				return ErrorChunkResult(RateLimitError(p.Name(), e.Error.Message))
			}
		} else {
			return ErrorChunkResult(UnknownError(p.Name(), string(data)))
		}
	case "message_start":
		var ms MessagesStreamMessageStart
		if err := json.Unmarshal(data, &ms); err == nil {
			if thread.ThreadId == "" && ms.Message.ID != "" {
				thread.ThreadId = ms.Message.ID
			}
			usage := ms.Message.Usage
			thread.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
			thread.Result.CacheReadTokens += usage.CacheReadInputTokens
			thread.Result.CacheWriteTokens += usage.CacheCreationInputTokens
			thread.Result.OutputTokens += usage.OutputTokens
		}
	case "message_delta":
		var md MessagesStreamMessageDelta
		if err := json.Unmarshal(data, &md); err == nil {
			usage := md.Usage
			thread.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
			thread.Result.OutputTokens += usage.OutputTokens
			thread.Result.CacheReadTokens += usage.CacheReadInputTokens
			thread.Result.CacheWriteTokens += usage.CacheCreationInputTokens
		}
	case "content_block_start":
		var cbs MessagesStreamContentBlockStart
		if err := json.Unmarshal(data, &cbs); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		blockId := p.blockId(thread, cbs.Index)
		switch cbs.ContentBlock.Type {
		case "thinking":
			thread.Thinking(blockId, cbs.ContentBlock.Thinking)
			thread.ThinkingSignature(blockId, cbs.ContentBlock.Signature)
		case "redacted_thinking":
			thread.EncryptedThinking(cbs.ContentBlock.Data)
		case "tool_use":
			thread.ToolCall(cbs.ContentBlock.ID, cbs.ContentBlock.Name, string(cbs.ContentBlock.Input))
			p.lastToolCall = messagesLastToolCall{ID: cbs.ContentBlock.ID, IsServer: false}
		case "server_tool_use":
			switch cbs.ContentBlock.Name {
			case "web_search":
				thread.WebSearch(cbs.ContentBlock.ID)
				p.lastToolCall = messagesLastToolCall{
					ID:       cbs.ContentBlock.ID,
					IsServer: true,
					ToolName: "web_search",
				}
			case "web_fetch":
				thread.ViewWebpage(cbs.ContentBlock.ID)
				p.lastToolCall = messagesLastToolCall{
					ID:       cbs.ContentBlock.ID,
					IsServer: true,
					ToolName: "web_fetch",
				}
			}
		case "web_search_tool_result":
			for _, search := range cbs.ContentBlock.Content {
				thread.WebSearchResult(cbs.ContentBlock.ToolUseId, ThreadWebSearchResult{
					Title: search.Title,
					URL:   search.URL,
				})
			}
			thread.CompleteWebSearch(cbs.ContentBlock.ToolUseId)
		case "text":
			thread.Text(blockId, cbs.ContentBlock.Text)
		}
	case "content_block_delta":
		var cbd MessagesStreamContentDelta
		if err := json.Unmarshal(data, &cbd); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		blockId := p.blockId(thread, cbd.Index)

		switch cbd.Delta.Type {
		case "text_delta":
			thread.Text(blockId, cbd.Delta.Text)
		case "citations_delta":
			if cbd.Delta.Citation != nil {
				thread.Cite(blockId, cbd.Delta.Citation.Url)
			}
		case "thinking_delta":
			thread.Thinking(blockId, cbd.Delta.Thinking)
		case "signature_delta":
			thread.ThinkingSignature(blockId, cbd.Delta.Signature)
		case "input_json_delta":
			if p.lastToolCall.IsServer {
				p.lastToolCall.Buffer += cbd.Delta.PartialJSON
			} else {
				thread.ToolCall(p.lastToolCall.ID, "", "")
			}
		}
	case "content_block_stop":
		var cbst MessagesStreamContentBlockStop
		if err := json.Unmarshal(data, &cbst); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		switch cbst.Type {
		case "server_tool_use":
			if p.lastToolCall.IsServer {
				switch p.lastToolCall.ToolName {
				case "web_search":
					var output MessagesWebSearchQuery
					if err := json.Unmarshal([]byte(p.lastToolCall.Buffer), &output); err != nil {
						return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
					}
					thread.WebSearchQuery(p.lastToolCall.ID, output.Query)
				case "web_fetch":
					var output MessagesWebFetchQuery
					if err := json.Unmarshal([]byte(p.lastToolCall.Buffer), &output); err != nil {
						return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
					}
					thread.ViewWebpageUrl(p.lastToolCall.ID, output.URL)
				}
			}
		}
		thread.Complete(p.blockId(thread, cbst.Index))
	case "message_stop":
		return DoneChunkResult()
	}
	return AcceptedResult()
}

func (p *MessagesAPIRequest) ParseHttpError(code int, body []byte) *AIError {
	var message MessagesErrorResponse
	if err := json.Unmarshal(body, &message); err == nil {
		switch code {
		case 401:
			return AuthenticationError(p.Name(), message.Error.Message)
		case 403:
			return AuthenticationError(p.Name(), message.Error.Message)
		case 429:
			return RateLimitError(p.Name(), message.Error.Message)
		}
	}
	return UnknownError(p.Name(), fmt.Sprintf("status %d: %s", code, string(body)))
}
