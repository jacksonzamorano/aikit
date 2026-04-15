package aikit

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type CompletionsAPIRequest struct {
	Config *ProviderConfig

	request  CompletionsRequest
	lastTool string
}

func (p *CompletionsAPIRequest) Name() string {
	return fmt.Sprintf("completions.%s", p.Config.Name)
}

func (p *CompletionsAPIRequest) PrepareForUpdates() {}

func (p *CompletionsAPIRequest) InitSession(thread *StreamState) {
	tools := make([]map[string]any, 0)
	threadTools := thread.Tools()
	for name := range threadTools {
		toolSpec := map[string]any{}
		toolSpec["description"] = threadTools[name].Description
		toolSpec["parameters"] = threadTools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, map[string]any{
			"type":     "function",
			"function": toolSpec,
		})
	}
	p.request = CompletionsRequest{
		Messages: []CompletionsMessage{},
		Model:    thread.Model(),
		Tools:    tools,
		Stream:   true,
		StreamOptions: map[string]any{
			"include_usage": true,
		},
		ReasoningEffort: thread.Reasoning().Effort,
	}
	if responseFormat := thread.StructuredOutputFormat(); responseFormat != nil {
		p.request.ResponseFormat = responseFormat
	}
}

func (p *CompletionsAPIRequest) Update(block *ThreadBlock) {
	switch block.Type {
	case InferenceBlockSystem:
		p.request.Messages = append(p.request.Messages, CompletionsMessage{
			Role:    "system",
			Content: block.Text,
		})
	case InferenceBlockInput:
		p.request.Messages = append(p.request.Messages, CompletionsMessage{
			Role: "user",
			Content: []CompletionTextBlock{
				{Type: "text", Text: block.Text},
			},
		})
	case InferenceBlockInputImage:
		if block.Image == nil {
			return
		}
		imgBlock := CompletionImageUrlBlock{
			Type: "image_url",
			ImageUrl: CompletionImageUrlDetail{
				Url: block.Image.GetDataURL(),
			},
		}
		// Append to last user message if exists, else create new
		if len(p.request.Messages) > 0 {
			lastIdx := len(p.request.Messages) - 1
			if p.request.Messages[lastIdx].Role == "user" {
				switch c := p.request.Messages[lastIdx].Content.(type) {
				case []CompletionTextBlock:
					arr := make([]any, len(c))
					for i, tb := range c {
						arr[i] = tb
					}
					arr = append(arr, imgBlock)
					p.request.Messages[lastIdx].Content = arr
				case []any:
					p.request.Messages[lastIdx].Content = append(c, imgBlock)
				}
				return
			}
		}
		p.request.Messages = append(p.request.Messages, CompletionsMessage{
			Role:    "user",
			Content: []any{imgBlock},
		})
	case InferenceBlockText:
		p.request.Messages = append(p.request.Messages, CompletionsMessage{
			Role: "assistant",
			Content: []CompletionTextBlock{
				{Type: "text", Text: block.Text},
			},
		})
	case InferenceBlockToolCall:
		tc := CompletionsToolCall{
			Id:   block.ID,
			Type: "function",
			Function: &CompletionsToolCallFunction{
				Name:      block.ToolCall.Name,
				Arguments: block.ToolCall.Arguments,
			},
		}
		if block.Signature != "" {
			tc.ExtraContent = &CompletionsExtraContent{
				Google: &CompletionsGoogleExtra{
					ThoughtSignature: block.Signature,
				},
			}
		}
		p.request.Messages = append(p.request.Messages, CompletionsMessage{
			Role:      "assistant",
			ToolCalls: []CompletionsToolCall{tc},
		})
		if block.ToolResult != nil {
			p.request.Messages = append(p.request.Messages, CompletionsMessage{
				Role:       "tool",
				Content:    string(block.ToolResult.Output),
				Name:       block.ToolCall.Name,
				ToolCallId: block.ToolCall.ID,
			})
		}
	}
}

func (p *CompletionsAPIRequest) EncodeRequest(state *StreamState) []byte {
	body, _ := json.Marshal(p.request)
	return body
}

func (p *CompletionsAPIRequest) MakeTransport() Transport {
	endpoint := p.Config.resolveEndpoint("/v1/chat/completions")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "text/event-stream")
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", p.Config.APIKey))
	return NewSSETransport(p.Name(), endpoint, headers, p.ParseHttpError)
}

func (p *CompletionsAPIRequest) OnChunk(data []byte, thread *StreamState) ChunkResult {
	var chunk CompletionsStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
	}

	thread.SetThreadId(chunk.Id)

	if chunk.Usage != nil {
		nonCachedInput := max(chunk.Usage.PromptTokens-chunk.Usage.PromptTokenDetails.CachedTokens, 0)
		thread.AddInputTokens(nonCachedInput)
		thread.AddOutputTokens(chunk.Usage.CompletionTokens)
		thread.AddCacheReadTokens(chunk.Usage.PromptTokenDetails.CachedTokens)
	}

	for _, choice := range chunk.Choices {
		baseId := fmt.Sprintf("%s-%d", chunk.Id, choice.Index)
		if choice.Delta.ReasoningContent != "" {
			thread.AppendThinking(baseId+"-thinking", choice.Delta.ReasoningContent)
		}
		if choice.Delta.Content != "" {
			thread.AppendText(baseId, choice.Delta.Content)
		}

		for i := range choice.Delta.ToolCalls {
			tc := choice.Delta.ToolCalls[i]
			toolId := tc.Id
			if len(toolId) == 0 {
				toolId = p.lastTool
			} else {
				p.lastTool = toolId
			}

			if tc.Function != nil {
				if tc.ExtraContent != nil && tc.ExtraContent.Google != nil && tc.ExtraContent.Google.ThoughtSignature != "" {
					thread.AppendToolCallWithThinking(toolId, tc.Function.Name, tc.Function.Arguments, "", tc.ExtraContent.Google.ThoughtSignature)
				} else {
					thread.AppendToolCall(toolId, tc.Function.Name, tc.Function.Arguments)
				}
			}
		}
		if choice.FinishReason != nil {
			thread.CompleteBlock(baseId)
			thread.CompleteBlock(baseId + "-thinking")
		}
	}
	return AcceptedResult()
}

func (p *CompletionsAPIRequest) ParseHttpError(code int, body []byte) *AIError {
	var errResp CompletionsErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil {
		switch errResp.Error.Type {
		case "invalid_request_error":
			return ConfigurationError(p.Name(), errResp.Error.Message)
		case "authentication_error":
			return AuthenticationError(p.Name(), errResp.Error.Message)
		case "rate_limit_error":
			return RateLimitError(p.Name(), errResp.Error.Message)
		}
	}
	switch code {
	case 401, 403:
		return AuthenticationError(p.Name(), string(body))
	case 404:
		return ConfigurationError(p.Name(), string(body))
	case 429:
		return RateLimitError(p.Name(), string(body))
	}
	if len(errResp.Error.Message) == 0 {
		return UnknownError(p.Name(), string(body))
	}
	return UnknownError(p.Name(), errResp.Error.Message)
}
