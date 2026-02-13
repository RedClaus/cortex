// Package bridge provides Wails bindings between Go and frontend
package bridge

import (
	"context"

	"github.com/normanking/cortexavatar/internal/avatar"
	"github.com/normanking/cortexavatar/internal/bus"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AvatarBridge exposes avatar methods to the frontend
type AvatarBridge struct {
	ctx        context.Context
	controller *avatar.Controller
	eventBus   *bus.EventBus
}

// NewAvatarBridge creates the avatar bridge
func NewAvatarBridge(controller *avatar.Controller, eventBus *bus.EventBus) *AvatarBridge {
	return &AvatarBridge{
		controller: controller,
		eventBus:   eventBus,
	}
}

// Bind sets the Wails runtime context
func (b *AvatarBridge) Bind(ctx context.Context) {
	b.ctx = ctx

	// Set up state change handler to emit to frontend
	b.controller.SetStateHandler(func(state avatar.State) {
		runtime.EventsEmit(b.ctx, "avatar:stateChanged", state)
	})

	// Subscribe to bus events
	b.eventBus.Subscribe(bus.EventTypeAvatarStateChanged, func(e bus.Event) {
		runtime.EventsEmit(b.ctx, "avatar:stateChanged", e.Data)
	})

	b.eventBus.Subscribe(bus.EventTypeEmotionChanged, func(e bus.Event) {
		if emotion, ok := e.Data["emotion"].(string); ok {
			runtime.EventsEmit(b.ctx, "avatar:emotionChanged", emotion)
		}
	})
}

// GetState returns current avatar state
func (b *AvatarBridge) GetState() avatar.State {
	return b.controller.GetState()
}

// SetEmotion sets avatar emotion
func (b *AvatarBridge) SetEmotion(emotion string) {
	b.controller.SetEmotion(avatar.EmotionState(emotion))
}

// SetIdle returns avatar to idle state
func (b *AvatarBridge) SetIdle() {
	b.controller.SetIdle()
}
