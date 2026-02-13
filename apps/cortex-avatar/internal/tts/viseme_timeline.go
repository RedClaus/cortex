// Package tts provides viseme timeline generation for lip-sync animations.
// This converts TTS phoneme/word timing data into the 15-Oculus viseme format
// expected by the frontend's TalkingHeadController.
package tts

import (
	"strings"
	"time"
)

// OculusViseme represents the 15 Oculus lip-sync viseme IDs
// These map directly to the frontend's TalkingHeadController
type OculusViseme int

const (
	VisemeOculusSil OculusViseme = 0  // Silence
	VisemeOculusPP  OculusViseme = 1  // p, b, m
	VisemeOculusFF  OculusViseme = 2  // f, v
	VisemeOculusTH  OculusViseme = 3  // th (dental)
	VisemeOculusDD  OculusViseme = 4  // t, d
	VisemeOculusKK  OculusViseme = 5  // k, g
	VisemeOculusCH  OculusViseme = 6  // ch, j, sh
	VisemeOculusSS  OculusViseme = 7  // s, z
	VisemeOculusNN  OculusViseme = 8  // n, l
	VisemeOculusRR  OculusViseme = 9  // r
	VisemeOculusAA  OculusViseme = 10 // a (as in "father")
	VisemeOculusE   OculusViseme = 11 // e (as in "bed")
	VisemeOculusIH  OculusViseme = 12 // i (as in "sit")
	VisemeOculusOH  OculusViseme = 13 // o (as in "go")
	VisemeOculusOU  OculusViseme = 14 // u (as in "boot")
)

// VisemeEvent represents a single viseme with timing for frontend playback
type VisemeEvent struct {
	VisemeID OculusViseme `json:"visemeId"`
	Time     float64      `json:"time"`   // Time in milliseconds from start
	Weight   float64      `json:"weight"` // Intensity 0-1
}

// VisemeTimeline represents a complete lip-sync animation timeline
type VisemeTimeline struct {
	Events   []VisemeEvent `json:"events"`
	Duration float64       `json:"duration"` // Total duration in milliseconds
}

// PhonemeToOculusViseme maps phoneme representations to Oculus viseme IDs
var PhonemeToOculusViseme = map[string]OculusViseme{
	// Silence
	"":  VisemeOculusSil,
	" ": VisemeOculusSil,

	// Bilabial stops - PP (p, b, m)
	"p": VisemeOculusPP, "b": VisemeOculusPP, "m": VisemeOculusPP,
	"P": VisemeOculusPP, "B": VisemeOculusPP, "M": VisemeOculusPP,

	// Labiodental fricatives - FF (f, v)
	"f": VisemeOculusFF, "v": VisemeOculusFF,
	"F": VisemeOculusFF, "V": VisemeOculusFF,

	// Dental fricatives - TH (th)
	"th": VisemeOculusTH, "TH": VisemeOculusTH,

	// Alveolar stops - DD (t, d)
	"t": VisemeOculusDD, "d": VisemeOculusDD,
	"T": VisemeOculusDD, "D": VisemeOculusDD,

	// Velar stops - KK (k, g)
	"k": VisemeOculusKK, "g": VisemeOculusKK, "c": VisemeOculusKK,
	"K": VisemeOculusKK, "G": VisemeOculusKK, "C": VisemeOculusKK,
	"q": VisemeOculusKK, "Q": VisemeOculusKK, "x": VisemeOculusKK,

	// Affricates/postalveolars - CH (ch, j, sh)
	"ch": VisemeOculusCH, "j": VisemeOculusCH, "sh": VisemeOculusCH,
	"CH": VisemeOculusCH, "J": VisemeOculusCH, "SH": VisemeOculusCH,

	// Sibilants - SS (s, z)
	"s": VisemeOculusSS, "z": VisemeOculusSS,
	"S": VisemeOculusSS, "Z": VisemeOculusSS,

	// Alveolar nasals/laterals - NN (n, l)
	"n": VisemeOculusNN, "l": VisemeOculusNN,
	"N": VisemeOculusNN, "L": VisemeOculusNN,

	// Retroflex - RR (r)
	"r": VisemeOculusRR, "R": VisemeOculusRR,

	// Vowels
	"a": VisemeOculusAA, "A": VisemeOculusAA,
	"e": VisemeOculusE, "E": VisemeOculusE,
	"i": VisemeOculusIH, "I": VisemeOculusIH,
	"o": VisemeOculusOH, "O": VisemeOculusOH,
	"u": VisemeOculusOU, "U": VisemeOculusOU,

	// Other consonants mapped to closest viseme
	"w": VisemeOculusOU, "W": VisemeOculusOU, // Rounded lips like 'u'
	"y": VisemeOculusIH, "Y": VisemeOculusIH, // Similar to 'i'
	"h": VisemeOculusAA, "H": VisemeOculusAA, // Open mouth
}

