package aikit

import (
	_ "embed"
	"encoding/json"
	"os"
)

type ToolDefinition struct {
	Description string      `json:"description,omitempty" xml:"description,omitempty"`
	Parameters  *JsonSchema `json:"parameters,omitempty" xml:"parameters,omitempty"`
}

type JsonSchema struct {
	Type        string                  `json:"type,omitempty" xml:"type,attr,omitempty"`
	Description string                  `json:"description,omitempty" xml:"description,omitempty"`
	Properties  *map[string]*JsonSchema `json:"properties,omitempty" xml:"properties,omitempty"`
	Items       *JsonSchema             `json:"items,omitempty" xml:"items,omitempty"`
	Required    []string                `json:"required,omitempty" xml:"required>field,omitempty"`

	Enum []any `json:"enum,omitempty" xml:"enum>value,omitempty"`

	OneOf []*JsonSchema `json:"oneOf,omitempty" xml:"oneOf>schema,omitempty"`
	AnyOf []*JsonSchema `json:"anyOf,omitempty" xml:"anyOf>schema,omitempty"`
	AllOf []*JsonSchema `json:"allOf,omitempty" xml:"allOf>schema,omitempty"`

	AdditionalProperties any `json:"additionalProperties,omitempty" xml:"additionalProperties,omitempty"`
}

type ToolJsonSchema = JsonSchema

func GetTools(filename string) map[string]JsonSchema {
	var defs map[string]JsonSchema
	bytes, err := os.ReadFile(filename)
	if err != nil {
		panic("failed to read tool definitions: " + err.Error())
	}
	if err := json.Unmarshal(bytes, &defs); err != nil {
		panic("failed to unmarshal tool definitions: " + err.Error())
	}
	return defs
}
