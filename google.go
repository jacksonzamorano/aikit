package aikit

// GoogleAPI implements Vertex AI's Gemini generateContent endpoint shape.
type GoogleAPI struct {
	Config ProviderConfig
}

func (p GoogleAPI) MakeSession(model string) *ProviderState {
	state := NewProviderState()
	state.Provider = "Google"
	state.Model = model
	return state
}

func (p GoogleAPI) addBlock(block *InferenceBlock, req VertexRequest) {
	switch block.Type {
	case InferenceBlockInput:
		req.Contents = append(req.Contents, VertexContent{
			Role: "user",
			Parts: []VertexPart{
				{Text: block.Text},
			},
		})
	case InferenceBlockSystem:
		req.Contents = append(req.Contents, VertexContent{
			Role: "system",
			Parts: []VertexPart{
				{Text: block.Text},
			},
		})
	case InferenceBlockThinking:
		req.Contents = append(req.Contents, VertexContent{
			Role: "model",
			Parts: []VertexPart{
				{Text: block.Text, Thought: true, ThoughtSignature: block.Signature},
			},
		})
	case InferenceBlockToolCall:
		req.Contents = append(req.Contents, VertexContent{
			Role: "model",
			Parts: []VertexPart{
				{
					FunctionCall: &VertexFunctionCall{
						Name: block.ToolCall.Name,
						Args: block.ToolCall.Arguments,
					},
				},
			},
		})
	case InferenceBlockToolResult:
		req.Contents = append(req.Contents, VertexContent{
			Role: "model",
			Parts: []VertexPart{
				{
					FunctionResult: &VertexFunctionResult{
						Id:       block.ToolResult.ToolCallID,
						Response: block.ToolResult.Output,
					},
				},
			},
		})
	}
}

