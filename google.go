package aikit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
)

type GoogleAPI struct {
	Config  ProviderConfig
	Request VertexRequest
}

func (p *GoogleAPI) Transport() InferenceTransport {
	return TransportSSE
}

func (p *GoogleAPI) PrepareForUpdates() {
}

func (p *GoogleAPI) InitSession(state *ProviderState) {
	tools := []map[string]any{}
	for k := range state.Tools {
		tool := map[string]any{}
		tool["description"] = state.Tools[k].Description
		tool["parameters"] = state.Tools[k].Parameters
		tool["name"] = k
		tools = append(tools, tool)
	}

	p.Request = VertexRequest{
		Contents: []VertexContent{},
		Tools: VertexTools{
			FunctionDeclarations: tools,
		},
	}
}

func (p *GoogleAPI) Update(block *InferenceBlock) {
	switch block.Type {
	case InferenceBlockInput:
		p.Request.Contents = append(p.Request.Contents, VertexContent{
			Role: "user",
			Parts: []VertexPart{
				{Text: block.Text},
			},
		})
	case InferenceBlockSystem:
		p.Request.SystemInstruction = &VertexContent{
			Parts: []VertexPart{
				{Text: block.Text},
			},
		}
	case InferenceBlockThinking:
		p.Request.Contents = append(p.Request.Contents, VertexContent{
			Role: "model",
			Parts: []VertexPart{
				{Text: block.Text, Thought: true, ThoughtSignature: block.Signature},
			},
		})
	case InferenceBlockToolCall:
		p.Request.Contents = append(p.Request.Contents, VertexContent{
			Role: "model",
			Parts: []VertexPart{
				{
					FunctionCall: &VertexFunctionCall{
						Name: block.ToolCall.Name,
						Args: block.ToolCall.Arguments,
					},
					Text:             block.Text,
					ThoughtSignature: block.Signature,
				},
			},
		})
	case InferenceBlockToolResult:
		p.Request.Contents = append(p.Request.Contents, VertexContent{
			Role: "model",
			Parts: []VertexPart{
				{
					FunctionResult: &VertexFunctionResult{
						Id:       block.ToolResult.ToolCallID,
						Name:     block.ToolCall.Name,
						Response: block.ToolResult.Output,
					},
				},
			},
		})
	}
}

func (p GoogleAPI) MakeRequest(state *ProviderState) *http.Request {
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

func (p GoogleAPI) OnChunk(data []byte, state *ProviderState) ChunkResult {
	var chunk VertexGenerateContentResponse
	if err := json.Unmarshal(data, &chunk); err != nil {
		return ErrorChunkResult(err)
	}

	state.Result.InputTokens += chunk.Usage.InputTokens
	state.Result.OutputTokens += chunk.Usage.OutputTokens
	state.Result.CacheReadTokens += chunk.Usage.CachedTokens

	if len(chunk.Candidates) == 0 {
		return EmptyChunkResult()
	}
	candidate := chunk.Candidates[0]
	if candidate.FinishReason != nil {
		return DoneChunkResult()
	}
	for i := range candidate.Content.Parts {
		part := candidate.Content.Parts[i]
		if part.Text != "" {
			if part.Thought {
				state.Thinking(part.Text, part.ThoughtSignature)
			} else {
				state.Text(part.Text)
			}
			return UpdateChunkResult()
		} else if part.FunctionCall != nil {
			id := state.NewBlockId(InferenceBlockToolCall)
			fnCall := part.FunctionCall
			state.ToolCallWithThinking(id, id, fnCall.Name, fnCall.Args, "", part.ThoughtSignature)
			return UpdateChunkResult()
		}
	}
	return EmptyChunkResult()
}
