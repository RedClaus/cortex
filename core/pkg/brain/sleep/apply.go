package sleep

import (
	"fmt"
	"time"
)

// ApplyProposal applies a personality proposal by ID.
func (sm *SleepManager) ApplyProposal(proposalID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.pendingWake == nil {
		return ErrNoProposal
	}

	var proposal *PersonalityProposal
	for i, p := range sm.pendingWake.PendingApproval {
		if p.ID == proposalID {
			proposal = &sm.pendingWake.PendingApproval[i]
			break
		}
	}

	if proposal == nil {
		return ErrNoProposal
	}

	return sm.applyProposalInternal(*proposal)
}

// applyProposal applies a proposal (called from prepareWakeReport for auto-apply).
func (sm *SleepManager) applyProposal(proposal PersonalityProposal) error {
	return sm.applyProposalInternal(proposal)
}

// applyProposalInternal is the internal implementation of applying a proposal.
func (sm *SleepManager) applyProposalInternal(proposal PersonalityProposal) error {
	personality, err := sm.personality.Load()
	if err != nil {
		return fmt.Errorf("failed to load personality: %w", err)
	}

	// Create a backup before applying
	if err := sm.personality.Backup(); err != nil {
		sm.log.Warn("[Sleep] Failed to backup personality before applying changes: %v", err)
	}

	// Apply each change
	for _, change := range proposal.Changes {
		if err := personality.ApplyChange(change); err != nil {
			return fmt.Errorf("failed to apply change to %s: %w", change.Path, err)
		}
	}

	// Update metadata
	personality.LastUpdated = time.Now()
	personality.Version = incrementVersion(personality.Version)

	// Save
	if err := sm.personality.Save(personality); err != nil {
		return fmt.Errorf("failed to save personality: %w", err)
	}

	sm.log.Info("[Sleep] Applied personality proposal: id=%s, type=%s, changes=%d",
		proposal.ID, proposal.Type, len(proposal.Changes))

	// Remove from pending if present
	if sm.pendingWake != nil {
		sm.removeFromPending(proposal.ID)
	}

	return nil
}

// RejectProposal rejects a pending proposal with optional feedback.
func (sm *SleepManager) RejectProposal(proposalID string, feedback string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.pendingWake == nil {
		return ErrNoProposal
	}

	found := false
	for i, p := range sm.pendingWake.PendingApproval {
		if p.ID == proposalID {
			sm.pendingWake.PendingApproval = append(
				sm.pendingWake.PendingApproval[:i],
				sm.pendingWake.PendingApproval[i+1:]...,
			)
			found = true
			break
		}
	}

	if !found {
		return ErrNoProposal
	}

	sm.log.Info("[Sleep] Proposal rejected by user: id=%s, feedback=%s", proposalID, feedback)

	return nil
}

// ApproveAllSafe approves all safe proposals.
func (sm *SleepManager) ApproveAllSafe() (int, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.pendingWake == nil {
		return 0, ErrNoProposal
	}

	approved := 0
	remaining := []PersonalityProposal{}

	for _, p := range sm.pendingWake.PendingApproval {
		if p.RiskLevel == RiskSafe {
			if err := sm.applyProposalInternal(p); err == nil {
				approved++
			} else {
				remaining = append(remaining, p)
			}
		} else {
			remaining = append(remaining, p)
		}
	}

	sm.pendingWake.PendingApproval = remaining

	return approved, nil
}

// RevertChange reverts a previously applied change by restoring from history.
func (sm *SleepManager) RevertChange(historyFile string) error {
	// Load the historical personality
	historical, err := sm.personality.LoadFromHistory(historyFile)
	if err != nil {
		return fmt.Errorf("failed to load historical personality: %w", err)
	}

	// Backup current before reverting
	if err := sm.personality.Backup(); err != nil {
		sm.log.Warn("[Sleep] Failed to backup before revert: %v", err)
	}

	// Save the historical version as current
	historical.LastUpdated = time.Now()
	if err := sm.personality.Save(historical); err != nil {
		return fmt.Errorf("failed to save reverted personality: %w", err)
	}

	sm.log.Info("[Sleep] Personality reverted from: %s", historyFile)

	return nil
}

// removeFromPending removes a proposal from the pending list.
func (sm *SleepManager) removeFromPending(proposalID string) {
	for i, p := range sm.pendingWake.PendingApproval {
		if p.ID == proposalID {
			sm.pendingWake.PendingApproval = append(
				sm.pendingWake.PendingApproval[:i],
				sm.pendingWake.PendingApproval[i+1:]...,
			)
			return
		}
	}
}

// incrementVersion increments a semantic version string.
func incrementVersion(version string) string {
	// Simple implementation: just append a revision number
	// In practice, you might want proper semver handling
	return version + ".1"
}
