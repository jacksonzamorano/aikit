package aikit

import "testing"

func TestStructuredOutputResponsesRequest(t *testing.T) {
	thread := &Thread{
		Model:                  "gpt-5-nano",
		StructuredOutputSchema: exampleStructuredSchema(),
	}

	request := ResponsesAPIRequest{Config: &ProviderConfig{Name: "openai"}}
	request.InitSession(thread)

	if request.Request.Text == nil || request.Request.Text.Format == nil {
		t.Fatalf("expected response_format to be set")
	}
	if request.Request.Text.Format.Type != "json_schema" {
		t.Fatalf("unexpected response_format type: %q", request.Request.Text.Format.Type)
	}
	if request.Request.Text.Format.Name != "response" {
		t.Fatalf("unexpected response_format name: %q", request.Request.Text.Format.Name)
	}
	if !request.Request.Text.Format.Strict {
		t.Fatalf("expected strict to default to true")
	}
}

func TestStructuredOutputCompletionsRequest(t *testing.T) {
	strict := false
	thread := &Thread{
		Model:                  "openai/gpt-oss-20b",
		StructuredOutputSchema: exampleStructuredSchema(),
		StructuredOutputStrict: &strict,
	}

	request := CompletionsAPIRequest{Config: &ProviderConfig{Name: "groq"}}
	request.InitSession(thread)

	if request.request.ResponseFormat == nil {
		t.Fatalf("expected response_format to be set")
	}
	if request.request.ResponseFormat.JsonSchema.Strict {
		t.Fatalf("expected strict to be false")
	}
}

func TestStructuredOutputMessagesRequest(t *testing.T) {
	thread := &Thread{
		Model:                  "claude-haiku-4-5-20251001",
		StructuredOutputSchema: exampleStructuredSchema(),
	}

	request := MessagesAPIRequest{Config: &ProviderConfig{Name: "anthropic"}}
	request.InitSession(thread)

	if request.request.OutputFormat == nil {
		t.Fatalf("expected output_format to be set")
	}
	if request.request.OutputFormat.Type != "json_schema" {
		t.Fatalf("unexpected output format type: %q", request.request.OutputFormat.Type)
	}
	if request.request.OutputFormat.Schema == nil {
		t.Fatalf("expected output_format schema to be set")
	}
	if request.request.OutputFormat.Schema.Type != "object" {
		t.Fatalf("unexpected output schema type: %q", request.request.OutputFormat.Schema.Type)
	}
}

func TestStructuredOutputAIStudioRequest(t *testing.T) {
	thread := &Thread{
		Model:                  "gemini-3-flash-preview",
		StructuredOutputSchema: exampleStructuredSchema(),
	}

	request := AIStudioAPIRequest{Config: &ProviderConfig{Name: "google"}}
	request.InitSession(thread)

	if request.request.GenerationConfig == nil {
		t.Fatalf("expected generation_config to be set")
	}
	if request.request.GenerationConfig.ResponseMimeType != "application/json" {
		t.Fatalf("unexpected response MIME type: %q", request.request.GenerationConfig.ResponseMimeType)
	}
	if request.request.GenerationConfig.ResponseSchema == nil {
		t.Fatalf("expected response schema to be set")
	}
}

func exampleStructuredSchema() *JsonSchema {
	return &JsonSchema{
		Type: "object",
		Properties: &map[string]*JsonSchema{
			"answer": {Type: "string"},
		},
		Required: []string{"answer"},
	}
}
