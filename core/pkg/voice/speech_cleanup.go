// Package voice provides voice-related types and utilities for Cortex.
// speech_cleanup.go provides STT output normalization and intent extraction.
//
// Problem: STT often picks up noise, partial words, or artifacts before/after
// the actual user query. Examples:
//   - "very internet and get me the latest headlines" -> "get me the latest headlines"
//   - "um uh hey cortex what's the weather" -> "what's the weather"
//   - "okay so like can you help me with" -> "can you help me with"
package voice

import (
	"regexp"
	"strings"
	"unicode"
)

// SpeechCleaner normalizes and cleans STT output to extract user intent.
type SpeechCleaner struct {
	// Patterns that commonly appear as STT artifacts/noise
	noisePatterns []*regexp.Regexp

	// Filler words that can be removed
	fillerWords map[string]bool

	// Common misheard prefixes (STT artifacts before actual speech)
	artifactPrefixes []string

	// Wake words that signal start of actual command
	wakeWords []string

	// Command starters that indicate beginning of intent
	commandStarters []string

	// CR-012-C: Misheard word corrections (STT often mishears short commands)
	misheardCorrections map[string]string
}

// NewSpeechCleaner creates a new speech cleaner with default patterns.
func NewSpeechCleaner() *SpeechCleaner {
	sc := &SpeechCleaner{
		// CR-012-C: Common STT misheard word corrections
		// These are phrases that Whisper commonly mishears, mapped to their intended meaning
		misheardCorrections: map[string]string{
			// "proceed" is often misheard as various phrases
			"i've never seen": "proceed",
			"i never seen":    "proceed",
			"i've never see":  "proceed",
			"i've ever seen":  "proceed",
			"i never see":     "proceed",
			"provide":         "proceed", // when spoken quickly
			"perceid":         "proceed",
			"per seed":        "proceed",
			// "continue" misheard variants
			"can't continue": "continue", // when said quickly
			"can continue":   "continue",
			// "go ahead" misheard variants
			"go head": "go ahead",
			"go had":  "go ahead",
			"go at":   "go ahead",
			// "yes" misheard variants
			"guess": "yes", // when said quickly
			"yet":   "yes",
			// "stop" misheard variants
			"stock": "stop",
			"top":   "stop", // when 's' is quiet
			// "cancel" misheard variants
			"council": "cancel",
			"counsel": "cancel",
			// "help" misheard variants
			"health": "help",
			"held":   "help",
			// Common confirmation misheards
			"do it":  "do it",
			"due it": "do it",
		},
		// Filler words that can be safely removed
		fillerWords: map[string]bool{
			"um":      true,
			"uh":      true,
			"uhm":     true,
			"hmm":     true,
			"hm":      true,
			"er":      true,
			"ah":      true,
			"eh":      true,
			"like":    true,
			"so":      true,
			"well":    true,
			"okay":    true,
			"ok":      true,
			"alright": true,
			"right":   true,
			"yeah":    true,
			// Note: "yes", "no", "ok" are NOT filler words - they're valid responses
			"wait":  true,
			"let":   true,
			"me":    true,
			"see":   true,
			"and":   true,
			"but":   true,
			"the":   true, // when at very start before command
			"a":     true, // when at very start before command
			"noise": true, // literal noise word
		},

		// Common STT artifact prefixes (misheard noise)
		artifactPrefixes: []string{
			"very internet and",
			"the internet and",
			"very and",
			"and and",
			"but but",
			"i i",
			"the the",
			"a a",
			"to to",
			"in the and",
			"so so",
			"very",      // when followed by non-adjective
			"berry",     // misheard "very"
			"bury",      // misheard "very"
			"vary",      // misheard "very"
			"fairy",     // misheard filler
			"ferry",     // misheard filler
			"sorry and", // false start
			"sorry but",
		},

		// Wake words that signal actual command start
		// Includes both male (Henry) and female (Hannah) voice personas
		wakeWords: []string{
			// Henry (male voice)
			"hey henry",
			"hi henry",
			"henry",
			// Hannah/Hanna (female voice) - include variants STT might transcribe
			"hey hannah",
			"hi hannah",
			"hannah",
			"hey hanna",
			"hi hanna",
			"hanna",
			// Generic
			"hey cortex",
			"hi cortex",
			"cortex",
			"assistant",
			"computer",
		},

		// Command starters - phrases that typically begin actual requests
		commandStarters: []string{
			"get me",
			"show me",
			"tell me",
			"find me",
			"give me",
			"can you",
			"could you",
			"would you",
			"will you",
			"please",
			"i want",
			"i need",
			"i'd like",
			"i would like",
			"what is",
			"what's",
			"what are",
			"where is",
			"where's",
			"where are",
			"when is",
			"when's",
			"when are",
			"who is",
			"who's",
			"who are",
			"how do",
			"how does",
			"how can",
			"how to",
			"why is",
			"why does",
			"why do",
			"search for",
			"look up",
			"look for",
			"find",
			"open",
			"start",
			"stop",
			"play",
			"pause",
			"help me",
			"help with",
			"explain",
			"describe",
			"summarize",
			"list",
			"create",
			"make",
			"write",
			"read",
			"check",
			"set",
			"turn on",
			"turn off",
		},
	}

	// Compile noise patterns
	sc.noisePatterns = []*regexp.Regexp{
		// Multiple punctuation
		regexp.MustCompile(`[.!?]{2,}`),
		// Orphan punctuation
		regexp.MustCompile(`^\s*[,.:;!?]\s*`),
		// Leading numbers that are likely timestamps/noise
		regexp.MustCompile(`^\d{1,2}:\d{2}\s*`),
		// Music/sound indicators from STT
		regexp.MustCompile(`(?i)^\s*\[.*?\]\s*`),
		regexp.MustCompile(`(?i)^\s*\(.*?\)\s*`),
		// Trailing incomplete words (single letters except I/a)
		regexp.MustCompile(`\s+[b-hj-z]\s*$`),
	}

	return sc
}

