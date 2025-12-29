package aikit

import (
	_ "embed"
	"encoding/json"
	"os"
)

type ToolDefinition struct {
	Description string          `json:"description,omitempty" xml:"description,omitempty"`
	Parameters  *ToolJsonSchema `json:"parameters,omitempty" xml:"parameters,omitempty"`
}

type ToolJsonSchema struct {
	Type        string                      `json:"type,omitempty" xml:"type,attr,omitempty"`
	Description string                      `json:"description,omitempty" xml:"description,omitempty"`
	Properties  *map[string]*ToolJsonSchema `json:"properties,omitempty" xml:"properties,omitempty"`
	Items       *ToolJsonSchema             `json:"items,omitempty" xml:"items,omitempty"`
	Required    []string                    `json:"required,omitempty" xml:"required>field,omitempty"`

	Enum []any `json:"enum,omitempty" xml:"enum>value,omitempty"`

	OneOf []*ToolJsonSchema `json:"oneOf,omitempty" xml:"oneOf>schema,omitempty"`
	AnyOf []*ToolJsonSchema `json:"anyOf,omitempty" xml:"anyOf>schema,omitempty"`
	AllOf []*ToolJsonSchema `json:"allOf,omitempty" xml:"allOf>schema,omitempty"`

	AdditionalProperties any `json:"additionalProperties,omitempty" xml:"additionalProperties,omitempty"`
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
