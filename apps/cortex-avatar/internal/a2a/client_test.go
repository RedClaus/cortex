package a2a

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendMessageOptions_VoiceMode(t *testing.T) {
	opts := SendMessageOptions{
		Mode: "voice",
	}
	assert.Equal(t, "voice", opts.Mode)
}

func TestSendMessageOptions_PersonaOverride(t *testing.T) {
	opts := SendMessageOptions{
		Mode:    "voice",
		Persona: "custom-persona",
	}
	assert.Equal(t, "voice", opts.Mode)
	assert.Equal(t, "custom-persona", opts.Persona)
}

func TestSendMessageOptions_WithVision(t *testing.T) {
	opts := SendMessageOptions{
		Mode:        "voice",
		ImageBase64: "base64data",
		MimeType:    "image/png",
	}
	assert.Equal(t, "voice", opts.Mode)
	assert.Equal(t, "base64data", opts.ImageBase64)
	assert.Equal(t, "image/png", opts.MimeType)
}

func TestMessageSendParams_ModeFieldSerialization(t *testing.T) {
	msg := NewTextMessage("user", "Hello", map[string]any{
		"userId":    "test-user",
		"personaId": "hannah",
	})

	params := MessageSendParams{
		Message: msg,
		Mode:    "voice",
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	// Verify the mode field is present in JSON
	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "voice", parsed["mode"])
	assert.NotNil(t, parsed["message"])
}

func TestMessageSendParams_ModeOmittedWhenEmpty(t *testing.T) {
	msg := NewTextMessage("user", "Hello", nil)

	params := MessageSendParams{
		Message: msg,
		Mode:    "", // Empty mode
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	// Verify mode field is omitted when empty (omitempty)
	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	_, hasMode := parsed["mode"]
	assert.False(t, hasMode, "mode field should be omitted when empty")
}

func TestJSONRPCRequest_VoiceModeIncluded(t *testing.T) {
	msg := NewTextMessage("user", "What's the weather?", map[string]any{
		"userId":    "user-123",
		"personaId": "hannah",
	})

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: MessageSendParams{
			Message: msg,
			Mode:    "voice",
		},
		ID: 1,
	}

	data, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	// Parse and verify structure
	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed["jsonrpc"])
	assert.Equal(t, "message/send", parsed["method"])

	params, ok := parsed["params"].(map[string]any)
	require.True(t, ok, "params should be an object")
	assert.Equal(t, "voice", params["mode"])
}

