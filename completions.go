package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type CompletionsAPI struct {
	Config  ProviderConfig
	Request CompletionsRequest

	lastTool string
}

func (p *CompletionsAPI) Name() string {
	return fmt.Sprintf("completions.%s", p.Config.Name)
}

func (p *CompletionsAPI) Transport() GatewayTransport {
	return TransportSSE
}

func (p *CompletionsAPI) PrepareForUpdates() {}

func (p *CompletionsAPI) InitSession(state *Thread) {
	tools := make([]map[string]any, 0)
	for name := range state.Tools {
		toolSpec := map[string]any{}
		toolSpec["description"] = state.Tools[name].Description
		toolSpec["parameters"] = state.Tools[name].Parameters
		toolSpec["name"] = name
		tools = append(tools, map[string]any{
			"type":     "function",
			"function": toolSpec,
		})
	}
	p.Request = CompletionsRequest{
		Messages: []CompletionsMessage{},
		Model:    state.Model,
		Tools:    tools,
		Stream:   true,
		StreamOptions: map[string]any{
			"include_usage": true,
		},
		ReasoningEffort: state.ReasoningEffort,
	}
}

func (p *CompletionsAPI) Update(block *ThreadBlock) {
	switch block.Type {
	case InferenceBlockInput:
		p.Request.Messages = append(p.Request.Messages, CompletionsMessage{
			Role: "user",
			Content: []CompletionTextBlock{
				{Type: "text", Text: block.Text},
			},
		})
	case InferenceBlockText:
		p.Request.Messages = append(p.Request.Messages, CompletionsMessage{
			Role: "assistant",
			Content: []CompletionTextBlock{
				{Type: "text", Text: block.Text},
			},
		})
	case InferenceBlockToolCall:
		p.Request.Messages = append(p.Request.Messages, CompletionsMessage{
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
			p.Request.Messages = append(p.Request.Messages, CompletionsMessage{
				Role:       "tool",
				Content:    string(block.ToolResult.Output),
				Name:       block.ToolCall.Name,
				ToolCallId: block.ToolCall.ID,
			})
		}
	}
}

func (p *CompletionsAPI) MakeRequest(state *Thread) *http.Request {
	endpoint := p.Config.resolveEndpoint("/v1/chat/completions")
	body, _ := json.Marshal(p.Request)
	providerReq, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	providerReq.Header.Add("Content-Type", "application/json")
	providerReq.Header.Add("Accept", "text/event-stream")
	providerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.Config.APIKey))
	return providerReq
}

func (p *CompletionsAPI) OnChunk(data []byte, state *Thread) ChunkResult {
	var chunk CompletionsStreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
	}

	state.ThreadId = chunk.Id

	if chunk.Usage != nil {
		nonCachedInput := max(chunk.Usage.PromptTokens-chunk.Usage.PromptTokenDetails.CachedTokens, 0)
		state.Result.InputTokens += nonCachedInput
		state.Result.OutputTokens += chunk.Usage.CompletionTokens
		state.Result.CacheReadTokens += chunk.Usage.PromptTokenDetails.CachedTokens
	}

	for _, choice := range chunk.Choices {
		id := fmt.Sprintf("%s-%d", chunk.Id, choice.Index)
		if choice.Delta.ReasoningContent != "" {
			state.Thinking(id, choice.Delta.ReasoningContent)
		}
		if choice.Delta.Content != "" {
			state.Text(id, choice.Delta.Content)
		}

		for i := range choice.Delta.ToolCalls {
			tc := choice.Delta.ToolCalls[i]
			id := tc.Id
			if len(id) == 0 {
				id = p.lastTool
			} else {
				p.lastTool = id
			}

			state.ToolCall(id, tc.Function.Name, tc.Function.Arguments)
		}
		if choice.FinishReason != nil {
			state.Complete(id)
		}
	}
	return AcceptedResult()
}

func (p *CompletionsAPI) ParseHttpError(code int, body []byte) *AIError {
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
	case 429:
		return RateLimitError(p.Name(), string(body))
	}
	return UnknownError(p.Name(), errResp.Error.Message)
}
