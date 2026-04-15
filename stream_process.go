package aikit

import (
	"io"
	"log"
)

// processStream pulls chunks from the transport, passes each through
// provider.OnChunk, and emits block lifecycle events (new, updated,
// completed). Returns nil on clean completion, or an error.
func processStream(
	transport Transport,
	provider APIRequest,
	state *StreamState,
	blocks func() []*ThreadBlock,
	emit func(StreamEventKind, *ThreadBlock, error),
	debug bool,
) error {
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
			return nil
		}
		if err != nil {
			return err
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
			emit(EventBlockNew, blks[i], nil)
			emittedAny = true
			if blks[i].Complete {
				completedSet[i] = true
			}
		}
		prevBlockCount = len(blks)

		// Detect newly completed blocks.
		for i := range blks {
			if blks[i].Complete && !completedSet[i] {
				emit(EventBlockCompleted, blks[i], nil)
				completedSet[i] = true
				emittedAny = true
			}
		}

		// If something updated but no new/completed events, emit update.
		if updated && !emittedAny {
			for i := len(blks) - 1; i >= 0; i-- {
				if !blks[i].Complete {
					emit(EventBlockUpdated, blks[i], nil)
					break
				}
			}
		}

		if result.Error != nil {
			return result.Error
		}
		if result.Done {
			return nil
		}
	}
}