func TestJSONRPCRequest_StreamingWithVoiceMode(t *testing.T) {
	msg := NewTextMessage("user", "Tell me a joke", map[string]any{
		"userId":    "user-456",
		"personaId": "spark",
	})

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/stream",
		Params: MessageSendParams{
			Message: msg,
			Mode:    "voice",
		},
		ID: 1,
	}

	data, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "message/stream", parsed["method"])

	params, ok := parsed["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "voice", params["mode"])

	message, ok := params["message"].(map[string]any)
	require.True(t, ok)

	metadata, ok := message["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "spark", metadata["personaId"])
}

func TestMessageSendParams_PersonaInMetadata(t *testing.T) {
	msg := NewTextMessage("user", "Hello", map[string]any{
		"userId":    "user-789",
		"personaId": "override-persona",
	})

	params := MessageSendParams{
		Message: msg,
		Mode:    "voice",
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	message, ok := parsed["message"].(map[string]any)
	require.True(t, ok)

	metadata, ok := message["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "override-persona", metadata["personaId"])
	assert.Equal(t, "user-789", metadata["userId"])
}

func TestMessageSendParams_WithConfig(t *testing.T) {
	msg := NewTextMessage("user", "Hello", nil)
	blocking := true
	historyLen := 5

	params := MessageSendParams{
		Message: msg,
		Mode:    "voice",
		Config: &MessageSendConfig{
			Blocking:      &blocking,
			HistoryLength: &historyLen,
		},
	}

	data, err := json.Marshal(params)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "voice", parsed["mode"])

	config, ok := parsed["configuration"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, config["blocking"])
	assert.Equal(t, float64(5), config["historyLength"])
}

func TestSendMessageOptions_EmptyFields(t *testing.T) {
	opts := SendMessageOptions{}

	assert.Empty(t, opts.Mode)
	assert.Empty(t, opts.Persona)
	assert.Empty(t, opts.ImageBase64)
	assert.Empty(t, opts.MimeType)
}

func TestNewTextMessage_WithMetadata(t *testing.T) {
	metadata := map[string]any{
		"userId":    "test-user",
		"personaId": "hannah",
	}
	msg := NewTextMessage("user", "Test message", metadata)

	assert.Equal(t, "user", msg.Role)
	require.Len(t, msg.Parts, 1)

	textPart, ok := msg.Parts[0].(TextPart)
	require.True(t, ok)
	assert.Equal(t, "text", textPart.Kind)
	assert.Equal(t, "Test message", textPart.Text)

	assert.Equal(t, "test-user", msg.Metadata["userId"])
	assert.Equal(t, "hannah", msg.Metadata["personaId"])
}

func TestNewVisionMessage_WithMetadata(t *testing.T) {
	metadata := map[string]any{
		"userId":    "vision-user",
		"personaId": "spark",
	}
	msg := NewVisionMessage("user", "Describe this", "base64imagedata", "image/jpeg", metadata)

	assert.Equal(t, "user", msg.Role)
	require.Len(t, msg.Parts, 2)

	textPart, ok := msg.Parts[0].(TextPart)
	require.True(t, ok)
	assert.Equal(t, "Describe this", textPart.Text)

	filePart, ok := msg.Parts[1].(FilePart)
	require.True(t, ok)
	assert.Equal(t, "file", filePart.Kind)
	assert.Equal(t, "image/jpeg", filePart.MimeType)
	assert.Equal(t, "base64imagedata", filePart.Bytes)

	assert.Equal(t, "spark", msg.Metadata["personaId"])
}

// --- Streaming Support Tests (US-004) ---

func TestSendMessageOptions_StreamField(t *testing.T) {
	opts := SendMessageOptions{
		Stream: true,
	}
	assert.True(t, opts.Stream)
}

func TestSendMessageOptions_StreamWithVoiceMode(t *testing.T) {
	opts := SendMessageOptions{
		Mode:   "voice",
		Stream: true,
	}
	assert.Equal(t, "voice", opts.Mode)
	assert.True(t, opts.Stream)
}

func TestSendMessageOptions_StreamDefaultsFalse(t *testing.T) {
	opts := SendMessageOptions{}
	assert.False(t, opts.Stream, "Stream should default to false")
}

func TestSendMessageOptions_AllFieldsCombined(t *testing.T) {
	opts := SendMessageOptions{
		Mode:        "voice",
		Persona:     "custom-persona",
		ImageBase64: "base64data",
		MimeType:    "image/png",
		Stream:      true,
	}
	assert.Equal(t, "voice", opts.Mode)
	assert.Equal(t, "custom-persona", opts.Persona)
	assert.Equal(t, "base64data", opts.ImageBase64)
	assert.Equal(t, "image/png", opts.MimeType)
	assert.True(t, opts.Stream)
}

func TestStreamingResponse_Structure(t *testing.T) {
	resp := StreamingResponse{
		Text:    "Hello world",
		Delta:   "world",
		IsFinal: false,
		State:   TaskStateWorking,
		Message: nil,
		Error:   nil,
	}
	assert.Equal(t, "Hello world", resp.Text)
	assert.Equal(t, "world", resp.Delta)
	assert.False(t, resp.IsFinal)
	assert.Equal(t, TaskStateWorking, resp.State)
}

func TestStreamingResponse_FinalState(t *testing.T) {
	msg := NewTextMessage("agent", "Final response", nil)
	resp := StreamingResponse{
		Text:    "Final response",
		Delta:   "",
		IsFinal: true,
		State:   TaskStateCompleted,
		Message: msg,
	}
	assert.True(t, resp.IsFinal)
	assert.Equal(t, TaskStateCompleted, resp.State)
	assert.NotNil(t, resp.Message)
}

func TestStreamingResponse_ErrorState(t *testing.T) {
	resp := StreamingResponse{
		IsFinal: true,
		Error:   assert.AnError,
	}
	assert.True(t, resp.IsFinal)
	assert.Error(t, resp.Error)
}

func TestTaskEvent_StreamingFields(t *testing.T) {
	msg := NewTextMessage("agent", "Streaming chunk", nil)
	event := TaskEvent{
		EventType: "status-update",
		TaskID:    "task-123",
		State:     TaskStateWorking,
		Message:   msg,
		Final:     false,
	}
	assert.Equal(t, "status-update", event.EventType)
	assert.Equal(t, TaskStateWorking, event.State)
	assert.False(t, event.Final)
	assert.NotNil(t, event.Message)
}

func TestTaskEvent_FinalEvent(t *testing.T) {
	msg := NewTextMessage("agent", "Complete response", nil)
	event := TaskEvent{
		EventType: "status-update",
		TaskID:    "task-456",
		State:     TaskStateCompleted,
		Message:   msg,
		Final:     true,
	}
	assert.True(t, event.Final)
	assert.Equal(t, TaskStateCompleted, event.State)
}

func TestTaskState_Constants(t *testing.T) {
	// Verify all task states are defined correctly
	assert.Equal(t, TaskState("submitted"), TaskStateSubmitted)
	assert.Equal(t, TaskState("working"), TaskStateWorking)
	assert.Equal(t, TaskState("completed"), TaskStateCompleted)
	assert.Equal(t, TaskState("failed"), TaskStateFailed)
	assert.Equal(t, TaskState("canceled"), TaskStateCanceled)
}

func TestJSONRPCRequest_StreamMethod(t *testing.T) {
	msg := NewTextMessage("user", "Stream this", nil)

	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/stream",
		Params: MessageSendParams{
			Message: msg,
			Mode:    "voice",
		},
		ID: 1,
	}

	data, err := json.Marshal(rpcReq)
	require.NoError(t, err)

	var parsed map[string]any
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Verify it uses the streaming method
	assert.Equal(t, "message/stream", parsed["method"])
}

func TestMessage_ExtractText(t *testing.T) {
	msg := &Message{
		Role: "agent",
		Parts: []Part{
			TextPart{Kind: "text", Text: "First part"},
			TextPart{Kind: "text", Text: "Second part"},
		},
	}
	extracted := msg.ExtractText()
	assert.Equal(t, "First part\nSecond part", extracted)
}

func TestMessage_ExtractText_SinglePart(t *testing.T) {
	msg := NewTextMessage("agent", "Single text", nil)
	extracted := msg.ExtractText()
	assert.Equal(t, "Single text", extracted)
}

func TestMessage_ExtractText_MixedParts(t *testing.T) {
	msg := &Message{
		Role: "agent",
		Parts: []Part{
			TextPart{Kind: "text", Text: "Text content"},
			DataPart{Kind: "data", Data: map[string]any{"key": "value"}},
			FilePart{Kind: "file", Bytes: "base64"},
		},
	}
	// Should only extract text parts
	extracted := msg.ExtractText()
	assert.Equal(t, "Text content", extracted)
}
