package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// ResponsesAPI implements the Responses API shape (OpenAI-style).
type ResponsesAPI struct {
	Config          ProviderConfig
	Request         ResponsesRequest
	GenerateSummary bool
}

func (p *ResponsesAPI) Transport() InferenceTransport {
	return TransportSSE
}

func (p *ResponsesAPI) PrepareForUpdates() {
	p.Request.Inputs = []ResponsesInput{}
}

func (p *ResponsesAPI) InitSession(state *ProviderState) {
	tools := []ResponsesTool{}
	for k := range state.Tools {
		tool := ResponsesTool{
			Description: state.Tools[k].Description,
			Parameters:  state.Tools[k].Parameters,
			Name:        k,
			Type:        "function",
		}
		tools = append(tools, tool)
	}

	p.Request = ResponsesRequest{
		Inputs: []ResponsesInput{},
		Tools:  tools,
		Model:  state.Model,
		Stream: true,
	}

	if state.ReasoningEffort != "" {
		p.Request.Reasoning = &ResponsesReasoning{
			Effort: state.ReasoningEffort,
		}
		if p.GenerateSummary {
			p.Request.Reasoning.Summary = "auto"
		}
	}
}

func (p *ResponsesAPI) Update(block *InferenceBlock) {
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
	case InferenceBlockSystem:
		p.Request.Instructions = block.Text
	case InferenceBlockToolResult:
		res, _ := json.Marshal(block.ToolResult.Output)
		p.Request.Inputs = append(p.Request.Inputs, ResponsesInput{
			ToolCallId: block.ToolCall.ID,
			Output:     res,
			Type:       "function_call_output",
		})
	}
}

func (p *ResponsesAPI) MakeRequest(state *ProviderState) *http.Request {
	body, _ := json.Marshal(p.Request)
	providerReq, _ := http.NewRequest("POST", p.Config.resolveEndpoint("/v1/responses"), bytes.NewReader(body))
	providerReq.Header.Add("Content-Type", "application/json")
	providerReq.Header.Add("Accept", "text/event-stream")
	providerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", p.Config.APIKey))
	return providerReq
}

func (p *ResponsesAPI) OnChunk(rawData []byte, state *ProviderState) ChunkResult {

	var data ResponsesStreamEvent
	if err := json.Unmarshal(rawData, &data); err != nil {
		return ErrorChunkResult(err)
	}
	data.Raw = rawData

	switch data.Type {
	case "response.output_text.delta":
		state.Text(data.Delta)
	case "response.output_item.done":
		switch data.Item.Type {
		case "function_call":
			state.ToolCall(data.Item.Id, data.Item.CallId, data.Item.Name, data.Item.Arguments)
			return UpdateChunkResult()
		case "reasoning":
			for s := range data.Summary {
				state.Thinking(data.Summary[s].Text, "")
			}
			return UpdateChunkResult()
		default:
			log.Printf("[Responses] Unhandled content part done: %s", data.Item.Type)
			return EmptyChunkResult()
		}
	case "response.reasoning_summary_text.delta":
		state.Thinking(data.Text, "")
	case "response.completed":
		usage := data.Response.Usage
		state.Result.CacheReadTokens += usage.InputDetails.CachedTokens
		state.Result.InputTokens += (usage.InputTokens + usage.PromptTokens - usage.InputDetails.CachedTokens)
		state.Result.OutputTokens += data.Response.Usage.OutputTokens + data.Response.Usage.PromptTokens
		state.ResponseID = data.Response.Id
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
		return ErrorChunkResult(fmt.Errorf("Responses stream error: %s", msg))
	}
	return EmptyChunkResult()
}
