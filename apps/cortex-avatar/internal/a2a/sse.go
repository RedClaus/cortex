package a2a

import (
	"bufio"
	"io"
	"strings"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int
}

// SSEReader reads SSE events from a stream
type SSEReader struct {
	reader *bufio.Reader
}

// NewSSEReader creates a new SSE reader
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent reads the next SSE event
func (s *SSEReader) ReadEvent() (*SSEEvent, error) {
	event := &SSEEvent{
		Event: "message", // default event type
	}

	var dataLines []string

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF && (len(dataLines) > 0 || event.Event != "message") {
				// Return partial event on EOF
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			return nil, err
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Empty line signals end of event
		if line == "" {
			if len(dataLines) > 0 || event.Event != "message" || event.ID != "" {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			continue
		}

		// Parse field
		if strings.HasPrefix(line, ":") {
			// Comment, ignore
			continue
		}

		colonIdx := strings.Index(line, ":")
		var field, value string

		if colonIdx == -1 {
			field = line
			value = ""
		} else {
			field = line[:colonIdx]
			value = line[colonIdx+1:]
			// Remove leading space from value if present
			if len(value) > 0 && value[0] == ' ' {
				value = value[1:]
			}
		}

		switch field {
		case "event":
			event.Event = value
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			event.ID = value
		case "retry":
			// Parse retry value (milliseconds)
			// For simplicity, we ignore parsing errors
		}
	}
}
