package aikit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// GoogleAPI implements Vertex AI's Gemini generateContent endpoint shape.
type GoogleAPI struct {
	Config ProviderConfig
}

func (p GoogleAPI) Infer(request *InferenceRequest, onPartial func(*ProviderState)) *ProviderState {
	state := NewProviderState()
	state.Provider = "vertex"
	state.Model = request.Model

	modelsBase, err := p.Config.resolveEndpoint("/v1beta/models/")
	if err != nil {
		state.Success = false
		state.Error = &ProviderError{
			Cause: err.Error(),
		}
		return state
	}
	endpoint, _ := url.JoinPath(modelsBase, request.Model+":streamGenerateContent")
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("key", p.Config.APIKey)
	q.Set("alt", "sse")
	u.RawQuery = q.Encode()

	state.System(request.SystemPrompt)
	state.Input(request.Prompt)

	payload := map[string]any{}
	contents := []VertexContent{
		{
			Role: "user",
			Parts: []VertexPart{
				{Text: request.Prompt},
			},
		},
	}
	if request.SystemPrompt != "" {
		payload["system_instruction"] = VertexContent{
			Parts: []VertexPart{
				{Text: request.SystemPrompt},
			},
		}
	}
	if request.ReasoningEffort != nil {
		if v, ok := reasoningEffortValue(request.ReasoningEffort); ok {
			budgetTokens := int64(2048)
			if parsed, err := strconv.Atoi(v); err == nil {
				budgetTokens = int64(parsed)
			}
			payload["generationConfig"] = map[string]any{
				"thinkingConfig": map[string]any{
					"thinkingBudget": budgetTokens,
				},
			}
		}
	}

	tools := []map[string]any{}
	for k := range request.Tools {
		tool := map[string]any{}
		tool["description"] = request.Tools[k].Description
		tool["parameters"] = request.Tools[k].Parameters
		tool["name"] = k
		tools = append(tools, tool)
	}
	if len(tools) > 0 {
		payload["tools"] = []map[string]any{
			{"functionDeclarations": tools},
		}
	}

	turn := 0
	processedMessageIdx := len(state.Blocks)
	for {
		turn++
		payload["contents"] = contents
		body, _ := json.Marshal(payload)
		providerReq, err := http.NewRequest("POST", u.String(), bytes.NewReader(body))
		if err != nil {
			state.Success = false
			state.Error = &ProviderError{
				Cause: err.Error(),
			}
			return state
		}
		providerReq.Header.Add("Content-Type", "application/json")
		providerReq.Header.Add("Accept", "text/event-stream")

		providerResp, err := p.Config.httpClient().Do(providerReq)
		if err != nil {
			state.Success = false
			state.Error = &ProviderError{
				Cause: err.Error(),
			}
			return state
		}
		if providerResp.StatusCode < 200 || providerResp.StatusCode > 299 {
			raw, _ := io.ReadAll(providerResp.Body)
			providerResp.Body.Close()
			state.Success = false
			state.Error = &ProviderError{
				Cause: fmt.Sprintf("Status code %d", providerResp.StatusCode),
				Data:  string(raw),
			}
			return state
		}

		var toolCalls []string = []string{}
		streamErr := readSSE(providerResp.Body, func(ev sseEvent) error {
			if bytes.Equal(ev.data, []byte("[DONE]")) {
				return nil
			}
			if len(ev.data) == 0 {
				return nil
			}
			var chunk VertexGenerateContentResponse
			if err := json.Unmarshal(ev.data, &chunk); err != nil {
				return err
			}

			state.Result.InputTokens += chunk.Usage.InputTokens
			state.Result.OutputTokens += chunk.Usage.OutputTokens
			state.Result.CacheReadTokens += chunk.Usage.CachedTokens

			if len(chunk.Candidates) == 0 {
				return nil
			}
			candidate := chunk.Candidates[0]
			for i := range candidate.Content.Parts {
				part := candidate.Content.Parts[i]
				if part.Text != "" {
					if part.Thought {
						state.Thinking(fmt.Sprintf("thinking-%d", turn), part.Text, part.ThoughtSignature)
					} else {
						state.Text(fmt.Sprintf("text-%d", turn), part.Text)
					}
					if onPartial != nil {
						onPartial(state)
					}
				}
				if part.FunctionCall != nil {
					id := fmt.Sprintf("toolcall-%d-%d", turn, len(toolCalls)+1)
					fnCall := part.FunctionCall
					state.ToolCall(id, id, fnCall.Name, fnCall.Args)
					toolCalls = append(toolCalls, id)
					if onPartial != nil {
						onPartial(state)
					}
				}
			}
			return nil
		})
		providerResp.Body.Close()
		if streamErr != nil {
			state.Success = false
			state.Error = streamErr
			return state
		}

		usedTool := false
		for processedMessageIdx < len(state.Blocks) {
			block := state.Blocks[processedMessageIdx]
			switch block.Type {
			case InferenceBlockText:
				part := VertexPart{}
				part.Text = block.Text
				contents = append(contents, VertexContent{
					Role: "model",
					Parts: []VertexPart{
						part,
					},
				})
			case InferenceBlockThinking:
				part := VertexPart{}
				part.Thought = true
				part.Text = block.Text
				part.ThoughtSignature = block.Signature
				contents = append(contents, VertexContent{
					Role: "model",
					Parts: []VertexPart{
						part,
					},
				})
			case InferenceBlockToolCall:
				usedTool = true
				tc := block.ToolCall
				tcPart := VertexPart{
					FunctionCall: &VertexFunctionCall{
						Name: tc.Name,
						Args: tc.Arguments,
					},
				}
				out := request.ToolHandler(tc.Name, tc.Arguments)
				enc, _ := json.Marshal(out)
				state.ToolResult(tc.ID, enc)
				if onPartial != nil {
					onPartial(state)
				}
				resEncoded, _ := json.Marshal(out)
				trPart := VertexPart{
					FunctionResult: &VertexFunctionResult{
						Id:       tc.ID,
						Name:     tc.Name,
						Response: resEncoded,
					},
				}
				contents = append(contents, VertexContent{
					Role: "model",
					Parts: []VertexPart{
						tcPart,
					},
				})
				contents = append(contents, VertexContent{
					Role: "user",
					Parts: []VertexPart{
						trPart,
					},
				})
			}

			processedMessageIdx++
		}

		if usedTool {
			continue
		}
		state.Success = true
		return state
	}
}
