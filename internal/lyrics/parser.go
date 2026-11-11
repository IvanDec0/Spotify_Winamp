package lyrics

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	timeTagPattern   = regexp.MustCompile(`\[(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?\]`)
	offsetTagPattern = regexp.MustCompile(`\[offset:([+-]?\d+)\]`)
)

// ParseLRC extracts timed lines from an LRC document. It accepts multiple
// timestamps per line, centisecond or millisecond fractions, metadata tags and
// an optional global [offset:ms] tag. Lines without a timestamp are ignored;
// callers use splitPlain for unsynced provider payloads.
func ParseLRC(raw string) []Line {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	offsetMS := 0
	if match := offsetTagPattern.FindStringSubmatch(raw); match != nil {
		if value, err := strconv.Atoi(match[1]); err == nil {
			offsetMS = value
		}
	}

	lines := make([]Line, 0, 64)
	for _, rawLine := range strings.Split(raw, "\n") {
		matches := timeTagPattern.FindAllStringSubmatchIndex(rawLine, -1)
		if len(matches) == 0 {
			continue
		}
		text := strings.TrimSpace(timeTagPattern.ReplaceAllString(rawLine, ""))
		if text == "" {
			continue
		}
		for _, match := range matches {
			minutes, errMinutes := strconv.Atoi(rawLine[match[2]:match[3]])
			seconds, errSeconds := strconv.Atoi(rawLine[match[4]:match[5]])
			if errMinutes != nil || errSeconds != nil {
				continue
			}
			fraction := 0
			if match[6] >= 0 {
				value := rawLine[match[6]:match[7]]
				fraction, _ = strconv.Atoi(value)
				switch len(value) {
				case 1:
					fraction *= 100
				case 2:
					fraction *= 10
				}
			}
			timeMS := minutes*60_000 + seconds*1_000 + fraction + offsetMS
			if timeMS < 0 {
				timeMS = 0
			}
			lines = append(lines, Line{TimeMS: timeMS, Text: text})
		}
	}

	sort.SliceStable(lines, func(left, right int) bool {
		return lines[left].TimeMS < lines[right].TimeMS
	})
	if len(lines) == 0 {
		return nil
	}
	return lines
}
