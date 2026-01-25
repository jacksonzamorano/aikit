package aikit

import (
	"testing"
)

func TestUnit_Tool_DefinitionSerialization(t *testing.T) {
	toolDef := ToolDefinition{
		Description: "Search for information on a topic",
		Parameters: &JsonSchema{
			Type: "object",
			Properties: &map[string]*JsonSchema{
				"query": {Type: "string", Description: "The search query"},
				"limit": {Type: "integer", Description: "Maximum number of results"},
			},
			Required: []string{"query"},
		},
	}

	if toolDef.Description != "Search for information on a topic" {
		t.Errorf("Unexpected description: %q", toolDef.Description)
	}
	if toolDef.Parameters.Type != "object" {
		t.Errorf("Expected parameters type 'object', got %q", toolDef.Parameters.Type)
	}
	if len(*toolDef.Parameters.Properties) != 2 {
		t.Errorf("Expected 2 properties, got %d", len(*toolDef.Parameters.Properties))
	}
}
