package aikit

type StreamEventKind int

const (
	EventBlockNew       StreamEventKind = iota // New block appended
	EventBlockUpdated                          // Existing block received content
	EventBlockCompleted                        // Block.Complete transitioned to true
	EventError                                 // Error occurred
	EventDone                                  // Stream finished successfully
)

type StreamEvent struct {
	Kind    StreamEventKind
	Block   *ThreadBlock // Set for BlockNew, BlockUpdated, BlockCompleted
	Session *Session     // Always set
	Error   error        // Set for EventError
}
