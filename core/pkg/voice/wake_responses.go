// Package voice provides voice-related types and utilities for Cortex.
// wake_responses.go provides categorized response pools for wake word and conversation flow (CR-012-C).
package voice

import (
	"math/rand"
	"time"
)

// ResponseCategory categorizes different response purposes.
type ResponseCategory string

const (
	CategoryWakeCold    ResponseCategory = "wake/cold"
	CategoryWakeWarm    ResponseCategory = "wake/warm"
	CategoryWakeActive  ResponseCategory = "wake/active"
	CategoryConfused    ResponseCategory = "confused"
	CategoryBackchannel ResponseCategory = "backchannel"
	CategoryFarewell    ResponseCategory = "farewell"
	CategoryAcknowledge ResponseCategory = "acknowledge"
)

// WakeResponse represents a single response with optional constraints.
type WakeResponse struct {
	// Text is the response text to speak.
	Text string
	// AudioFile is the pre-generated audio path (optional, relative to cache dir).
	AudioFile string
	// TimeRange constrains when this response is appropriate (optional).
	TimeRange *TimeRange
	// VolumeScale is the volume modifier (1.0 = normal, 0.4 = quiet for late night).
	VolumeScale float64
}

// TimeRange defines when a response is appropriate (hour of day).
type TimeRange struct {
	StartHour int // 0-23
	EndHour   int // 0-23
}

// WakeResponsePool holds categorized responses for different conversation states.
type WakeResponsePool struct {
	// Introduction responses (very first interaction - includes name)
	Introduction []WakeResponse
	// Cold state responses (formal greetings - returning after timeout)
	Cold []WakeResponse
	// Warm state responses (casual acknowledgments)
	Warm []WakeResponse
	// Active state responses (minimal, engaged)
	Active []WakeResponse
	// Confused responses for low confidence speech
	Confused []WakeResponse
	// Backchannel responses for active listening
	Backchannel []WakeResponse
	// Farewell responses for ending conversation
	Farewell []WakeResponse
	// Acknowledge responses for confirming actions
	Acknowledge []WakeResponse
}

