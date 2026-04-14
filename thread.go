package aikit

import "fmt"

// ReasoningConfig configures reasoning behavior for the thread.
// Effort is for string-based providers (e.g., OpenAI's "low"/"medium"/"high").
// Budget is for integer-based token budgets.
type ReasoningConfig struct {
	Effort string `json:"effort,omitempty"`
	Budget int    `json:"budget,omitempty"`
}

type thread struct {
	reasoning              ReasoningConfig
	tools                  map[string]ToolDefinition
	structuredOutputSchema *JsonSchema
	structuredOutputStrict *bool
	maxWebSearches         int
	webFetchEnabled        bool
	handleToolFunction     func(name string, args string) string
	updateOnFinalize       bool
	coalesceTextBlocks     bool

	success bool
	err     string
	result  ThreadUsage

	model    string
	threadId string

	blocks []*ThreadBlock

	updated         bool
	currentProvider string
}

type ThreadUsage struct {
	CacheReadTokens  int64
	CacheWriteTokens int64
	InputTokens      int64
	OutputTokens     int64
	WebSearches      int
	PageViews        int
}

func newThread() *thread {
	return &thread{}
}

// Internal helper methods

func (t *thread) create(id string, typ ThreadBlockType) *ThreadBlock {
	b := &ThreadBlock{
		ID:         id,
		Type:       typ,
		ProviderID: t.currentProvider,
	}
	t.blocks = append(t.blocks, b)
	return b
}

func (t *thread) appendBlockId(typ ThreadBlockType) string {
	return fmt.Sprintf("%s-%d", typ, len(t.blocks)+1)
}

func (t *thread) completeBlock(id string) {
	for blockIdx := range t.blocks {
		if t.blocks[blockIdx].ID == id {
			t.blocks[blockIdx].Complete = true
			if t.updateOnFinalize {
				t.updated = true
			}
		}
	}
}

func (t *thread) getType(id string, ofType ThreadBlockType) *ThreadBlock {
	blockIdx := len(t.blocks) - 1
	for blockIdx >= 0 {
		if t.blocks[blockIdx].Type == ofType && t.blocks[blockIdx].ID == id {
			return t.blocks[blockIdx]
		}
		blockIdx--
	}
	return nil
}

func (t *thread) findOrCreateIDBlock(id string, typ ThreadBlockType) *ThreadBlock {
	blockIdx := len(t.blocks) - 1
	for blockIdx >= 0 {
		if t.blocks[blockIdx].Type == typ && t.blocks[blockIdx].ID == id {
			return t.blocks[blockIdx]
		}
		blockIdx--
	}
	if t.coalesceTextBlocks && typ == InferenceBlockText {
		if block := t.appendCoalesce(id, typ); block != nil {
			return block
		}
	}
	return t.create(id, typ)
}

func (t *thread) appendCoalesce(id string, typ ThreadBlockType) *ThreadBlock {
	searchIdx := len(t.blocks) - 1
	if searchIdx < 0 {
		return nil
	}
	if t.blocks[searchIdx].Type != typ {
		return nil
	}
	t.blocks[searchIdx].Continued = true
	b := t.create(id, typ)
	return b
}

func (t *thread) setError(err error) {
	t.err = err.Error()
	t.success = false
}
