package bridge

import (
	"context"

	"github.com/normanking/cortexavatar/internal/a2a"
	"github.com/normanking/cortexavatar/internal/avatar"
	"github.com/normanking/cortexavatar/internal/bus"
	"github.com/normanking/cortexavatar/internal/config"
	"github.com/normanking/cortexavatar/internal/vision"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ConnectionBridge exposes connection methods to the frontend
type ConnectionBridge struct {
	ctx          context.Context
	a2aClient    *a2a.Client
	eventBus     *bus.EventBus
	cfg          *config.Config
	logger       zerolog.Logger
	avatarSync   *avatar.SyncClient
	visionStream *vision.StreamClient
}

// NewConnectionBridge creates the connection bridge
func NewConnectionBridge(
	a2aClient *a2a.Client,
	eventBus *bus.EventBus,
	cfg *config.Config,
	controller *avatar.Controller,
	logger zerolog.Logger,
) *ConnectionBridge {
	// Create avatar sync client for real-time state from CortexBrain
	avatarSync := avatar.NewSyncClient(cfg.A2A.ServerURL, controller, logger)

	// Create vision stream client for sending frames to CortexBrain
	visionStream := vision.NewStreamClient(cfg.A2A.ServerURL, logger)

	return &ConnectionBridge{
		a2aClient:    a2aClient,
		eventBus:     eventBus,
		cfg:          cfg,
		logger:       logger,
		avatarSync:   avatarSync,
		visionStream: visionStream,
	}
}

// Bind sets the Wails runtime context
func (b *ConnectionBridge) Bind(ctx context.Context) {
	b.ctx = ctx

	// Set up A2A status handler
	b.a2aClient.SetStatusHandler(func(connected bool, agentCard *a2a.AgentCard) {
		status := map[string]any{
			"isConnected": connected,
			"serverUrl":   b.cfg.A2A.ServerURL,
		}

		if agentCard != nil {
			status["agentName"] = agentCard.Name
			status["agentVersion"] = agentCard.Version
		}

		runtime.EventsEmit(b.ctx, "connection:status", status)

		if connected {
			b.eventBus.Publish(bus.Event{Type: bus.EventTypeConnected, Data: status})

			// Start avatar state sync when connected
			go func() {
				if err := b.avatarSync.Connect(context.Background()); err != nil {
					b.logger.Error().Err(err).Msg("Failed to connect avatar sync")
				}
			}()

			// Start vision stream when connected
			go func() {
				if err := b.visionStream.Connect(context.Background()); err != nil {
					b.logger.Error().Err(err).Msg("Failed to connect vision stream")
				}
			}()
		} else {
			b.eventBus.Publish(bus.Event{Type: bus.EventTypeDisconnected, Data: status})

			// Disconnect sync clients
			b.avatarSync.Disconnect()
			b.visionStream.Disconnect()
		}
	})

	// Set up error handler
	b.a2aClient.SetErrorHandler(func(err error) {
		runtime.EventsEmit(b.ctx, "connection:error", err.Error())
		b.eventBus.Publish(bus.Event{
			Type: bus.EventTypeError,
			Data: map[string]any{"error": err.Error()},
		})
	})
}

// Connect initiates connection to CortexBrain
func (b *ConnectionBridge) Connect() error {
	return b.a2aClient.Connect(context.Background())
}

// Disconnect closes the connection
func (b *ConnectionBridge) Disconnect() error {
	return b.a2aClient.Close()
}

// IsConnected returns connection status
func (b *ConnectionBridge) IsConnected() bool {
	return b.a2aClient.IsConnected()
}

// GetServerURL returns the configured server URL
func (b *ConnectionBridge) GetServerURL() string {
	return b.cfg.A2A.ServerURL
}

// GetAgentCard returns the connected agent's card
func (b *ConnectionBridge) GetAgentCard() *a2a.AgentCard {
	return b.a2aClient.GetAgentCard()
}

// IsAvatarSyncConnected returns avatar sync connection status
func (b *ConnectionBridge) IsAvatarSyncConnected() bool {
	return b.avatarSync.IsConnected()
}

// IsVisionStreamConnected returns vision stream connection status
func (b *ConnectionBridge) IsVisionStreamConnected() bool {
	return b.visionStream.IsConnected()
}

// GetVisionStreamClient returns the vision stream client for sending frames
func (b *ConnectionBridge) GetVisionStreamClient() *vision.StreamClient {
	return b.visionStream
}

// GetConnectionStatus returns full connection status
func (b *ConnectionBridge) GetConnectionStatus() map[string]any {
	return map[string]any{
		"a2a":          b.a2aClient.IsConnected(),
		"avatarSync":   b.avatarSync.IsConnected(),
		"visionStream": b.visionStream.IsConnected(),
		"serverUrl":    b.cfg.A2A.ServerURL,
	}
}