// DefaultWakeResponsePool returns the production response pool.
func DefaultWakeResponsePool() *WakeResponsePool {
	return &WakeResponsePool{
		// Introduction responses - very first interaction (includes persona name)
		// Note: {name} is a placeholder replaced at runtime with Henry/Hannah
		Introduction: []WakeResponse{
			{Text: "Hi, I'm {name}. What can I assist with today?", AudioFile: "intro/hi_im_name.wav", VolumeScale: 1.0},
			{Text: "Hello! I'm {name}. How can I help you?", AudioFile: "intro/hello_im_name.wav", VolumeScale: 1.0},
			{Text: "Hey there, I'm {name}. What do you need?", AudioFile: "intro/hey_there_im_name.wav", VolumeScale: 1.0},
		},
		// Cold state - returning after timeout (no self-introduction needed)
		Cold: []WakeResponse{
			{Text: "Hi, how can I help?", AudioFile: "wake/cold/hi_how_can_i_help.wav", VolumeScale: 1.0},
			{Text: "Hey there. What do you need?", AudioFile: "wake/cold/hey_there.wav", VolumeScale: 1.0},
			{Text: "Good morning. How can I assist?", AudioFile: "wake/cold/good_morning.wav", VolumeScale: 1.0,
				TimeRange: &TimeRange{StartHour: 5, EndHour: 12}},
			{Text: "Good afternoon. What's up?", AudioFile: "wake/cold/good_afternoon.wav", VolumeScale: 1.0,
				TimeRange: &TimeRange{StartHour: 12, EndHour: 17}},
			{Text: "Evening. What can I do for you?", AudioFile: "wake/cold/evening.wav", VolumeScale: 1.0,
				TimeRange: &TimeRange{StartHour: 17, EndHour: 22}},
			// Late night quiet mode
			{Text: "Hey.", AudioFile: "wake/cold/hey_quiet.wav", VolumeScale: 0.4,
				TimeRange: &TimeRange{StartHour: 22, EndHour: 5}},
		},
		Warm: []WakeResponse{
			{Text: "Yeah?", AudioFile: "wake/warm/yeah.wav", VolumeScale: 1.0},
			{Text: "I'm listening.", AudioFile: "wake/warm/im_listening.wav", VolumeScale: 1.0},
			{Text: "Go ahead.", AudioFile: "wake/warm/go_ahead.wav", VolumeScale: 1.0},
			{Text: "What's up?", AudioFile: "wake/warm/whats_up.wav", VolumeScale: 1.0},
			{Text: "Hey.", AudioFile: "wake/warm/hey.wav", VolumeScale: 1.0},
			{Text: "Uh-huh?", AudioFile: "wake/warm/uh_huh.wav", VolumeScale: 1.0},
		},
		Active: []WakeResponse{
			{Text: "Mhm?", AudioFile: "wake/active/mhm.wav", VolumeScale: 1.0},
			{Text: "Yes?", AudioFile: "wake/active/yes.wav", VolumeScale: 1.0},
			{Text: "Still here.", AudioFile: "wake/active/still_here.wav", VolumeScale: 1.0},
		},
		Confused: []WakeResponse{
			{Text: "Say again?", AudioFile: "confused/say_again.wav", VolumeScale: 1.0},
			{Text: "Hmm?", AudioFile: "confused/hmm.wav", VolumeScale: 1.0},
			{Text: "Sorry, what was that?", AudioFile: "confused/sorry_what.wav", VolumeScale: 1.0},
			{Text: "One more time?", AudioFile: "confused/one_more_time.wav", VolumeScale: 1.0},
		},
		Backchannel: []WakeResponse{
			{Text: "Mhm", AudioFile: "backchannel/mhm.wav", VolumeScale: 0.8},
			{Text: "Uh-huh", AudioFile: "backchannel/uh_huh.wav", VolumeScale: 0.8},
			{Text: "Got it", AudioFile: "backchannel/got_it.wav", VolumeScale: 0.8},
			{Text: "Okay", AudioFile: "backchannel/okay.wav", VolumeScale: 0.8},
			{Text: "Right", AudioFile: "backchannel/right.wav", VolumeScale: 0.8},
		},
		Farewell: []WakeResponse{
			{Text: "Sure thing.", AudioFile: "farewell/sure_thing.wav", VolumeScale: 1.0},
			{Text: "Anytime.", AudioFile: "farewell/anytime.wav", VolumeScale: 1.0},
			{Text: "Let me know if you need anything.", AudioFile: "farewell/let_me_know.wav", VolumeScale: 1.0},
			{Text: "I'll be here.", AudioFile: "farewell/ill_be_here.wav", VolumeScale: 1.0},
		},
		Acknowledge: []WakeResponse{
			{Text: "On it.", AudioFile: "acknowledge/on_it.wav", VolumeScale: 1.0},
			{Text: "Got it.", AudioFile: "acknowledge/got_it.wav", VolumeScale: 1.0},
			{Text: "Working on it.", AudioFile: "acknowledge/working_on_it.wav", VolumeScale: 1.0},
			{Text: "Let me check.", AudioFile: "acknowledge/let_me_check.wav", VolumeScale: 1.0},
		},
	}
}

// GetWakeResponse selects an appropriate wake response for the given state.
func (p *WakeResponsePool) GetWakeResponse(state ConversationState) WakeResponse {
	var pool []WakeResponse

	switch state {
	case StateCold:
		pool = p.filterByTime(p.Cold)
	case StateWarm:
		pool = p.Warm
	case StateActive:
		pool = p.Active
	default:
		pool = p.Cold
	}

	if len(pool) == 0 {
		return WakeResponse{Text: "Yes?", VolumeScale: 1.0}
	}

	return pool[rand.Intn(len(pool))]
}

