// Package voice provides voice-related types and utilities for Cortex.
// response_formatter.go converts text responses to voice-friendly format (CR-012-A).
package voice

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// VoiceResponseFormatter converts text responses to voice-friendly format.
type VoiceResponseFormatter struct {
	// Convert technical formats to spoken forms
	SpokenNumbers bool
	SpokenDates   bool
	SpokenPaths   bool

	// Limit response length
	MaxSentences int

	// Add natural speech elements
	NaturalTransitions bool
}

// DefaultVoiceResponseFormatter returns production configuration.
func DefaultVoiceResponseFormatter() *VoiceResponseFormatter {
	return &VoiceResponseFormatter{
		SpokenNumbers:      true,
		SpokenDates:        true,
		SpokenPaths:        true,
		MaxSentences:       4,
		NaturalTransitions: true,
	}
}

// Format converts a text response to voice-optimized format.
func (f *VoiceResponseFormatter) Format(text string) string {
	result := text

	// Strip markdown first
	result = StripMarkdown(result)

	// Handle code blocks
	result = HandleCodeBlocks(result)

	if f.SpokenDates {
		result = f.convertDates(result)
	}

	if f.SpokenNumbers {
		result = f.convertNumbers(result)
	}

	if f.SpokenPaths {
		result = f.simplifyPaths(result)
	}

	if f.MaxSentences > 0 {
		result = f.limitSentences(result)
	}

	return strings.TrimSpace(result)
}

