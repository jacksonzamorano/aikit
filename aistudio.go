package aikit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
)

type AIStudioAPI struct {
	Config  ProviderConfig
	Request AIStudioRequest
}

func (p *AIStudioAPI) Name() string {
	return "aistudio." + p.Config.Name
}
func (p *AIStudioAPI) Transport() GatewayTransport {
	return TransportSSE
}

func (p *AIStudioAPI) PrepareForUpdates() {
}

func (p *AIStudioAPI) InitSession(state *Thread) {
	tools := []map[string]any{}
	for k := range state.Tools {
		tool := map[string]any{}
		tool["description"] = state.Tools[k].Description
		tool["parameters"] = state.Tools[k].Parameters
		tool["name"] = k
		tools = append(tools, tool)
	}

	p.Request = AIStudioRequest{
		Contents: []AIStudioContent{},
		Tools: AIStudioTools{
			FunctionDeclarations: tools,
		},
	}
}

func (p *AIStudioAPI) Update(block *ThreadBlock) {
	switch block.Type {
	case InferenceBlockInput:
		p.Request.Contents = append(p.Request.Contents, AIStudioContent{
			Role: "user",
			Parts: []AIStudioPart{
				{Text: block.Text},
			},
		})
	case InferenceBlockSystem:
		p.Request.SystemInstruction = &AIStudioContent{
			Parts: []AIStudioPart{
				{Text: block.Text},
			},
		}
	case InferenceBlockThinking:
		p.Request.Contents = append(p.Request.Contents, AIStudioContent{
			Role: "model",
			Parts: []AIStudioPart{
				{Text: block.Text, Thought: true, ThoughtSignature: block.Signature},
			},
		})
	case InferenceBlockToolCall:
		p.Request.Contents = append(p.Request.Contents, AIStudioContent{
			Role: "model",
			Parts: []AIStudioPart{
				{
					FunctionCall: &AIStudioFunctionCall{
						Name: block.ToolCall.Name,
						Args: []byte(block.ToolCall.Arguments),
					},
					Text:             block.Text,
					ThoughtSignature: block.Signature,
				},
			},
		})
		if block.ToolResult != nil {
			p.Request.Contents = append(p.Request.Contents, AIStudioContent{
				Role: "model",
				Parts: []AIStudioPart{
					{
						FunctionResult: &AIStudioFunctionResult{
							Id:       block.ToolResult.ToolCallID,
							Name:     block.ToolCall.Name,
							Response: []byte(block.ToolResult.Output),
						},
					},
				},
			})
		}
	}
}

func (p AIStudioAPI) MakeRequest(state *Thread) *http.Request {
	modelsBase := p.Config.resolveEndpoint("/v1beta/models/")
	endpoint, _ := url.JoinPath(modelsBase, state.Model+":streamGenerateContent")
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("key", p.Config.APIKey)
	q.Set("alt", "sse")
	u.RawQuery = q.Encode()

	body, _ := json.Marshal(p.Request)
	providerReq, _ := http.NewRequest("POST", u.String(), bytes.NewReader(body))
	return providerReq
}

func (p AIStudioAPI) OnChunk(data []byte, state *Thread) ChunkResult {
	var chunk AIStudioGenerateContentResponse
	if err := json.Unmarshal(data, &chunk); err != nil {
		return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
	}
	state.Result.InputTokens += chunk.Usage.InputTokens
	state.Result.OutputTokens += chunk.Usage.OutputTokens
	state.Result.CacheReadTokens += chunk.Usage.CachedTokens
	state.ThreadId = chunk.ResponseId

	candidate := chunk.Candidates[0]
	if candidate.FinishReason != nil {
		state.Complete(chunk.ResponseId)
		return DoneChunkResult()
	}
	for i := range candidate.Content.Parts {
		id := chunk.ResponseId
		part := candidate.Content.Parts[i]
		if part.Text != "" {
			if part.Thought {
				state.ThinkingWithSignature(id, part.Text, part.ThoughtSignature)
			} else {
				state.Text(id, part.Text)
			}
		} else if part.FunctionCall != nil {
			id := state.NewBlockId(InferenceBlockToolCall)
			fnCall := part.FunctionCall
			state.ToolCallWithThinking(id, fnCall.Name, string(fnCall.Args), "", part.ThoughtSignature)
		}
	}
	return AcceptedResult()
}

func (p AIStudioAPI) ParseHttpError(code int, body []byte) *AIError {
	var data AIStudioErrorResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}
	switch data.Error.Code {
	case 401:
		return AuthenticationError(p.Name(), data.Error.Message)
	case 429:
		return RateLimitError(p.Name(), data.Error.Message)
	default:
		return UnknownError(p.Name(), data.Error.Message)
	}
}