// GenerateVisemeTimeline creates a viseme timeline from TTS phoneme data
func GenerateVisemeTimeline(phonemes []Phoneme) *VisemeTimeline {
	if len(phonemes) == 0 {
		return &VisemeTimeline{
			Events:   []VisemeEvent{{VisemeID: VisemeOculusSil, Time: 0, Weight: 1.0}},
			Duration: 0,
		}
	}

	events := make([]VisemeEvent, 0, len(phonemes)+2)

	// Start with silence
	events = append(events, VisemeEvent{
		VisemeID: VisemeOculusSil,
		Time:     0,
		Weight:   1.0,
	})

	var maxTime float64

	for _, p := range phonemes {
		// Convert our Viseme type to Oculus viseme ID
		oculusID := visemeToOculusID(Viseme(p.Viseme))

		timeMs := float64(p.Start.Milliseconds())
		events = append(events, VisemeEvent{
			VisemeID: oculusID,
			Time:     timeMs,
			Weight:   0.8, // Default weight, can be adjusted based on phoneme intensity
		})

		endMs := float64(p.End.Milliseconds())
		if endMs > maxTime {
			maxTime = endMs
		}
	}

	// End with silence
	events = append(events, VisemeEvent{
		VisemeID: VisemeOculusSil,
		Time:     maxTime + 50, // 50ms after last phoneme
		Weight:   1.0,
	})

	return &VisemeTimeline{
		Events:   events,
		Duration: maxTime + 100, // Add buffer
	}
}

// GenerateVisemeTimelineFromText creates approximate visemes from raw text
// Used when phoneme data is not available from TTS provider
func GenerateVisemeTimelineFromText(text string, duration time.Duration) *VisemeTimeline {
	if len(text) == 0 {
		return &VisemeTimeline{
			Events:   []VisemeEvent{{VisemeID: VisemeOculusSil, Time: 0, Weight: 1.0}},
			Duration: 0,
		}
	}

	// Clean text - remove non-speaking characters
	cleanText := strings.TrimSpace(text)

	// Estimate speaking rate: ~15 characters per second for natural speech
	// This gives us a baseline timing
	charsPerMs := float64(len(cleanText)) / float64(duration.Milliseconds())
	if charsPerMs <= 0 {
		charsPerMs = 0.015 // Default: 15 chars/sec
	}

	events := make([]VisemeEvent, 0, len(cleanText)/2+2)

	// Start with silence
	events = append(events, VisemeEvent{
		VisemeID: VisemeOculusSil,
		Time:     0,
		Weight:   1.0,
	})

	currentTime := 50.0 // Start 50ms in

	// Process text character by character
	chars := []byte(cleanText)
	for i := 0; i < len(chars); i++ {
		ch := chars[i]

		// Skip whitespace (add small pause)
		if ch == ' ' || ch == '\n' || ch == '\t' {
			// Add silence for word boundaries
			events = append(events, VisemeEvent{
				VisemeID: VisemeOculusSil,
				Time:     currentTime,
				Weight:   0.5,
			})
			currentTime += 80 // 80ms pause between words
			continue
		}

		// Skip punctuation (add pause for sentence boundaries)
		if ch == '.' || ch == '!' || ch == '?' {
			events = append(events, VisemeEvent{
				VisemeID: VisemeOculusSil,
				Time:     currentTime,
				Weight:   1.0,
			})
			currentTime += 150 // Longer pause for sentence end
			continue
		}
		if ch == ',' || ch == ';' || ch == ':' {
			events = append(events, VisemeEvent{
				VisemeID: VisemeOculusSil,
				Time:     currentTime,
				Weight:   0.7,
			})
			currentTime += 100 // Medium pause for clauses
			continue
		}

		// Check for digraphs (th, ch, sh)
		var phoneme string
		if i < len(chars)-1 {
			digraph := string(chars[i : i+2])
			if digraph == "th" || digraph == "TH" ||
				digraph == "ch" || digraph == "CH" ||
				digraph == "sh" || digraph == "SH" {
				phoneme = digraph
				i++ // Skip next character
			}
		}
		if phoneme == "" {
			phoneme = string(ch)
		}

		// Look up viseme for this phoneme
		viseme, ok := PhonemeToOculusViseme[phoneme]
		if !ok {
			// Default to previous viseme or silence
			viseme = VisemeOculusSil
		}

		// Calculate duration for this phoneme
		// Vowels typically last longer than consonants
		phonemeDuration := 60.0 // Default 60ms
		if isVowel(ch) {
			phonemeDuration = 100.0 // Vowels: 100ms
		} else if ch == 's' || ch == 'z' || ch == 'f' || ch == 'v' {
			phonemeDuration = 80.0 // Fricatives: 80ms
		}

		events = append(events, VisemeEvent{
			VisemeID: viseme,
			Time:     currentTime,
			Weight:   0.8,
		})

		currentTime += phonemeDuration
	}

	// End with silence
	events = append(events, VisemeEvent{
		VisemeID: VisemeOculusSil,
		Time:     currentTime,
		Weight:   1.0,
	})

	// Adjust total duration
	totalDuration := currentTime + 50
	if float64(duration.Milliseconds()) > totalDuration {
		totalDuration = float64(duration.Milliseconds())
	}

	return &VisemeTimeline{
		Events:   events,
		Duration: totalDuration,
	}
}

