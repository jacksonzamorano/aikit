package aikit

type JsonSchemaResponseFormat struct {
	Type       string                `json:"type"`
	JsonSchema JsonSchemaDescription `json:"json_schema"`
}

type JsonSchemaDescription struct {
	Name   string      `json:"name"`
	Schema *JsonSchema `json:"schema"`
	Strict bool        `json:"strict"`
}

func PrepareStructuredOutputSchema(schema *JsonSchema, strict bool, allowAdditionalProperties bool) *JsonSchema {
	if schema == nil {
		return nil
	}
	return copyStructuredSchema(schema, strict, allowAdditionalProperties)
}

func copyStructuredSchema(schema *JsonSchema, strict bool, allowAdditionalProperties bool) *JsonSchema {
	if schema == nil {
		return nil
	}

	copySchema := &JsonSchema{
		Type:                 schema.Type,
		Description:          schema.Description,
		Required:             append([]string{}, schema.Required...),
		Enum:                 append([]any{}, schema.Enum...),
		AdditionalProperties: schema.AdditionalProperties,
	}

	if schema.Properties != nil {
		props := map[string]*JsonSchema{}
		for key, value := range *schema.Properties {
			props[key] = copyStructuredSchema(value, strict, allowAdditionalProperties)
		}
		copySchema.Properties = &props
	}
	if schema.Items != nil {
		copySchema.Items = copyStructuredSchema(schema.Items, strict, allowAdditionalProperties)
	}
	if len(schema.OneOf) > 0 {
		copySchema.OneOf = make([]*JsonSchema, len(schema.OneOf))
		for i, value := range schema.OneOf {
			copySchema.OneOf[i] = copyStructuredSchema(value, strict, allowAdditionalProperties)
		}
	}
	if len(schema.AnyOf) > 0 {
		copySchema.AnyOf = make([]*JsonSchema, len(schema.AnyOf))
		for i, value := range schema.AnyOf {
			copySchema.AnyOf[i] = copyStructuredSchema(value, strict, allowAdditionalProperties)
		}
	}
	if len(schema.AllOf) > 0 {
		copySchema.AllOf = make([]*JsonSchema, len(schema.AllOf))
		for i, value := range schema.AllOf {
			copySchema.AllOf[i] = copyStructuredSchema(value, strict, allowAdditionalProperties)
		}
	}

	if allowAdditionalProperties {
		if strict && schema.Type == "object" && schema.AdditionalProperties == nil {
			copySchema.AdditionalProperties = false
		}
	} else {
		copySchema.AdditionalProperties = nil
	}

	if nested, ok := schema.AdditionalProperties.(*JsonSchema); ok {
		copySchema.AdditionalProperties = copyStructuredSchema(nested, strict, allowAdditionalProperties)
	}

	return copySchema
}
