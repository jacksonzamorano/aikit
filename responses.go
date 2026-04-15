package aikit

import (
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
func (p *ResponsesAPIRequest) PrepareForUpdates() {
	p.Request.Inputs = []ResponsesInput{}
}

func (p *ResponsesAPIRequest) InitSession(thread *StreamState) {
	tools := []ResponsesTool{}
	threadTools := thread.Tools()
	for k := range threadTools {
		tool := ResponsesTool{
			Description: threadTools[k].Description,
			Parameters:  threadTools[k].Parameters,
			Name:        k,
			Type:        "function",
		}
		tools = append(tools, tool)
	}

	if thread.MaxWebSearches() > 0 && p.Config.WebSearchToolName != "" {
		tools = append(tools, ResponsesTool{
			Type: p.Config.WebSearchToolName,
		})
	}

	p.Request = ResponsesRequest{
		Inputs: []ResponsesInput{},
		Tools:  tools,
		Model:  thread.Model(),
		Stream: true,
	}

	if thread.Reasoning().Effort != "" {
		p.Request.Reasoning = &ResponsesReasoning{
			Effort: thread.Reasoning().Effort,
		}
		if p.Config.UseThinkingSummaries {
			p.Request.Reasoning.Summary = "auto"
		}
	}
	if textFormat := thread.StructuredOutputTextFormat(); textFormat != nil {
		p.Request.Text = &ResponsesText{Format: textFormat}
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

func (p *ResponsesAPIRequest) EncodeRequest(state *StreamState) []byte {
	body, _ := json.Marshal(p.Request)
	return body
}

func (p *ResponsesAPIRequest) MakeTransport() Transport {
	endpoint := p.Config.resolveEndpoint("/v1/responses")
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "text/event-stream")
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", p.Config.APIKey))
	return NewSSETransport(p.Name(), endpoint, headers, p.ParseHttpError)
}

func (p *ResponsesAPIRequest) OnChunk(rawData []byte, thread *StreamState) ChunkResult {

	var data ResponsesStreamEvent
	if err := json.Unmarshal(rawData, &data); err != nil {
		return ErrorChunkResult(DecodingError("responses", err.Error()))
	}
	data.Raw = rawData

	switch data.Type {
	case "response.output_text.delta":
		thread.AppendText(data.ItemId, data.Delta)
	case "response.output_text.done":
		thread.CompleteBlock(data.ItemId)
	case "response.output_item.done":
		switch data.Item.Type {
		case "function_call":
			thread.AppendToolCall(data.Item.CallId, data.Item.Name, data.Item.Arguments)
		case "web_search_call":
			switch data.Item.Action.Type {
			case "search":
				thread.AppendWebSearchQuery(data.Item.Id, data.Item.Action.Query)
				thread.AppendCompleteWebSearch(data.Item.Id)
			case "open_page":
				thread.AppendViewWebpageUrl(data.Item.Id, data.Item.Action.Url)
			}
		case "reasoning":
			for s := range data.Summary {
				thread.AppendThinking(data.ItemId, data.Summary[s].Text)
			}
		}
	case "response.output_text.annotation.added":
		thread.AppendCite(data.ItemId, data.Annotation.Url)
	case "response.reasoning_summary_text.delta":
		thread.AppendThinking(data.ItemId, data.Delta)
	case "response.reasoning_summary_text.done":
		thread.CompleteBlock(data.ItemId)
	case "response.completed":
		usage := data.Response.Usage
		thread.AddCacheReadTokens(usage.InputDetails.CachedTokens)
		thread.AddInputTokens(usage.InputTokens + usage.PromptTokens - usage.InputDetails.CachedTokens)
		thread.AddOutputTokens(usage.OutputTokens + usage.CompletionTokens)
		thread.SetThreadId(data.Response.Id)
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
