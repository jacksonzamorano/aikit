package aikit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

func (p *MessagesAPIRequest) blockId(thread *StreamState, index int) string {
	return fmt.Sprintf("%s.%d", thread.ThreadId(), index)
}

func (p *MessagesAPIRequest) Name() string {
	return fmt.Sprintf("messages.%s", p.Config.Name)
}

func (p *MessagesAPIRequest) PrepareForUpdates() {}

func (p *MessagesAPIRequest) InitSession(thread *StreamState) {
	tools := make([]map[string]any, 0)
	threadTools := thread.Tools()
	for name := range threadTools {
		toolSpec := map[string]any{}
		toolSpec["description"] = threadTools[name].Description
		toolSpec["input_schema"] = threadTools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, toolSpec)
	}

	if thread.MaxWebSearches() > 0 && p.Config.WebSearchToolName != "" {
		tools = append(tools, map[string]any{
			"type":     p.Config.WebSearchToolName,
			"name":     "web_search",
			"max_uses": thread.MaxWebSearches(),
		})
	}
	if thread.WebFetchEnabled() && p.Config.WebFetchToolName != "" {
		tools = append(tools, map[string]any{
			"type": p.Config.WebFetchToolName,
			"name": "web_fetch",
		})
	}

	p.request = MessagesRequest{
		Messages:  []MessagesMessage{},
		Model:     thread.Model(),
		Tools:     tools,
		MaxTokens: p.Config.MaxTokens,
		Stream:    true,
	}
	if schema := thread.StructuredOutputSchemaValue(); schema != nil {
		p.request.OutputFormat = &MessagesOutputFormat{
			Type:   "json_schema",
			Schema: schema,
		}
	}

	if thread.Reasoning().Budget > 0 {
		p.request.Thinking = &MessagesThinking{
			BudgetTokens: int64(thread.Reasoning().Budget),
			Type:         "enabled",
		}
	}
}