// convertDates converts date formats to spoken form.
func (f *VoiceResponseFormatter) convertDates(text string) string {
	// Convert ISO dates: 2025-01-15 -> January 15, 2025
	isoDate := regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)
	text = isoDate.ReplaceAllStringFunc(text, func(match string) string {
		parts := isoDate.FindStringSubmatch(match)
		if len(parts) == 4 {
			year, _ := strconv.Atoi(parts[1])
			month, _ := strconv.Atoi(parts[2])
			day, _ := strconv.Atoi(parts[3])

			t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			return t.Format("January 2, 2006")
		}
		return match
	})

	// Convert slash dates: 1/15/2025 -> January 15
	slashDate := regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{2,4})\b`)
	text = slashDate.ReplaceAllStringFunc(text, func(match string) string {
		parts := slashDate.FindStringSubmatch(match)
		if len(parts) == 4 {
			month, _ := strconv.Atoi(parts[1])
			day, _ := strconv.Atoi(parts[2])
			year, _ := strconv.Atoi(parts[3])
			if year < 100 {
				year += 2000
			}

			t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			return t.Format("January 2")
		}
		return match
	})

	// Convert 24-hour time: 15:30 -> 3:30 PM
	time24 := regexp.MustCompile(`\b(\d{2}):(\d{2})\b`)
	text = time24.ReplaceAllStringFunc(text, func(match string) string {
		parts := time24.FindStringSubmatch(match)
		if len(parts) == 3 {
			hour, _ := strconv.Atoi(parts[1])
			minute, _ := strconv.Atoi(parts[2])

			period := "AM"
			if hour >= 12 {
				period = "PM"
				if hour > 12 {
					hour -= 12
				}
			}
			if hour == 0 {
				hour = 12
			}

			if minute == 0 {
				return strconv.Itoa(hour) + " " + period
			}
			return strconv.Itoa(hour) + ":" + parts[2] + " " + period
		}
		return match
	})

	return text
}

// convertNumbers converts large numbers to spoken form.
func (f *VoiceResponseFormatter) convertNumbers(text string) string {
	// Convert exact large numbers to approximate spoken form
	// 2048 -> "about 2 thousand"
	// 1536000 -> "about 1.5 million"

	largeNum := regexp.MustCompile(`\b(\d{4,})\b`)
	text = largeNum.ReplaceAllStringFunc(text, func(match string) string {
		n, err := strconv.ParseInt(match, 10, 64)
		if err != nil {
			return match
		}

		// Keep small numbers exact
		if n < 1000 {
			return match
		}

		// Approximate large numbers
		switch {
		case n >= 1000000000:
			billions := float64(n) / 1000000000
			return f.spokenNumber(billions) + " billion"
		case n >= 1000000:
			millions := float64(n) / 1000000
			return f.spokenNumber(millions) + " million"
		case n >= 1000:
			thousands := float64(n) / 1000
			return f.spokenNumber(thousands) + " thousand"
		default:
			return match
		}
	})

	return text
}

func (f *VoiceResponseFormatter) spokenNumber(n float64) string {
	// Round to 1 decimal place if needed
	if n == float64(int(n)) {
		return strconv.Itoa(int(n))
	}
	if n < 10 {
		return "about " + strconv.FormatFloat(n, 'f', 1, 64)
	}
	return "about " + strconv.Itoa(int(n+0.5))
}

// simplifyPaths converts file paths to spoken references.
func (f *VoiceResponseFormatter) simplifyPaths(text string) string {
	// Don't read out full paths like /usr/local/bin/something
	// Instead: "the something binary" or just "something"

	// Keep paths if they're short or clearly intentional
	// Only simplify very long paths (4+ directories deep)

	longPath := regexp.MustCompile(`(/[a-zA-Z0-9_.-]+){4,}`)
	text = longPath.ReplaceAllStringFunc(text, func(match string) string {
		parts := strings.Split(match, "/")
		filename := parts[len(parts)-1]

		// Return just the filename for voice
		return filename
	})

	return text
}

// limitSentences truncates response to max sentences.
func (f *VoiceResponseFormatter) limitSentences(text string) string {
	// Split on sentence endings
	sentenceEnd := regexp.MustCompile(`[.!?]+\s+`)
	sentences := sentenceEnd.Split(text, -1)

	if len(sentences) <= f.MaxSentences {
		return text
	}

	// Keep first N sentences
	kept := sentences[:f.MaxSentences]
	result := strings.Join(kept, ". ")

	if !strings.HasSuffix(result, ".") && !strings.HasSuffix(result, "!") && !strings.HasSuffix(result, "?") {
		result += "."
	}

	return result
}

// AddAcknowledgment prepends a natural acknowledgment to the response.
func (f *VoiceResponseFormatter) AddAcknowledgment(text string, ackType string) string {
	var ack string

	switch ackType {
	case "confirm":
		ack = "Got it. "
	case "working":
		ack = "On it. "
	case "done":
		ack = "Done. "
	case "found":
		ack = "Found it. "
	case "error":
		ack = "Hmm. "
	case "checking":
		ack = "Let me check. "
	default:
		return text
	}

	return ack + text
}

// StripMarkdown removes markdown formatting from text.
func StripMarkdown(text string) string {
	// Remove **bold**
	text = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(text, "$1")
	// Remove *italic*
	text = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(text, "$1")
	// Remove __bold__
	text = regexp.MustCompile(`__(.+?)__`).ReplaceAllString(text, "$1")
	// Remove _italic_
	text = regexp.MustCompile(`_(.+?)_`).ReplaceAllString(text, "$1")
	// Remove `code`
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "$1")
	// Remove headers (# ## ### etc)
	text = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(text, "")
	// Remove bullet points (- or *)
	text = regexp.MustCompile(`(?m)^[-*]\s+`).ReplaceAllString(text, "")
	// Remove numbered lists
	text = regexp.MustCompile(`(?m)^\d+\.\s+`).ReplaceAllString(text, "")
	// Remove horizontal rules
	text = regexp.MustCompile(`(?m)^---+$`).ReplaceAllString(text, "")
	// Remove links [text](url) -> text
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1")

	return text
}

// HandleCodeBlocks replaces code blocks with spoken descriptions.
func HandleCodeBlocks(text string) string {
	// Match code blocks ```language\ncode\n```
	codeBlock := regexp.MustCompile("```[a-z]*\n([\\s\\S]*?)\n```")

	return codeBlock.ReplaceAllStringFunc(text, func(match string) string {
		// Count lines in code block
		lines := strings.Count(match, "\n")

		if lines <= 3 {
			// Short code: extract and read (remove the backticks)
			inner := codeBlock.FindStringSubmatch(match)
			if len(inner) > 1 {
				code := strings.TrimSpace(inner[1])
				// Remove any remaining backticks
				code = strings.ReplaceAll(code, "`", "")
				return code
			}
		}

		// Long code: summarize
		return "[Code block with " + strconv.Itoa(lines) + " lines]"
	})
}

// FormatForTTS is a convenience function that applies all voice formatting.
func FormatForTTS(text string) string {
	formatter := DefaultVoiceResponseFormatter()
	return formatter.Format(text)
}