// GetIntroductionResponse returns a first-time introduction response.
// The {name} placeholder should be replaced with the persona name (Henry/Hannah).
func (p *WakeResponsePool) GetIntroductionResponse() WakeResponse {
	if len(p.Introduction) == 0 {
		return WakeResponse{Text: "Hi, I'm {name}. What can I help with?", VolumeScale: 1.0}
	}
	return p.Introduction[rand.Intn(len(p.Introduction))]
}

// GetConfusedResponse returns a clarification response for low-confidence speech.
func (p *WakeResponsePool) GetConfusedResponse() WakeResponse {
	if len(p.Confused) == 0 {
		return WakeResponse{Text: "Say again?", VolumeScale: 1.0}
	}
	return p.Confused[rand.Intn(len(p.Confused))]
}

// GetBackchannelResponse returns a random backchanneling response.
func (p *WakeResponsePool) GetBackchannelResponse() WakeResponse {
	if len(p.Backchannel) == 0 {
		return WakeResponse{Text: "Mhm", VolumeScale: 0.8}
	}
	return p.Backchannel[rand.Intn(len(p.Backchannel))]
}

// GetFarewellResponse returns a random farewell response.
func (p *WakeResponsePool) GetFarewellResponse() WakeResponse {
	if len(p.Farewell) == 0 {
		return WakeResponse{Text: "Sure thing.", VolumeScale: 1.0}
	}
	return p.Farewell[rand.Intn(len(p.Farewell))]
}

// GetAcknowledgeResponse returns a random acknowledgment response.
func (p *WakeResponsePool) GetAcknowledgeResponse() WakeResponse {
	if len(p.Acknowledge) == 0 {
		return WakeResponse{Text: "On it.", VolumeScale: 1.0}
	}
	return p.Acknowledge[rand.Intn(len(p.Acknowledge))]
}

// filterByTime filters responses by current time of day.
func (p *WakeResponsePool) filterByTime(responses []WakeResponse) []WakeResponse {
	hour := time.Now().Hour()
	var filtered []WakeResponse
	var noTimeConstraint []WakeResponse

	for _, r := range responses {
		if r.TimeRange == nil {
			noTimeConstraint = append(noTimeConstraint, r)
			continue
		}

		// Handle overnight ranges (e.g., 22-5)
		if r.TimeRange.StartHour > r.TimeRange.EndHour {
			if hour >= r.TimeRange.StartHour || hour < r.TimeRange.EndHour {
				filtered = append(filtered, r)
			}
		} else {
			if hour >= r.TimeRange.StartHour && hour < r.TimeRange.EndHour {
				filtered = append(filtered, r)
			}
		}
	}

	// Prefer time-appropriate responses, fall back to unconstrained
	if len(filtered) > 0 {
		return filtered
	}
	return noTimeConstraint
}

// GetAllResponses returns all responses for cache generation.
func (p *WakeResponsePool) GetAllResponses() []WakeResponse {
	var all []WakeResponse
	all = append(all, p.Introduction...)
	all = append(all, p.Cold...)
	all = append(all, p.Warm...)
	all = append(all, p.Active...)
	all = append(all, p.Confused...)
	all = append(all, p.Backchannel...)
	all = append(all, p.Farewell...)
	all = append(all, p.Acknowledge...)
	return all
}

// GetResponsesByCategory returns responses for a specific category.
func (p *WakeResponsePool) GetResponsesByCategory(category ResponseCategory) []WakeResponse {
	switch category {
	case CategoryWakeCold:
		return p.Cold
	case CategoryWakeWarm:
		return p.Warm
	case CategoryWakeActive:
		return p.Active
	case CategoryConfused:
		return p.Confused
	case CategoryBackchannel:
		return p.Backchannel
	case CategoryFarewell:
		return p.Farewell
	case CategoryAcknowledge:
		return p.Acknowledge
	default:
		return nil
	}
}

// ResponseCount returns the total number of responses in the pool.
func (p *WakeResponsePool) ResponseCount() int {
	return len(p.Cold) + len(p.Warm) + len(p.Active) +
		len(p.Confused) + len(p.Backchannel) + len(p.Farewell) + len(p.Acknowledge)
}
