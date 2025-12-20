package aikit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
)

type AIStudioAPI struct {
	Config  ProviderConfig
	request AIStudioRequest
}

func (p *AIStudioAPI) Name() string {
	return "aistudio." + p.Config.Name
}
func (p *AIStudioAPI) Transport() GatewayTransport {
	return TransportSSE
}

func (p *AIStudioAPI) PrepareForUpdates() {
}

func (p *AIStudioAPI) InitSession(thread *Thread) {
	tools := []map[string]any{}
	for k := range thread.Tools {
		tool := map[string]any{}
		tool["description"] = thread.Tools[k].Description
		tool["parameters"] = thread.Tools[k].Parameters
		tool["name"] = k
		tools = append(tools, tool)
	}

	p.request = AIStudioRequest{
		Contents: []AIStudioContent{},
		Tools: AIStudioTools{
			FunctionDeclarations: tools,
		},
	}
}

func (p *AIStudioAPI) Update(block *ThreadBlock) {
	switch block.Type {
	case InferenceBlockInput:
		p.request.Contents = append(p.request.Contents, AIStudioContent{
			Role: "user",
			Parts: []AIStudioPart{
				{Text: block.Text},
			},
		})
	case InferenceBlockSystem:
		p.request.SystemInstruction = &AIStudioContent{
			Parts: []AIStudioPart{
				{Text: block.Text},
			},
		}
	case InferenceBlockThinking:
		p.request.Contents = append(p.request.Contents, AIStudioContent{
			Role: "model",
			Parts: []AIStudioPart{
				{Text: block.Text, Thought: true, ThoughtSignature: block.Signature},
			},
		})
	case InferenceBlockToolCall:
		p.request.Contents = append(p.request.Contents, AIStudioContent{
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
			p.request.Contents = append(p.request.Contents, AIStudioContent{
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

func (p AIStudioAPI) MakeRequest(thread *Thread) *http.Request {
	modelsBase := p.Config.resolveEndpoint("/v1beta/models/")
	endpoint, _ := url.JoinPath(modelsBase, thread.Model+":streamGenerateContent")
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("key", p.Config.APIKey)
	q.Set("alt", "sse")
	u.RawQuery = q.Encode()

	body, _ := json.Marshal(p.request)
	providerReq, _ := http.NewRequest("POST", u.String(), bytes.NewReader(body))
	return providerReq
}

func (p AIStudioAPI) OnChunk(data []byte, thread *Thread) ChunkResult {
	var chunk AIStudioGenerateContentResponse
	if err := json.Unmarshal(data, &chunk); err != nil {
		return ErrorChunkResult(DecodingError(p.Name(), err.Error()))
	}
	thread.Result.InputTokens += chunk.Usage.InputTokens
	thread.Result.OutputTokens += chunk.Usage.OutputTokens
	thread.Result.CacheReadTokens += chunk.Usage.CachedTokens
	thread.ThreadId = chunk.ResponseId

	candidate := chunk.Candidates[0]
	if candidate.FinishReason != nil {
		thread.Complete(chunk.ResponseId)
		return DoneChunkResult()
	}
	for i := range candidate.Content.Parts {
		id := chunk.ResponseId
		part := candidate.Content.Parts[i]
		if part.Text != "" {
			if part.Thought {
				thread.ThinkingWithSignature(id, part.Text, part.ThoughtSignature)
			} else {
				thread.Text(id, part.Text)
			}
		} else if part.FunctionCall != nil {
			id := thread.NewBlockId(InferenceBlockToolCall)
			fnCall := part.FunctionCall
			thread.ToolCallWithThinking(id, fnCall.Name, string(fnCall.Args), "", part.ThoughtSignature)
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