// func (p GoogleAPI) Infer(state *ProviderState, onPartial func(*ProviderState)) {
// 	modelsBase, err := p.Config.resolveEndpoint("/v1beta/models/")
// 	if err != nil {
// 		state.Success = false
// 		state.Error = &ProviderError{
// 			Cause: err.Error(),
// 		}
// 		return
// 	}
// 	endpoint, _ := url.JoinPath(modelsBase, state.Model+":streamGenerateContent")
// 	u, _ := url.Parse(endpoint)
// 	q := u.Query()
// 	q.Set("key", p.Config.APIKey)
// 	q.Set("alt", "sse")
// 	u.RawQuery = q.Encode()
//
// 	payload := VertexRequest{}
//
// 	tools := []map[string]any{}
// 	for k := range state.Tools {
// 		tool := map[string]any{}
// 		tool["description"] = state.Tools[k].Description
// 		tool["parameters"] = state.Tools[k].Parameters
// 		tool["name"] = k
// 		tools = append(tools, tool)
// 	}
// 	if len(tools) > 0 {
// 		payload.Tools.FunctionDeclarations = tools
// 	}
//
// 	evalDepth := 0
// 	for {
// 		for evalDepth < len(state.Blocks) {
// 			block := state.Blocks[evalDepth]
// 			switch block.Type {
// 			case InferenceBlockInput:
// 				payload.Contents = append(payload.Contents, VertexContent{
// 					Role: ",
// 				})
// 			}
// 			evalDepth++
// 		}
//
// 		body, _ := json.Marshal(payload)
// 		providerReq, err := http.NewRequest("POST", u.String(), bytes.NewReader(body))
// 		if err != nil {
// 			state.Success = false
// 			state.Error = &ProviderError{
// 				Cause: err.Error(),
// 			}
// 			return state
// 		}
// 		providerReq.Header.Add("Content-Type", "application/json")
// 		providerReq.Header.Add("Accept", "text/event-stream")
//
// 		providerResp, err := p.Config.httpClient().Do(providerReq)
// 		if err != nil {
// 			state.Success = false
// 			state.Error = &ProviderError{
// 				Cause: err.Error(),
// 			}
// 			return state
// 		}
// 		if providerResp.StatusCode < 200 || providerResp.StatusCode > 299 {
// 			raw, _ := io.ReadAll(providerResp.Body)
// 			providerResp.Body.Close()
// 			state.Success = false
// 			state.Error = &ProviderError{
// 				Cause: fmt.Sprintf("Status code %d", providerResp.StatusCode),
// 				Data:  string(raw),
// 			}
// 			return state
// 		}
//
// 		var toolCalls []string = []string{}
// 		streamErr := readSSE(providerResp.Body, func(ev sseEvent) error {
// 			if bytes.Equal(ev.data, []byte("[DONE]")) {
// 				return nil
// 			}
// 			if len(ev.data) == 0 {
// 				return nil
// 			}
// 			var chunk VertexGenerateContentResponse
// 			if err := json.Unmarshal(ev.data, &chunk); err != nil {
// 				return err
// 			}
//
// 			state.Result.InputTokens += chunk.Usage.InputTokens
// 			state.Result.OutputTokens += chunk.Usage.OutputTokens
// 			state.Result.CacheReadTokens += chunk.Usage.CachedTokens
//
// 			if len(chunk.Candidates) == 0 {
// 				return nil
// 			}
// 			candidate := chunk.Candidates[0]
// 			for i := range candidate.Content.Parts {
// 				part := candidate.Content.Parts[i]
// 				if part.Text != "" {
// 					if part.Thought {
// 						state.Thinking(fmt.Sprintf("thinking-%d", turn), part.Text, part.ThoughtSignature)
// 					} else {
// 						state.Text(fmt.Sprintf("text-%d", turn), part.Text)
// 					}
// 					if onPartial != nil {
// 						onPartial(state)
// 					}
// 				}
// 				if part.FunctionCall != nil {
// 					id := fmt.Sprintf("toolcall-%d-%d", turn, len(toolCalls)+1)
// 					fnCall := part.FunctionCall
// 					state.ToolCall(id, id, fnCall.Name, fnCall.Args)
// 					toolCalls = append(toolCalls, id)
// 					if onPartial != nil {
// 						onPartial(state)
// 					}
// 				}
// 			}
// 			return nil
// 		})
// 		providerResp.Body.Close()
// 		if streamErr != nil {
// 			state.Success = false
// 			state.Error = streamErr
// 			return state
// 		}
//
// 		usedTool := false
// 		for processedMessageIdx < len(state.Blocks) {
// 			block := state.Blocks[processedMessageIdx]
// 			switch block.Type {
// 			case InferenceBlockText:
// 				part := VertexPart{}
// 				part.Text = block.Text
// 				contents = append(contents, VertexContent{
// 					Role: "model",
// 					Parts: []VertexPart{
// 						part,
// 					},
// 				})
// 			case InferenceBlockThinking:
// 				part := VertexPart{}
// 				part.Thought = true
// 				part.Text = block.Text
// 				part.ThoughtSignature = block.Signature
// 				contents = append(contents, VertexContent{
// 					Role: "model",
// 					Parts: []VertexPart{
// 						part,
// 					},
// 				})
// 			case InferenceBlockToolCall:
// 				usedTool = true
// 				tc := block.ToolCall
// 				tcPart := VertexPart{
// 					FunctionCall: &VertexFunctionCall{
// 						Name: tc.Name,
// 						Args: tc.Arguments,
// 					},
// 				}
// 				out := request.ToolHandler(tc.Name, tc.Arguments)
// 				enc, _ := json.Marshal(out)
// 				state.ToolResult(tc.ID, enc)
// 				if onPartial != nil {
// 					onPartial(state)
// 				}
// 				resEncoded, _ := json.Marshal(out)
// 				trPart := VertexPart{
// 					FunctionResult: &VertexFunctionResult{
// 						Id:       tc.ID,
// 						Name:     tc.Name,
// 						Response: resEncoded,
// 					},
// 				}
// 				contents = append(contents, VertexContent{
// 					Role: "model",
// 					Parts: []VertexPart{
// 						tcPart,
// 					},
// 				})
// 				contents = append(contents, VertexContent{
// 					Role: "user",
// 					Parts: []VertexPart{
// 						trPart,
// 					},
// 				})
// 			}
//
// 			processedMessageIdx++
// 		}
//
// 		if usedTool {
// 			continue
// 		}
// 		state.Success = true
// 		return state
// 	}
// }
