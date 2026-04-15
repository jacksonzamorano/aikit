package aikit

import (
	"io"
	"log"
)

// processStream pulls chunks from the transport, passes each through
// provider.OnChunk, and emits block lifecycle events (new, updated,
// completed) on the returned channel. The channel is closed when the
// stream ends or an error occurs.
func processStream(
	transport Transport,
	provider APIRequest,
	state *StreamState,
	blocks func() []*ThreadBlock,
	debug bool,
) <-chan StreamEvent {
	ch := make(chan StreamEvent, 1)
	go func() {
		defer close(ch)

		blks := blocks()
		prevBlockCount := len(blks)
		completedSet := make(map[int]bool)
		for i, b := range blks {
			if b.Complete {
				completedSet[i] = true
			}
		}

		for {
			data, err := transport.ReadChunk()
			if err == io.EOF {
				return
			}
			if err != nil {
				ch <- StreamEvent{Kind: EventError, Error: err}
				return
			}

			if debug {
				log.Printf("[Session] SSE Event: %s", string(data))
			}

			result := provider.OnChunk(data, state)
			updated := state.TakeUpdate()
			blks = blocks()

			// Detect new blocks.
			emittedAny := false
			for i := prevBlockCount; i < len(blks); i++ {
				ch <- StreamEvent{Kind: EventBlockNew, Block: blks[i]}
				emittedAny = true
				if blks[i].Complete {
					completedSet[i] = true
				}
			}
			prevBlockCount = len(blks)

			// Detect newly completed blocks.
			for i := range blks {
				if blks[i].Complete && !completedSet[i] {
					ch <- StreamEvent{Kind: EventBlockCompleted, Block: blks[i]}
					completedSet[i] = true
					emittedAny = true
				}
			}

			// If something updated but no new/completed events, emit update.
			if updated && !emittedAny {
				for i := len(blks) - 1; i >= 0; i-- {
					if !blks[i].Complete {
						ch <- StreamEvent{Kind: EventBlockUpdated, Block: blks[i]}
						break
					}
				}
			}

			if result.Error != nil {
				ch <- StreamEvent{Kind: EventError, Error: result.Error}
				return
			}
			if result.Done {
				return
			}
		}
	}()
	return ch
}
