package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// ResponsesAPIRequest implements the Responses API shape (OpenAI-style).
type ResponsesAPIRequest struct {
	Config  *ProviderConfig
	Request ResponsesRequest
}

func (p *ResponsesAPIRequest) Name() string {
	return fmt.Sprintf("responses.%s", p.Config.Name)
}
func (p *ResponsesAPIRequest) Transport() GatewayTransport {
	return TransportSSE
}

func (p *ResponsesAPIRequest) PrepareForUpdates() {
	p.Request.Inputs = []ResponsesInput{}
}

func (p *ResponsesAPIRequest) InitSession(thread *Thread) {
	tools := []ResponsesTool{}
	for k := range thread.Tools {
		tool := ResponsesTool{
			Description: thread.Tools[k].Description,
			Parameters:  thread.Tools[k].Parameters,
			Name:        k,
			Type:        "function",
		}
		tools = append(tools, tool)
	}

	if thread.MaxWebSearches > 0 && p.Config.WebSearchToolName != "" {
		tools = append(tools, ResponsesTool{
			Type: p.Config.WebSearchToolName,
		})
	}

	p.Request = ResponsesRequest{
		Inputs: []ResponsesInput{},
		Tools:  tools,
		Model:  thread.Model,
		Stream: true,
	}

	if thread.Reasoning.Effort != "" {
		p.Request.Reasoning = &ResponsesReasoning{
			Effort: thread.Reasoning.Effort,
		}
		if p.Config.UseThinkingSummaries {
			p.Request.Reasoning.Summary = "auto"
		}
	}
}

func (p *ResponsesAPIRequest) Update(block *ThreadBlock) {
	switch block.Type {
	case InferenceBlockInput:
		p.Request.Inputs = append(p.Request.Inputs, ResponsesInput{
			Role: "user",
			Content: []ResponsesContent{
				{
					Typ:  "input_text",
					Text: block.Text,
				},
			},
		})
	case InferenceBlockInputImage:
		if block.Image == nil {
			return
		}
		imgContent := ResponsesContent{
			Typ:      "input_image",
			ImageUrl: block.Image.GetDataURL(),
		}
		// Append to last user input if exists, else create new
		if len(p.Request.Inputs) > 0 {
			lastIdx := len(p.Request.Inputs) - 1
			if p.Request.Inputs[lastIdx].Role == "user" {
				p.Request.Inputs[lastIdx].Content = append(
					p.Request.Inputs[lastIdx].Content,
					imgContent,
				)
				return
			}
		}
		p.Request.Inputs = append(p.Request.Inputs, ResponsesInput{
			Role:    "user",
			Content: []ResponsesContent{imgContent},
		})
	case InferenceBlockSystem:
		p.Request.Instructions = block.Text
	case InferenceBlockToolCall:
		if block.ProviderID != p.Name() {
			p.Request.Inputs = append(p.Request.Inputs, ResponsesInput{
				Type:       "function_call",
				ToolCallId: block.ToolCall.ID,
				Name:       block.ToolCall.Name,
				Arguments:  block.ToolCall.Arguments,
				Status:     "completed",
			})
		}
		if block.ToolResult != nil {
			res, _ := json.Marshal(block.ToolResult.Output)
			p.Request.Inputs = append(p.Request.Inputs, ResponsesInput{
				ToolCallId: block.ToolCall.ID,
				Output:     res,
				Type:       "function_call_output",
			})
		}
	}
}

func (p *ResponsesAPIRequest) MakeRequest(thread *Thread) *http.Request {
	body, _ := json.Marshal(p.Request)
	providerReq, _ := http.NewRequest("POST", p.Config.resolveEndpoint("/v1/responses"), bytes.NewReader(body))
	providerReq.Header.Add("Content-Type", "application/json")
	providerReq.Header.Add("Accept", "text/event-stream")
	providerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.Config.APIKey))
	return providerReq
}

func (p *ResponsesAPIRequest) OnChunk(rawData []byte, thread *Thread) ChunkResult {

	var data ResponsesStreamEvent
	if err := json.Unmarshal(rawData, &data); err != nil {
		return ErrorChunkResult(DecodingError("responses", err.Error()))
	}
	data.Raw = rawData

	switch data.Type {
	case "response.output_text.delta":
		thread.Text(data.ItemId, data.Delta)
	case "response.output_text.done":
		thread.Complete(data.ItemId)
	case "response.output_item.done":
		switch data.Item.Type {
		case "function_call":
			thread.ToolCall(data.Item.CallId, data.Item.Name, data.Item.Arguments)
		case "web_search_call":
			switch data.Item.Action.Type {
			case "search":
				thread.WebSearchQuery(data.Item.Id, data.Item.Action.Query)
				thread.CompleteWebSearch(data.Item.Id)
			case "open_page":
				thread.ViewWebpageUrl(data.Item.Id, data.Item.Action.Url)
			}
		case "reasoning":
			for s := range data.Summary {
				thread.Thinking(data.ItemId, data.Summary[s].Text)
			}
		}
	case "response.output_text.annotation.added":
		thread.Cite(data.ItemId, data.Annotation.Url)
	case "response.reasoning_summary_text.delta":
		thread.Thinking(data.ItemId, data.Delta)
	case "response.reasoning_summary_text.done":
		thread.Complete(data.ItemId)
	case "response.completed":
		usage := data.Response.Usage
		thread.Result.CacheReadTokens += usage.InputDetails.CachedTokens
		thread.Result.InputTokens += (usage.InputTokens + usage.PromptTokens - usage.InputDetails.CachedTokens)
		thread.Result.OutputTokens += usage.OutputTokens + usage.CompletionTokens
		thread.ThreadId = data.Response.Id
		p.Request.PreviousResponseID = data.Response.Id
		return DoneChunkResult()
	case "error", "response.failed":
		msg := ""
		if data.Error != nil {
			msg = data.Error.Message
		}
		if msg == "" {
			msg = string(data.Raw)
		}
		return ErrorChunkResult(UnknownError("responses", msg))
	}
	return AcceptedResult()
}

func (p *ResponsesAPIRequest) ParseHttpError(code int, body []byte) *AIError {
	return nil
}
