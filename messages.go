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

// MessagesAPI implements the Messages API shape (Anthropic-style).
// Configurable BaseURL/Endpoint allows pointing at compatible endpoints.
type MessagesAPI struct {
	Config               ProviderConfig
	Request              MessagesRequest
	Version              string
	BetaFeatures         []string

	lastToolCall messagesLastToolCall
}

func (p *MessagesAPI) blockId(thread *Thread, index int) string {
	return fmt.Sprintf("%s.%d", thread.ThreadId, index)
}

func (p *MessagesAPI) Name() string {
	return fmt.Sprintf("messages.%s", p.Config.Name)
}

func (p *MessagesAPI) Transport() GatewayTransport {
	return TransportSSE
}

func (p *MessagesAPI) PrepareForUpdates() {}

func (p *MessagesAPI) InitSession(state *Thread) {
	tools := make([]map[string]any, 0)
	for name := range state.Tools {
		toolSpec := map[string]any{}
		toolSpec["description"] = state.Tools[name].Description
		toolSpec["input_schema"] = state.Tools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, toolSpec)
	}

	if state.MaxWebSearches > 0 && p.Config.WebSearchToolName != "" {
		tools = append(tools, map[string]any{
			"type":     p.Config.WebSearchToolName,
			"name":     "web_search",
			"max_uses": state.MaxWebSearches,
		})
	}
	if state.WebFetchEnabled && p.Config.WebFetchToolName != "" {
		tools = append(tools, map[string]any{
			"type": p.Config.WebFetchToolName,
			"name": "web_fetch",
		})
	}

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

func (p *MessagesAPI) Update(block *ThreadBlock) {
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
					Input: []byte(block.ToolCall.Arguments),
				},
			},
		})
		if block.ToolResult != nil {
			p.Request.Messages = append(p.Request.Messages, MessagesMessage{
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

func (p *MessagesAPI) MakeRequest(state *Thread) *http.Request {
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
	if len(p.BetaFeatures) > 0 {
		providerReq.Header.Add("x-beta-features", fmt.Sprintf("%s", p.BetaFeatures))
	}
	return providerReq
}

func (p *MessagesAPI) OnChunk(data []byte, state *Thread) ChunkResult {
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
			if state.ThreadId == "" && ms.Message.ID != "" {
				state.ThreadId = ms.Message.ID
			}
			usage := ms.Message.Usage
			state.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
			state.Result.CacheReadTokens += usage.CacheReadInputTokens
			state.Result.CacheWriteTokens += usage.CacheCreationInputTokens
			state.Result.OutputTokens += usage.OutputTokens
		}
	case "message_delta":
		var md MessagesStreamMessageDelta
		if err := json.Unmarshal(data, &md); err == nil {
			usage := md.Usage
			state.Result.InputTokens += usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
			state.Result.OutputTokens += usage.OutputTokens
			state.Result.CacheReadTokens += usage.CacheReadInputTokens
			state.Result.CacheWriteTokens += usage.CacheCreationInputTokens
		}
	case "content_block_start":
		var cbs MessagesStreamContentBlockStart
		if err := json.Unmarshal(data, &cbs); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		blockId := p.blockId(state, cbs.Index)
		switch cbs.ContentBlock.Type {
		case "thinking":
			state.Thinking(blockId, cbs.ContentBlock.Thinking)
			state.ThinkingSignature(blockId, cbs.ContentBlock.Signature)
		case "redacted_thinking":
			state.EncryptedThinking(cbs.ContentBlock.Data)
		case "tool_use":
			state.ToolCall(cbs.ContentBlock.ID, cbs.ContentBlock.Name, string(cbs.ContentBlock.Input))
			p.lastToolCall = messagesLastToolCall{ID: cbs.ContentBlock.ID, IsServer: false}
		case "server_tool_use":
			switch cbs.ContentBlock.Name {
			case "web_search":
				state.WebSearch(cbs.ContentBlock.ID)
				p.lastToolCall = messagesLastToolCall{
					ID:       cbs.ContentBlock.ID,
					IsServer: true,
					ToolName: "web_search",
				}
			case "web_fetch":
				state.ViewWebpage(cbs.ContentBlock.ID)
				p.lastToolCall = messagesLastToolCall{
					ID:       cbs.ContentBlock.ID,
					IsServer: true,
					ToolName: "web_fetch",
				}
			}
		case "web_search_tool_result":
			for _, search := range cbs.ContentBlock.Content {
				state.WebSearchResult(cbs.ContentBlock.ToolUseId, ThreadWebSearchResult{
					Title: search.Title,
					URL:   search.URL,
				})
			}
			state.CompleteWebSearch(cbs.ContentBlock.ToolUseId)
		case "text":
			state.Text(blockId, cbs.ContentBlock.Text)
		}
	case "content_block_delta":
		var cbd MessagesStreamContentDelta
		if err := json.Unmarshal(data, &cbd); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		blockId := p.blockId(state, cbd.Index)

		switch cbd.Delta.Type {
		case "text_delta":
			state.Text(blockId, cbd.Delta.Text)
		case "citations_delta":
			if cbd.Delta.Citation != nil {
				state.Cite(blockId, cbd.Delta.Citation.Url)
			}
		case "thinking_delta":
			state.Thinking(blockId, cbd.Delta.Thinking)
		case "signature_delta":
			state.ThinkingSignature(blockId, cbd.Delta.Signature)
		case "input_json_delta":
			if p.lastToolCall.IsServer {
				p.lastToolCall.Buffer += cbd.Delta.PartialJSON
			} else {
				state.ToolCall(p.lastToolCall.ID, "", "")
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
					state.WebSearchQuery(p.lastToolCall.ID, output.Query)
				case "web_fetch":
					var output MessagesWebFetchQuery
					if err := json.Unmarshal([]byte(p.lastToolCall.Buffer), &output); err != nil {
						return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
					}
					state.ViewWebpageUrl(p.lastToolCall.ID, output.URL)
				}
			}
		}
		state.Complete(p.blockId(state, cbst.Index))
	case "message_stop":
		return DoneChunkResult()
	}
	return AcceptedResult()
}

func (p *MessagesAPI) ParseHttpError(code int, body []byte) *AIError {
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
