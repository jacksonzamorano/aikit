package aikit

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
)

type sseChunkOrErr struct {
	data []byte
	err  error
}

// readSSE parses Server-Sent Events from r and returns a channel of raw
// data chunks. Protocol-level concerns (empty data, [DONE] sentinel,
// comments/keepalives) are handled internally. The channel is closed
// when the stream ends or an error is sent.
func readSSE(provider string, r io.Reader) <-chan sseChunkOrErr {
	ch := make(chan sseChunkOrErr, 1)
	go func() {
		defer close(ch)

		br := bufio.NewReader(r)
		var eventType string
		var data bytes.Buffer

		flush := func() {
			raw := bytes.TrimRight(data.Bytes(), "\n")
			data.Reset()
			defer func() { eventType = "" }()

			if len(raw) == 0 && eventType == "" {
				return
			}
			if len(raw) == 0 {
				return
			}
			if string(raw) == "[DONE]" {
				return
			}

			// Copy — the buffer is reused across events.
			cp := make([]byte, len(raw))
			copy(cp, raw)
			ch <- sseChunkOrErr{data: cp}
		}

		for {
			line, err := br.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				ch <- sseChunkOrErr{err: &AIError{
					Category: AIErrorCategoryStreamingError,
					Provider: provider,
					Message:  err.Error(),
				}}
				return
			}

			line = strings.TrimRight(line, "\r\n")

			if line == "" {
				flush()
			} else if strings.HasPrefix(line, ":") {
				// comment/keepalive
			} else if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				chunk := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				data.WriteString(chunk)
				data.WriteString("\n")
			}

			if errors.Is(err, io.EOF) {
				flush()
				return
			}
		}
	}()
	return ch
}
