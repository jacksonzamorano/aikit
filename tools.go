package aikit

import (
	_ "embed"
	"encoding/json"
	"os"
)

type ToolDefinition struct {
	Description string      `json:"description,omitempty"`
	Parameters  *JsonSchema `json:"parameters,omitempty"`
}

type JsonSchema struct {
	Type        string                  `json:"type,omitempty"`
	Description string                  `json:"description,omitempty"`
	Properties  *map[string]*JsonSchema `json:"properties,omitempty"`
	Items       *JsonSchema             `json:"items,omitempty"`
	Required    []string                `json:"required,omitempty"`

	Enum []any `json:"enum,omitempty"`

	OneOf []*JsonSchema `json:"oneOf,omitempty"`
	AnyOf []*JsonSchema `json:"anyOf,omitempty"`
	AllOf []*JsonSchema `json:"allOf,omitempty"`

	AdditionalProperties any `json:"additionalProperties,omitempty"`
}

type ToolJsonSchema = JsonSchema

// BoundTool pairs a ToolDefinition with its handler function.
type BoundTool struct {
	Definition ToolDefinition
	Handle     func(args string) string
}

// SchemaObject creates an object JsonSchema with the given properties and required fields.
func SchemaObject(properties map[string]*JsonSchema, required ...string) *JsonSchema {
	return &JsonSchema{
		Type:       "object",
		Properties: &properties,
		Required:   required,
	}
}

// SchemaString creates a string JsonSchema with an optional description.
func SchemaString(description string) *JsonSchema {
	return &JsonSchema{Type: "string", Description: description}
}

// SchemaNumber creates a number JsonSchema with an optional description.
func SchemaNumber(description string) *JsonSchema {
	return &JsonSchema{Type: "number", Description: description}
}

// SchemaInteger creates an integer JsonSchema with an optional description.
func SchemaInteger(description string) *JsonSchema {
	return &JsonSchema{Type: "integer", Description: description}
}

// SchemaBoolean creates a boolean JsonSchema with an optional description.
func SchemaBoolean(description string) *JsonSchema {
	return &JsonSchema{Type: "boolean", Description: description}
}

// SchemaArray creates an array JsonSchema with the given items schema.
func SchemaArray(items *JsonSchema) *JsonSchema {
	return &JsonSchema{Type: "array", Items: items}
}

// SchemaEnum creates an enum JsonSchema with the given values.
func SchemaEnum(values ...any) *JsonSchema {
	return &JsonSchema{Enum: values}
}

func GetTools(filename string) map[string]ToolDefinition {
	var defs map[string]ToolDefinition
	bytes, err := os.ReadFile(filename)
	if err != nil {
		panic("failed to read tool definitions: " + err.Error())
	}
	if err := json.Unmarshal(bytes, &defs); err != nil {
		panic("failed to unmarshal tool definitions: " + err.Error())
	}
	return defs
}