// GenerateVisemeTimelineFromWordTimestamps creates visemes from word-level timestamps
// This is optimized for Cartesia's word_timestamps format
func GenerateVisemeTimelineFromWordTimestamps(words []string, startTimes, endTimes []float64) *VisemeTimeline {
	if len(words) == 0 {
		return &VisemeTimeline{
			Events:   []VisemeEvent{{VisemeID: VisemeOculusSil, Time: 0, Weight: 1.0}},
			Duration: 0,
		}
	}

	events := make([]VisemeEvent, 0, len(words)*4+2)

	// Start with silence
	events = append(events, VisemeEvent{
		VisemeID: VisemeOculusSil,
		Time:     0,
		Weight:   1.0,
	})

	var maxEndTime float64

	for i, word := range words {
		if i >= len(startTimes) || i >= len(endTimes) {
			break
		}

		startMs := startTimes[i] * 1000 // Convert seconds to ms
		endMs := endTimes[i] * 1000
		wordDuration := endMs - startMs

		if endMs > maxEndTime {
			maxEndTime = endMs
		}

		// Generate visemes for this word
		wordVisemes := textToVisemeSequence(word)
		numVisemes := len(wordVisemes)
		if numVisemes == 0 {
			continue
		}

		// Distribute visemes across word duration
		visemeDuration := wordDuration / float64(numVisemes)

		for j, viseme := range wordVisemes {
			visemeTime := startMs + float64(j)*visemeDuration
			events = append(events, VisemeEvent{
				VisemeID: viseme,
				Time:     visemeTime,
				Weight:   0.8,
			})
		}

		// Add brief silence at word boundary
		events = append(events, VisemeEvent{
			VisemeID: VisemeOculusSil,
			Time:     endMs,
			Weight:   0.3,
		})
	}

	// End with full silence
	events = append(events, VisemeEvent{
		VisemeID: VisemeOculusSil,
		Time:     maxEndTime + 50,
		Weight:   1.0,
	})

	return &VisemeTimeline{
		Events:   events,
		Duration: maxEndTime + 100,
	}
}

// textToVisemeSequence converts a word to a sequence of Oculus visemes
func textToVisemeSequence(text string) []OculusViseme {
	if len(text) == 0 {
		return nil
	}

	visemes := make([]OculusViseme, 0, len(text))
	chars := []byte(strings.ToLower(text))

	for i := 0; i < len(chars); i++ {
		ch := chars[i]

		// Skip non-letters
		if ch < 'a' || ch > 'z' {
			continue
		}

		// Check for digraphs
		var phoneme string
		if i < len(chars)-1 {
			next := chars[i+1]
			digraph := string([]byte{ch, next})
			if digraph == "th" || digraph == "ch" || digraph == "sh" {
				phoneme = digraph
				i++
			}
		}
		if phoneme == "" {
			phoneme = string(ch)
		}

		viseme, ok := PhonemeToOculusViseme[phoneme]
		if !ok {
			// For unknown characters, use a neutral mouth position
			viseme = VisemeOculusAA
		}

		// Avoid consecutive identical visemes
		if len(visemes) > 0 && visemes[len(visemes)-1] == viseme {
			continue
		}

		visemes = append(visemes, viseme)
	}

	return visemes
}

// visemeToOculusID converts our internal Viseme type to Oculus viseme ID
func visemeToOculusID(v Viseme) OculusViseme {
	switch v {
	case VisemeSilent:
		return VisemeOculusSil
	case VisemeAA:
		return VisemeOculusAA
	case VisemeEE:
		return VisemeOculusE
	case VisemeII:
		return VisemeOculusIH
	case VisemeOO:
		return VisemeOculusOH
	case VisemeUU:
		return VisemeOculusOU
	case VisemeFV:
		return VisemeOculusFF
	case VisemeTH:
		return VisemeOculusTH
	case VisemeMBP:
		return VisemeOculusPP
	case VisemeLNTD:
		return VisemeOculusDD
	case VisemeWQ:
		return VisemeOculusOU
	case VisemeSZ:
		return VisemeOculusSS
	case VisemeKG:
		return VisemeOculusKK
	case VisemeCHJ:
		return VisemeOculusCH
	case VisemeR:
		return VisemeOculusRR
	default:
		return VisemeOculusSil
	}
}

// isVowel checks if a character is a vowel
func isVowel(ch byte) bool {
	switch ch {
	case 'a', 'e', 'i', 'o', 'u', 'A', 'E', 'I', 'O', 'U':
		return true
	default:
		return false
	}
}

// ConvertToFrontendFormat converts VisemeTimeline to the format expected by frontend
func (vt *VisemeTimeline) ConvertToFrontendFormat() map[string]interface{} {
	return map[string]interface{}{
		"events":   vt.Events,
		"duration": vt.Duration,
	}
}
