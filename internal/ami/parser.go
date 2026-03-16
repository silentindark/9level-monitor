package ami

import (
	"bufio"
	"strings"
)

// ParseEvents reads AMI text and yields events.
// AMI events/responses are blocks of "Key: Value\r\n" lines separated by "\r\n\r\n".
func ParseEvents(data string) []Event {
	var events []Event
	blocks := splitBlocks(data)
	for _, block := range blocks {
		if ev := parseBlock(block); len(ev) > 0 {
			events = append(events, ev)
		}
	}
	return events
}

func splitBlocks(data string) []string {
	// AMI separates messages with blank lines
	var blocks []string
	var current strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimRight(line, "\r")
		if line == "" {
			if current.Len() > 0 {
				blocks = append(blocks, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	if current.Len() > 0 {
		blocks = append(blocks, current.String())
	}
	return blocks
}

func parseBlock(block string) Event {
	ev := make(Event)
	scanner := bufio.NewScanner(strings.NewReader(block))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, ": ")
		if idx < 0 {
			// Try without space (e.g., "Key:Value")
			idx = strings.Index(line, ":")
			if idx < 0 {
				continue
			}
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			ev[key] = val
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+2:])
		ev[key] = val
	}
	return ev
}
