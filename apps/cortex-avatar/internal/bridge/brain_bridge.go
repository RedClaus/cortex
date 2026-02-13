// Package bridge provides Wails Go-JS bindings.
package bridge

import (
	"context"

	"github.com/normanking/cortexavatar/internal/a2a"
	"github.com/normanking/cortexavatar/internal/config"
	"github.com/normanking/cortexavatar/internal/discovery"
	"github.com/normanking/cortexavatar/internal/logging"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// BrainBridge exposes brain discovery and selection to the frontend
type BrainBridge struct {
	ctx       context.Context
	discovery *discovery.Service
	a2aClient *a2a.Client
	cfg       *config.Config
	logger    *logging.Logger

	onReconnect func(string) // Callback to reconnect to new brain
}

// NewBrainBridge creates a new brain bridge
func NewBrainBridge(
	disc *discovery.Service,
	a2aClient *a2a.Client,
	cfg *config.Config,
	logger *logging.Logger,
) *BrainBridge {
	return &BrainBridge{
		discovery: disc,
		a2aClient: a2aClient,
		cfg:       cfg,
		logger:    logger,
	}
}

// SetOnReconnect sets the callback for when brain changes
func (b *BrainBridge) SetOnReconnect(fn func(string)) {
	b.onReconnect = fn
}

// Bind sets the Wails context
func (b *BrainBridge) Bind(ctx context.Context) {
	b.ctx = ctx

	// Set up discovery callbacks to emit events to frontend
	b.discovery.SetOnUpdate(func(brains []*discovery.Brain) {
		if b.ctx != nil {
			runtime.EventsEmit(b.ctx, "brain:list-updated", brains)
		}
	})

	b.discovery.SetOnSelect(func(brain *discovery.Brain) {
		if b.ctx != nil {
			runtime.EventsEmit(b.ctx, "brain:selected", brain)
		}
	})

	// Start discovery
	b.discovery.Start()

	b.logger.Info("brain-bridge", "Brain discovery started", nil)
}

// GetBrains returns all discovered brains
func (b *BrainBridge) GetBrains() []*discovery.Brain {
	return b.discovery.GetBrains()
}

// ScanBrains triggers a manual scan
func (b *BrainBridge) ScanBrains() []*discovery.Brain {
	b.logger.Debug("brain-bridge", "Manual brain scan triggered", nil)
	return b.discovery.Scan()
}

// GetSelectedBrain returns the currently selected brain
func (b *BrainBridge) GetSelectedBrain() *discovery.Brain {
	return b.discovery.GetSelected()
}

// SelectBrain selects a brain and reconnects
func (b *BrainBridge) SelectBrain(brainID string) error {
	b.logger.Info("brain-bridge", "Selecting brain", map[string]interface{}{
		"brainID": brainID,
	})

	// Select in discovery service
	if err := b.discovery.Select(brainID); err != nil {
		b.logger.Error("brain-bridge", "Failed to select brain", err, map[string]interface{}{
			"brainID": brainID,
		})
		return err
	}

	// Get the brain details
	brain := b.discovery.GetBrain(brainID)
	if brain == nil {
		return nil
	}

	// Update config
	b.cfg.A2A.ServerURL = brain.URL

	// Trigger reconnect
	if b.onReconnect != nil {
		b.onReconnect(brain.URL)
	}

	// Emit event
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "brain:connecting", brain)
	}

	// Reconnect A2A client
	go func() {
		b.a2aClient.Close()

		// Update client config
		b.a2aClient.UpdateServerURL(brain.URL)

		if err := b.a2aClient.Connect(context.Background()); err != nil {
			b.logger.Error("brain-bridge", "Failed to connect to brain", err, map[string]interface{}{
				"brainID": brainID,
				"url":     brain.URL,
			})
			if b.ctx != nil {
				runtime.EventsEmit(b.ctx, "brain:connection-failed", map[string]interface{}{
					"brain": brain,
					"error": err.Error(),
				})
			}
		} else {
			b.logger.Info("brain-bridge", "Connected to brain", map[string]interface{}{
				"brainID": brainID,
				"name":    brain.Name,
				"url":     brain.URL,
			})
			if b.ctx != nil {
				runtime.EventsEmit(b.ctx, "brain:connected", brain)
			}
		}
	}()

	return nil
}

// AddCustomBrainURL adds a custom brain URL to scan
func (b *BrainBridge) AddCustomBrainURL(url string) {
	b.logger.Info("brain-bridge", "Adding custom brain URL", map[string]interface{}{
		"url": url,
	})
	b.discovery.AddCustomURL(url)
	// Trigger rescan
	go b.discovery.Scan()
}

// RemoveCustomBrainURL removes a custom brain URL
func (b *BrainBridge) RemoveCustomBrainURL(url string) {
	b.logger.Info("brain-bridge", "Removing custom brain URL", map[string]interface{}{
		"url": url,
	})
	b.discovery.RemoveCustomURL(url)
}

// GetCurrentBrainInfo returns info about the currently connected brain
func (b *BrainBridge) GetCurrentBrainInfo() map[string]interface{} {
	info := make(map[string]interface{})

	info["serverURL"] = b.cfg.A2A.ServerURL
	info["connected"] = b.a2aClient.IsConnected()

	card := b.a2aClient.GetAgentCard()
	if card != nil {
		info["name"] = card.Name
		info["version"] = card.Version
		info["protocol"] = card.ProtocolVersion
		info["description"] = card.Description
	}

	return info
}

// Shutdown stops the discovery service
func (b *BrainBridge) Shutdown() {
	b.discovery.Stop()
}
