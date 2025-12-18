package aikit

import (
	_ "embed"
	"encoding/json"
	"os"
)

type ToolDefinition struct {
	Description string          `json:"description,omitempty"`
	Parameters  *ToolJsonSchema `json:"parameters,omitempty"`
}

type ToolJsonSchema struct {
	Type        string                     `json:"type,omitempty"`
	Description string                     `json:"description,omitempty"`
	Properties  map[string]*ToolJsonSchema `json:"properties"`
	Items       *ToolJsonSchema            `json:"items,omitempty"`
	Required    []string                   `json:"required,omitempty"`

	Enum []any `json:"enum,omitempty"`

	OneOf []*ToolJsonSchema `json:"oneOf,omitempty"`
	AnyOf []*ToolJsonSchema `json:"anyOf,omitempty"`
	AllOf []*ToolJsonSchema `json:"allOf,omitempty"`

	AdditionalProperties any `json:"additionalProperties,omitempty"`
}

func GetTools(filename string) map[string]ToolJsonSchema {
	var defs map[string]ToolJsonSchema
	bytes, err := os.ReadFile(filename)
	if err != nil {
		panic("failed to read tool definitions: " + err.Error())
	}
	if err := json.Unmarshal(bytes, &defs); err != nil {
		panic("failed to unmarshal tool definitions: " + err.Error())
	}
	return defs
}
