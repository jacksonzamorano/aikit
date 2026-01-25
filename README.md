# aikit

A unified Go library for interacting with multiple AI provider APIs through a consistent interface.

## Features

- **Multi-provider support** - Anthropic, OpenAI, Google, Groq, Fireworks, X.AI
- **Streaming responses** - Real-time output via Server-Sent Events
- **Tool/function calling** - Define tools and handle function calls automatically
- **Web search & fetch** - Built-in web capabilities for supported providers
- **Extended thinking** - Reasoning/thinking output support
- **Token tracking** - Input, output, and cache token usage

## Installation

```bash
go get github.com/jacksonzamorano/aikit
```

## Quick Start

```go
package main

import (
    "fmt"
    "os"

    "github.com/jacksonzamorano/aikit"
)

func main() {
    provider := aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY"))
    session := provider.Session()

    session.Thread.Model = "claude-sonnet-4-20250514"
    session.Thread.System("You are a helpful assistant.")
    session.Thread.Input("What is the capital of France?")

    result := session.Stream(func(thread *aikit.Thread) {
        // Called on each streaming update
        for _, block := range thread.Blocks {
            if block.Type == aikit.InferenceBlockText {
                fmt.Print(block.Text)
            }
        }
    })

    if !result.Success {
        fmt.Println("Error:", result.Error)
    }
}
```

## Providers

Pre-configured providers are available in `registry.go`:

| Provider | Function | API Type | Features |
|----------|----------|----------|----------|
| Anthropic | `AnthropicProvider(key)` | Messages | Web search, web fetch, thinking |
| OpenAI | `OpenAIProvider(key)` | Responses | Web search |
| OpenAI Verified | `OpenAIVerifiedProvider(key)` | Responses | Thinking summaries, web search |
| Google | `GoogleProvider(key)` | AI Studio | - |
| Groq | `GroqProvider(key)` | Completions | - |
| Fireworks | `FireworksProvider(key)` | Completions | - |
| X.AI | `XAIProvider(key)` | Completions | - |

### Custom Provider Configuration

You can create custom provider configurations:

```go
config := aikit.ProviderConfig{
    Name:                "custom",
    BaseURL:             "https://api.example.com",
    APIKey:              "your-key",
    MakeSessionFunction: aikit.CreateCompletionsSession,
}
session := config.Session()
```

## Examples

### Tool Calling

Define tools and handle function calls:

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "time"

    "github.com/jacksonzamorano/aikit"
)

func main() {
    provider := aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY"))
    session := provider.Session()

    session.Thread.Model = "claude-sonnet-4-20250514"
    session.Thread.System("You are a helpful assistant.")
    session.Thread.Input("What time is it in New York?")

    // Define available tools
	    session.Thread.Tools = map[string]aikit.ToolDefinition{
	        "get_time": {
	            Description: "Get the current time for a timezone",
	            Parameters: &aikit.JsonSchema{
	                Type: "object",
	                Properties: &map[string]*aikit.JsonSchema{
	                    "timezone": {
	                        Type:        "string",
                        Description: "Timezone (e.g., 'America/New_York')",
                    },
                },
                Required: []string{"timezone"},
            },
        },
    }

    // Handle tool calls
    session.Thread.HandleToolFunction = func(name string, args string) string {
        switch name {
        case "get_time":
            var params struct {
                Timezone string `json:"timezone"`
            }
            json.Unmarshal([]byte(args), &params)
            loc, _ := time.LoadLocation(params.Timezone)
            return time.Now().In(loc).Format(time.RFC3339)
        default:
            return "Unknown tool"
        }
    }

    result := session.Stream(func(thread *aikit.Thread) {
        // Handle streaming updates
    })

    // Print final response
    for _, block := range result.Blocks {
        if block.Type == aikit.InferenceBlockText {
            fmt.Println(block.Text)
        }
    }
}
```

### Web Search (Anthropic/OpenAI)

Enable web search for supported providers:

```go
provider := aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY"))
session := provider.Session()

session.Thread.Model = "claude-sonnet-4-20250514"
session.Thread.MaxWebSearches = 3 // Allow up to 3 web searches
session.Thread.System("You are a helpful assistant with access to the web.")
session.Thread.Input("What are the latest developments in Go 1.23?")

result := session.Stream(func(thread *aikit.Thread) {
    for _, block := range thread.Blocks {
        if block.Type == aikit.InferenceBlockWebSearch && block.WebSearch != nil {
            fmt.Printf("Searching: %s\n", block.WebSearch.Query)
        }
    }
})
```

### Extended Thinking

Enable reasoning/thinking output:

```go
provider := aikit.AnthropicProvider(os.Getenv("ANTHROPIC_KEY"))
session := provider.Session()

session.Thread.Model = "claude-sonnet-4-20250514"
session.Thread.Reasoning.Budget = 1024 // Token budget for thinking
session.Thread.Input("Solve this step by step: What is 15% of 340?")

result := session.Stream(func(thread *aikit.Thread) {
    for _, block := range thread.Blocks {
        if block.Type == aikit.InferenceBlockThinking {
            fmt.Printf("[Thinking] %s\n", block.Text)
        }
    }
})
```

### Structured Output

Provide a JSON schema to request structured output:

```go
provider := aikit.OpenAIProvider(os.Getenv("OPENAI_KEY"))
session := provider.Session()

session.Thread.Model = "gpt-4.1-mini"
session.Thread.System("Return a short answer in JSON.")
session.Thread.Input("What is the capital of France?")

session.Thread.StructuredOutputSchema = &aikit.JsonSchema{
    Type: "object",
    Properties: &map[string]*aikit.JsonSchema{
        "answer": {Type: "string"},
    },
    Required: []string{"answer"},
}

strict := true
session.Thread.StructuredOutputStrict = &strict

result := session.Stream(func(thread *aikit.Thread) {
    // Handle streaming updates
})

_ = result
```