func (p *MessagesAPIRequest) Update(block *ThreadBlock) {
	switch block.Type {
	case InferenceBlockSystem:
		p.request.System = block.Text
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
	case InferenceBlockInputImage:
		if block.Image == nil {
			return
		}
		imgContent := MessagesContent{
			Type: "image",
			Source: &MessagesImageSource{
				Type:      "base64",
				MediaType: block.Image.MediaType,
				Data:      block.Image.GetBase64(),
			},
		}
		// Append to last user message if exists, else create new
		if len(p.request.Messages) > 0 {
			lastIdx := len(p.request.Messages) - 1
			if p.request.Messages[lastIdx].Role == "user" {
				p.request.Messages[lastIdx].Content = append(
					p.request.Messages[lastIdx].Content,
					imgContent,
				)
				return
			}
		}
		p.request.Messages = append(p.request.Messages, MessagesMessage{
			Role:    "user",
			Content: []MessagesContent{imgContent},
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

func (p *MessagesAPIRequest) EncodeRequest(state *StreamState) []byte {
	body, _ := json.Marshal(p.request)
	return body
}

func (p *MessagesAPIRequest) MakeTransport() Transport {
	endpoint := p.Config.resolveEndpoint("/v1/messages")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "text/event-stream")
	if p.Config.APIVersion == "" {
		headers.Set("anthropic-version", "2023-06-01")
	} else {
		headers.Set("anthropic-version", p.Config.APIVersion)
	}
	headers.Set("x-api-key", p.Config.APIKey)
	if p.request.OutputFormat != nil || len(p.Config.BetaFeatures) > 0 {
		features := make([]string, 0, len(p.Config.BetaFeatures)+1)
		features = append(features, p.Config.BetaFeatures...)
		if p.request.OutputFormat != nil {
			features = append(features, "structured-outputs-2025-11-13")
		}
		betaHeader := "x-beta-features"
		if p.Config.Name == "anthropic" {
			betaHeader = "anthropic-beta"
		}
		headers.Set(betaHeader, strings.Join(features, ","))
	}
	return NewSSETransport(p.Name(), endpoint, headers, p.ParseHttpError)
}

func (p *MessagesAPIRequest) OnChunk(data []byte, thread *StreamState) ChunkResult {
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
			if ms.Message.ID != "" {
				thread.SetThreadId(ms.Message.ID)
			}
			usage := ms.Message.Usage
			thread.AddInputTokens(usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens)
			thread.AddCacheReadTokens(usage.CacheReadInputTokens)
			thread.AddCacheWriteTokens(usage.CacheCreationInputTokens)
			thread.AddOutputTokens(usage.OutputTokens)
		}
	case "message_delta":
		var md MessagesStreamMessageDelta
		if err := json.Unmarshal(data, &md); err == nil {
			usage := md.Usage
			thread.AddInputTokens(usage.InputTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens)
			thread.AddOutputTokens(usage.OutputTokens)
			thread.AddCacheReadTokens(usage.CacheReadInputTokens)
			thread.AddCacheWriteTokens(usage.CacheCreationInputTokens)
		}
	case "content_block_start":
		var cbs MessagesStreamContentBlockStart
		if err := json.Unmarshal(data, &cbs); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		blockId := p.blockId(thread, cbs.Index)
		switch cbs.ContentBlock.Type {
		case "thinking":
			thread.AppendThinking(blockId, cbs.ContentBlock.Thinking)
			thread.AppendThinkingSignature(blockId, cbs.ContentBlock.Signature)
		case "redacted_thinking":
			thread.AppendEncryptedThinking(cbs.ContentBlock.Data)
		case "tool_use":
			// In streaming mode, input comes via input_json_delta events, so we start with empty arguments.
			// Only use the initial input if it's not empty (non-streaming or complete input).
			initialArgs := ""
			if len(cbs.ContentBlock.Input) > 0 && string(cbs.ContentBlock.Input) != "{}" {
				initialArgs = string(cbs.ContentBlock.Input)
			}
			thread.AppendToolCall(cbs.ContentBlock.ID, cbs.ContentBlock.Name, initialArgs)
			p.lastToolCall = messagesLastToolCall{ID: cbs.ContentBlock.ID, IsServer: false}
		case "server_tool_use":
			switch cbs.ContentBlock.Name {
			case "web_search":
				thread.AppendWebSearch(cbs.ContentBlock.ID)
				p.lastToolCall = messagesLastToolCall{
					ID:       cbs.ContentBlock.ID,
					IsServer: true,
					ToolName: "web_search",
				}
			case "web_fetch":
				thread.AppendViewWebpage(cbs.ContentBlock.ID)
				p.lastToolCall = messagesLastToolCall{
					ID:       cbs.ContentBlock.ID,
					IsServer: true,
					ToolName: "web_fetch",
				}
			}
		case "web_search_tool_result":
			for _, search := range cbs.ContentBlock.Content {
				thread.AppendWebSearchResult(cbs.ContentBlock.ToolUseId, ThreadWebSearchResult{
					Title: search.Title,
					URL:   search.URL,
				})
			}
			thread.AppendCompleteWebSearch(cbs.ContentBlock.ToolUseId)
		case "text":
			thread.AppendText(blockId, cbs.ContentBlock.Text)
		}
	case "content_block_delta":
		var cbd MessagesStreamContentDelta
		if err := json.Unmarshal(data, &cbd); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		blockId := p.blockId(thread, cbd.Index)

		switch cbd.Delta.Type {
		case "text_delta":
			thread.AppendText(blockId, cbd.Delta.Text)
		case "citations_delta":
			if cbd.Delta.Citation != nil {
				thread.AppendCite(blockId, cbd.Delta.Citation.Url)
			}
		case "thinking_delta":
			thread.AppendThinking(blockId, cbd.Delta.Thinking)
		case "signature_delta":
			thread.AppendThinkingSignature(blockId, cbd.Delta.Signature)
		case "input_json_delta":
			if p.lastToolCall.IsServer {
				p.lastToolCall.Buffer += cbd.Delta.PartialJSON
				switch p.lastToolCall.ToolName {
				case "web_search":
					var output MessagesWebSearchQuery
					if err := json.Unmarshal([]byte(p.lastToolCall.Buffer), &output); err == nil {
						thread.AppendWebSearchQuery(p.lastToolCall.ID, output.Query)
					}
				case "web_fetch":
					var output MessagesWebFetchQuery
					if err := json.Unmarshal([]byte(p.lastToolCall.Buffer), &output); err == nil {
						thread.AppendViewWebpageUrl(p.lastToolCall.ID, output.URL)
					}
				}
			} else {
				thread.AppendToolCall(p.lastToolCall.ID, "", cbd.Delta.PartialJSON)
			}
		}
	case "content_block_stop":
		var cbst MessagesStreamContentBlockStop
		if err := json.Unmarshal(data, &cbst); err != nil {
			return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
		}
		thread.CompleteBlock(p.blockId(thread, cbst.Index))
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
