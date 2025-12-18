package aikit

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type sseEvent struct {
	event string
	data  []byte
}

type ProviderError struct {
	Data  string
	Cause string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("-- Error --\n\tError: %s\n\tData: %s\n------------", e.Cause, e.Data)
}

func readSSE(r io.Reader, onEvent func(sseEvent) (bool, error)) *ProviderError {
	br := bufio.NewReader(r)
	var ev sseEvent
	var data bytes.Buffer

	flush := func() (bool, error) {
		if ev.event == "" && data.Len() == 0 {
			return true, nil
		}
		ev.data = bytes.TrimRight(data.Bytes(), "\n")
		data.Reset()
		if len(ev.data) == 0 && ev.event == "" {
			return true, nil
		}
		cont, handlerErr := onEvent(ev)
		if handlerErr != nil {
			if err, ok := handlerErr.(*ProviderError); ok {
				return false, err
			} else {
				return false, &ProviderError{
					Cause: handlerErr.Error(),
					Data:  string(ev.data),
				}
			}
		}
		ev = sseEvent{}
		return cont, nil
	}

	for {
		line, err := br.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return &ProviderError{
				Cause: err.Error(),
				Data:  line,
			}
		}

		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = strings.TrimSuffix(line, "\n")
		}
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = strings.TrimSuffix(line, "\r")
		}

		if line == "" {
			cont, err := flush()
			if err != nil {
				if err, ok := err.(*ProviderError); ok {
					return err
				} else {
					return &ProviderError{
						Cause: err.Error(),
						Data:  string(ev.data),
					}
				}
			}
			if !cont {
				return nil
			}
		} else if strings.HasPrefix(line, ":") {
			// comment/keepalive
		} else if strings.HasPrefix(line, "event:") {
			ev.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			chunk := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			data.WriteString(chunk)
			data.WriteString("\n")
		}

		if errors.Is(err, io.EOF) {
			flush()
			return nil
		}
	}
}