// Clean normalizes STT output and extracts the likely user intent.
func (sc *SpeechCleaner) Clean(text string) string {
	if text == "" {
		return ""
	}

	// Step 1: Basic normalization
	text = sc.normalizeText(text)

	// Step 2: Remove noise patterns
	text = sc.removeNoisePatterns(text)

	// Step 3: Remove artifact prefixes
	text = sc.removeArtifactPrefixes(text)

	// Step 4: Extract command after wake word
	text = sc.extractAfterWakeWord(text)

	// Step 5: Find command starter and extract from there
	text = sc.extractFromCommandStarter(text)

	// Step 6: Remove leading filler words
	text = sc.removeLeadingFillers(text)

	// Step 7: CR-012-C: Correct commonly misheard words
	text = sc.correctMisheardWords(text)

	// Step 8: Final cleanup
	text = sc.finalCleanup(text)

	return text
}

// correctMisheardWords fixes commonly misheard STT transcriptions.
// CR-012-C: Whisper often mishears short commands like "proceed" as longer phrases.
func (sc *SpeechCleaner) correctMisheardWords(text string) string {
	if len(sc.misheardCorrections) == 0 {
		return text
	}

	lower := strings.ToLower(text)
	words := strings.Fields(lower)
	originalWords := strings.Fields(text)
	result := text

	// First, apply multi-word corrections (these are safe for any length input)
	// e.g., "i've never seen" -> "proceed"
	for wrong, correct := range sc.misheardCorrections {
		if strings.Contains(wrong, " ") && strings.Contains(strings.ToLower(result), wrong) {
			// Case-insensitive replacement
			lowerResult := strings.ToLower(result)
			idx := strings.Index(lowerResult, wrong)
			if idx >= 0 {
				result = result[:idx] + correct + result[idx+len(wrong):]
			}
		}
	}

	// For short inputs (<=3 words), also apply single-word corrections
	// This prevents "stock prices" -> "stop prices" but allows "stock" -> "stop"
	if len(words) <= 3 {
		// Re-parse after multi-word corrections
		correctedWords := strings.Fields(result)

		for i, word := range correctedWords {
			cleanWord := strings.ToLower(strings.Trim(word, ",.!?"))
			if correct, ok := sc.misheardCorrections[cleanWord]; ok {
				// Only apply if it's a single-word correction (no spaces in the key)
				if !strings.Contains(cleanWord, " ") {
					// Preserve punctuation from original
					prefix := ""
					suffix := ""
					// Extract leading punctuation
					for _, r := range word {
						if unicode.IsPunct(r) {
							prefix += string(r)
						} else {
							break
						}
					}
					// Extract trailing punctuation
					for j := len(word) - 1; j >= 0; j-- {
						r := rune(word[j])
						if unicode.IsPunct(r) {
							suffix = string(r) + suffix
						} else {
							break
						}
					}
					correctedWords[i] = prefix + correct + suffix
				}
			}
		}
		result = strings.Join(correctedWords, " ")
	}

	// Handle two-word phrases that should be corrected (like "go head" -> "go ahead")
	// These are stored as single entries with spaces
	if len(originalWords) == 2 {
		twoWordPhrase := strings.ToLower(strings.Join(originalWords, " "))
		if correct, ok := sc.misheardCorrections[twoWordPhrase]; ok {
			result = correct
		}
	}

	return result
}

