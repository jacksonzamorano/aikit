package aikit

import (
	"bytes"
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

func (p *CompletionsAPIRequest) Transport() GatewayTransport {
	return TransportSSE
}

func (p *CompletionsAPIRequest) PrepareForUpdates() {}

func (p *CompletionsAPIRequest) InitSession(thread *Thread) {
	tools := make([]map[string]any, 0)
	for name := range thread.Tools {
		toolSpec := map[string]any{}
		toolSpec["description"] = thread.Tools[name].Description
		toolSpec["parameters"] = thread.Tools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, map[string]any{
			"type":     "function",
			"function": toolSpec,
		})
	}
	p.request = CompletionsRequest{
		Messages: []CompletionsMessage{},
		Model:    thread.Model,
		Tools:    tools,
		Stream:   true,
		StreamOptions: map[string]any{
			"include_usage": true,
		},
		ReasoningEffort: thread.ReasoningEffort,
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
				// Content is []CompletionTextBlock or could be []any
				switch c := p.request.Messages[lastIdx].Content.(type) {
				case []CompletionTextBlock:
					// Convert to []any and append image
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
		p.request.Messages = append(p.request.Messages, CompletionsMessage{
			Role: "assistant",
			ToolCalls: []CompletionsToolCall{{
				Id:   block.ID,
				Type: "function",
				Function: &CompletionsToolCallFunction{
					Name:      block.ToolCall.Name,
					Arguments: block.ToolCall.Arguments,
				},
			}},
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

func (p *CompletionsAPIRequest) MakeRequest(thread *Thread) *http.Request {
	endpoint := p.Config.resolveEndpoint("/v1/chat/completions")
	body, _ := json.Marshal(p.request)
	providerReq, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	providerReq.Header.Add("Content-Type", "application/json")
	providerReq.Header.Add("Accept", "text/event-stream")
	providerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.Config.APIKey))
	return providerReq
}

func (p *CompletionsAPIRequest) OnChunk(data []byte, thread *Thread) ChunkResult {
	var chunk CompletionsStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
	}

	thread.ThreadId = chunk.Id

	if chunk.Usage != nil {
		nonCachedInput := max(chunk.Usage.PromptTokens-chunk.Usage.PromptTokenDetails.CachedTokens, 0)
		thread.Result.InputTokens += nonCachedInput
		thread.Result.OutputTokens += chunk.Usage.CompletionTokens
		thread.Result.CacheReadTokens += chunk.Usage.PromptTokenDetails.CachedTokens
	}

	for _, choice := range chunk.Choices {
		baseId := fmt.Sprintf("%s-%d", chunk.Id, choice.Index)
		if choice.Delta.ReasoningContent != "" {
			thread.Thinking(baseId+"-thinking", choice.Delta.ReasoningContent)
		}
		if choice.Delta.Content != "" {
			thread.Text(baseId, choice.Delta.Content)
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
				thread.ToolCall(toolId, tc.Function.Name, tc.Function.Arguments)
			}
		}
		if choice.FinishReason != nil {
			thread.Complete(baseId)
			thread.Complete(baseId + "-thinking")
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