// normalizeText performs basic text normalization.
func (sc *SpeechCleaner) normalizeText(text string) string {
	// Trim whitespace
	text = strings.TrimSpace(text)

	// Normalize multiple spaces
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Remove leading/trailing quotes that STT sometimes adds
	text = strings.Trim(text, `"'`)

	return text
}

// removeNoisePatterns removes common STT noise patterns.
func (sc *SpeechCleaner) removeNoisePatterns(text string) string {
	// Remove repeated words at start (e.g., "the the" -> "the")
	text = sc.removeRepeatedWords(text)

	for _, pattern := range sc.noisePatterns {
		text = pattern.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

// removeRepeatedWords removes consecutive repeated words.
func (sc *SpeechCleaner) removeRepeatedWords(text string) string {
	words := strings.Fields(text)
	if len(words) < 2 {
		return text
	}

	result := []string{words[0]}
	for i := 1; i < len(words); i++ {
		// Compare lowercase versions
		if strings.ToLower(words[i]) != strings.ToLower(words[i-1]) {
			result = append(result, words[i])
		}
	}

	return strings.Join(result, " ")
}

// removeArtifactPrefixes removes known STT artifact prefixes.
func (sc *SpeechCleaner) removeArtifactPrefixes(text string) string {
	lower := strings.ToLower(text)

	for _, prefix := range sc.artifactPrefixes {
		if strings.HasPrefix(lower, prefix) {
			// Check if removing this prefix leaves a valid command
			remaining := strings.TrimSpace(text[len(prefix):])
			if sc.looksLikeValidCommand(remaining) {
				text = remaining
				lower = strings.ToLower(text)
			}
		}
	}

	return text
}

// HasWakeWord checks if the text contains any wake word.
func (sc *SpeechCleaner) HasWakeWord(text string) bool {
	lower := strings.ToLower(text)
	for _, wake := range sc.wakeWords {
		if strings.Contains(lower, wake) {
			return true
		}
	}
	return false
}

// extractAfterWakeWord finds wake word and extracts text after it.
func (sc *SpeechCleaner) extractAfterWakeWord(text string) string {
	lower := strings.ToLower(text)

	for _, wake := range sc.wakeWords {
		idx := strings.Index(lower, wake)
		if idx != -1 {
			// Extract everything after wake word
			afterWake := strings.TrimSpace(text[idx+len(wake):])
			// Remove leading comma/punctuation after wake word
			afterWake = strings.TrimLeft(afterWake, ",.!? ")
			if afterWake != "" {
				return afterWake
			}
		}
	}

	return text
}

// extractFromCommandStarter finds a command starter and extracts from there.
func (sc *SpeechCleaner) extractFromCommandStarter(text string) string {
	lower := strings.ToLower(text)
	words := strings.Fields(lower)

	// If text already starts with command starter, return as-is
	for _, starter := range sc.commandStarters {
		if strings.HasPrefix(lower, starter) {
			return text
		}
	}

	// Look for command starter later in the text
	bestIdx := -1

	for _, starter := range sc.commandStarters {
		idx := strings.Index(lower, starter)
		if idx > 0 && (bestIdx == -1 || idx < bestIdx) {
			// Verify it's at a word boundary
			if idx == 0 || !unicode.IsLetter(rune(lower[idx-1])) {
				bestIdx = idx
			}
		}
	}

	if bestIdx > 0 {
		// Check if the prefix looks like noise vs important context
		prefix := strings.TrimSpace(text[:bestIdx])
		prefixWords := strings.Fields(strings.ToLower(prefix))

		// If prefix is short and contains mostly filler/noise, remove it
		if len(prefixWords) <= 3 && sc.isProbablyNoise(prefix) {
			return strings.TrimSpace(text[bestIdx:])
		}

		// If prefix is "and" or "but" followed by command, remove connector
		if len(words) > 0 && (prefixWords[len(prefixWords)-1] == "and" || prefixWords[len(prefixWords)-1] == "but") {
			return strings.TrimSpace(text[bestIdx:])
		}
	}

	return text
}

// removeLeadingFillers removes filler words from the start.
func (sc *SpeechCleaner) removeLeadingFillers(text string) string {
	words := strings.Fields(text)
	startIdx := 0

	for i, word := range words {
		cleanWord := strings.ToLower(strings.Trim(word, ",.!?"))
		if sc.fillerWords[cleanWord] {
			startIdx = i + 1
		} else {
			break
		}
		// Don't remove more than 6 filler words
		if startIdx >= 6 {
			break
		}
	}

	if startIdx > 0 && startIdx < len(words) {
		return strings.Join(words[startIdx:], " ")
	}

	// If all words are filler words, check if it's a valid confirmation/denial response
	if startIdx >= len(words) {
		// Preserve single-word confirmation/denial responses
		if len(words) == 1 {
			cleanWord := strings.ToLower(strings.Trim(words[0], ",.!?"))
			// These are valid standalone responses, not filler
			validResponses := map[string]bool{
				"yes": true, "yeah": true, "yep": true, "yup": true,
				"no": true, "nope": true, "nah": true,
				"ok": true, "okay": true, "sure": true,
				"stop": true, "cancel": true, "wait": true,
				"go": true, "do": true, "done": true,
			}
			if validResponses[cleanWord] {
				return text // Keep the original text
			}
		}
		return ""
	}

	return text
}

// finalCleanup performs final text cleanup.
func (sc *SpeechCleaner) finalCleanup(text string) string {
	// Normalize whitespace again
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Ensure first letter is capitalized
	if len(text) > 0 {
		runes := []rune(text)
		runes[0] = unicode.ToUpper(runes[0])
		text = string(runes)
	}

	return text
}

// looksLikeValidCommand checks if text appears to be a valid command.
func (sc *SpeechCleaner) looksLikeValidCommand(text string) bool {
	if len(text) < 3 {
		return false
	}

	lower := strings.ToLower(text)

	// Check if starts with command starter
	for _, starter := range sc.commandStarters {
		if strings.HasPrefix(lower, starter) {
			return true
		}
	}

	// Check if contains reasonable words (not just noise)
	words := strings.Fields(lower)
	if len(words) < 2 {
		return false
	}

	// At least one word should be > 3 chars
	hasSubstantialWord := false
	for _, w := range words {
		if len(w) > 3 {
			hasSubstantialWord = true
			break
		}
	}

	return hasSubstantialWord
}

// isProbablyNoise checks if text is likely STT noise.
func (sc *SpeechCleaner) isProbablyNoise(text string) bool {
	lower := strings.ToLower(text)
	words := strings.Fields(lower)

	if len(words) == 0 {
		return true
	}

	noiseCount := 0
	for _, word := range words {
		cleanWord := strings.Trim(word, ",.!?")
		if sc.fillerWords[cleanWord] || len(cleanWord) <= 2 {
			noiseCount++
		}
		// Check for artifact prefix words
		for _, prefix := range sc.artifactPrefixes {
			if strings.Contains(prefix, cleanWord) {
				noiseCount++
				break
			}
		}
	}

	// If more than half the words are noise, it's probably noise
	return noiseCount > len(words)/2
}

// CleanTranscription is a convenience function using default cleaner.
func CleanTranscription(text string) string {
	return defaultCleaner.Clean(text)
}

// HasWakeWord checks if the text contains a wake word.
func HasWakeWord(text string) bool {
	return defaultCleaner.HasWakeWord(text)
}

// CleanTranscriptionWithInfo returns both the cleaned text and whether a wake word was found.
func CleanTranscriptionWithInfo(text string) (cleaned string, hadWakeWord bool) {
	hadWakeWord = defaultCleaner.HasWakeWord(text)
	cleaned = defaultCleaner.Clean(text)
	return
}

var defaultCleaner = NewSpeechCleaner()

// AddArtifactPrefix adds a custom artifact prefix to filter.
func (sc *SpeechCleaner) AddArtifactPrefix(prefix string) {
	sc.artifactPrefixes = append(sc.artifactPrefixes, strings.ToLower(prefix))
}

// AddWakeWord adds a custom wake word.
func (sc *SpeechCleaner) AddWakeWord(word string) {
	sc.wakeWords = append(sc.wakeWords, strings.ToLower(word))
}

// AddCommandStarter adds a custom command starter phrase.
func (sc *SpeechCleaner) AddCommandStarter(starter string) {
	sc.commandStarters = append(sc.commandStarters, strings.ToLower(starter))
}

// AddMisheardCorrection adds a custom misheard word correction.
// CR-012-C: Use this to add corrections for words that are commonly misheard in your domain.
func (sc *SpeechCleaner) AddMisheardCorrection(misheard, correct string) {
	if sc.misheardCorrections == nil {
		sc.misheardCorrections = make(map[string]string)
	}
	sc.misheardCorrections[strings.ToLower(misheard)] = correct
}
