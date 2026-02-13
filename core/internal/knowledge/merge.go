// Package knowledge provides conflict resolution strategies for knowledge sync.
package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/normanking/cortex/pkg/types"
)

// Note: MergeStrategy interface is defined in interfaces.go
// This file provides the implementation.

// TrustWeightedMerge implements the three-tier merge strategy:
//
// 1. Global scope → Remote always wins (admin authority)
// 2. Personal scope → Local always wins (your private data)
// 3. Team scope → Trust-weighted merge with timestamp tiebreaker
type TrustWeightedMerge struct {
	// LocalBias adjusts team-scope conflict resolution toward local preference.
	// Range: 0.0 (no bias) to 1.0 (strong local preference)
	// Default: 0.0 (pure trust scoring)
	//
	// Example: With LocalBias=0.2, local needs only 80% of remote's trust to win.
	LocalBias float64
}

// NewTrustWeightedMerge creates a merge strategy with default settings.
func NewTrustWeightedMerge() *TrustWeightedMerge {
	return &TrustWeightedMerge{
		LocalBias: 0.0,
	}
}

// NewTrustWeightedMergeWithBias creates a merge strategy with custom local bias.
func NewTrustWeightedMergeWithBias(localBias float64) (*TrustWeightedMerge, error) {
	if localBias < 0 || localBias > 1 {
		return nil, fmt.Errorf("local_bias must be between 0.0 and 1.0, got: %f", localBias)
	}
	return &TrustWeightedMerge{LocalBias: localBias}, nil
}

// Resolve determines which version of a knowledge item wins in a sync conflict.
//
// Resolution rules:
//  1. Global scope: Remote wins (read-only, admin-controlled)
//  2. Personal scope: Local wins (private data, no external authority)
//  3. Team scope: Compare trust scores with optional local bias
//     - If trust scores differ by >0.05: Higher trust wins
//     - If trust scores are close (<0.05 diff): Most recent wins (updated_at)
//     - Local bias adjustment: local_effective_trust = local_trust * (1 + local_bias)
func (m *TrustWeightedMerge) Resolve(ctx context.Context, local, remote *types.KnowledgeItem) (*types.MergeResult, error) {
	// Validation
	if local == nil {
		return nil, fmt.Errorf("local item cannot be nil")
	}
	if remote == nil {
		return nil, fmt.Errorf("remote item cannot be nil")
	}
	if local.ID != remote.ID {
		return nil, fmt.Errorf("cannot merge items with different IDs: %s vs %s", local.ID, remote.ID)
	}

	// Scope mismatch should be rare but handle gracefully
	if local.Scope != remote.Scope {
		return &types.MergeResult{
			Winner:     remote,
			Resolution: "remote_wins",
			Reason:     fmt.Sprintf("Scope mismatch: local=%s, remote=%s. Remote wins by default.", local.Scope, remote.Scope),
		}, nil
	}

	// Rule 1: Global scope → Remote always wins
	if local.Scope == types.ScopeGlobal {
		return &types.MergeResult{
			Winner:     remote,
			Resolution: "remote_wins",
			Reason:     "Global scope: admin authority. Remote always wins.",
		}, nil
	}

	// Rule 2: Personal scope → Local always wins
	if local.Scope == types.ScopePersonal {
		return &types.MergeResult{
			Winner:     local,
			Resolution: "local_wins",
			Reason:     "Personal scope: private data. Local always wins.",
		}, nil
	}

	// Rule 3: Team scope → Trust-weighted merge
	return m.resolveTeamScope(local, remote)
}

// resolveTeamScope implements trust-weighted conflict resolution for team items.
func (m *TrustWeightedMerge) resolveTeamScope(local, remote *types.KnowledgeItem) (*types.MergeResult, error) {
	localTrust := local.TrustScore
	remoteTrust := remote.TrustScore

	// Apply local bias if configured
	if m.LocalBias > 0 {
		localTrust = localTrust * (1.0 + m.LocalBias)
		// Cap at 1.0 to prevent unfair advantage
		if localTrust > 1.0 {
			localTrust = 1.0
		}
	}

	trustDiff := localTrust - remoteTrust
	const trustThreshold = 0.05 // 5% difference is considered significant

	// Case 1: Local has significantly higher trust
	if trustDiff > trustThreshold {
		reason := fmt.Sprintf("Team scope: Local trust (%.3f) > Remote trust (%.3f) by >%.0f%%",
			local.TrustScore, remote.TrustScore, trustThreshold*100)
		if m.LocalBias > 0 {
			reason += fmt.Sprintf(" [with %.0f%% local bias]", m.LocalBias*100)
		}

		return &types.MergeResult{
			Winner:     local,
			Resolution: "local_wins",
			Reason:     reason,
		}, nil
	}

	// Case 2: Remote has significantly higher trust
	if trustDiff < -trustThreshold {
		reason := fmt.Sprintf("Team scope: Remote trust (%.3f) > Local trust (%.3f) by >%.0f%%",
			remote.TrustScore, local.TrustScore, trustThreshold*100)

		return &types.MergeResult{
			Winner:     remote,
			Resolution: "remote_wins",
			Reason:     reason,
		}, nil
	}

	// Case 3: Trust scores are close → Use timestamp tiebreaker
	return m.resolveByTimestamp(local, remote)
}

// resolveByTimestamp breaks ties by choosing the most recently updated item.
func (m *TrustWeightedMerge) resolveByTimestamp(local, remote *types.KnowledgeItem) (*types.MergeResult, error) {
	localTime := local.UpdatedAt
	remoteTime := remote.UpdatedAt

	// Local is newer
	if localTime.After(remoteTime) {
		timeDiff := localTime.Sub(remoteTime)
		return &types.MergeResult{
			Winner:     local,
			Resolution: "local_wins",
			Reason: fmt.Sprintf("Team scope: Trust scores equal (%.3f vs %.3f). Local is newer by %s.",
				local.TrustScore, remote.TrustScore, formatDuration(timeDiff)),
		}, nil
	}

	// Remote is newer
	if remoteTime.After(localTime) {
		timeDiff := remoteTime.Sub(localTime)
		return &types.MergeResult{
			Winner:     remote,
			Resolution: "remote_wins",
			Reason: fmt.Sprintf("Team scope: Trust scores equal (%.3f vs %.3f). Remote is newer by %s.",
				local.TrustScore, remote.TrustScore, formatDuration(timeDiff)),
		}, nil
	}

	// Exact same timestamp (very rare) → Prefer remote for consistency
	return &types.MergeResult{
		Winner:     remote,
		Resolution: "remote_wins",
		Reason: fmt.Sprintf("Team scope: Trust scores equal (%.3f vs %.3f) and identical timestamps. Remote wins by default.",
			local.TrustScore, remote.TrustScore),
	}, nil
}

// BatchResolve resolves multiple conflicts in one pass.
// Useful for bulk sync operations.
func (m *TrustWeightedMerge) BatchResolve(ctx context.Context, conflicts []ConflictPair) ([]*types.MergeResult, error) {
	results := make([]*types.MergeResult, 0, len(conflicts))

	for i, conflict := range conflicts {
		result, err := m.Resolve(ctx, conflict.Local, conflict.Remote)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve conflict %d (ID: %s): %w", i, conflict.Local.ID, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// ConflictPair represents a local/remote conflict.
type ConflictPair struct {
	Local  *types.KnowledgeItem
	Remote *types.KnowledgeItem
}

// MergeSummary provides statistics for a batch merge operation.
type MergeSummary struct {
	TotalConflicts int
	LocalWins      int
	RemoteWins     int
	Errors         int
	Duration       time.Duration
}

// SummarizeBatch generates statistics from batch merge results.
func SummarizeBatch(results []*types.MergeResult) MergeSummary {
	summary := MergeSummary{
		TotalConflicts: len(results),
	}

	for _, result := range results {
		switch result.Resolution {
		case "local_wins":
			summary.LocalWins++
		case "remote_wins":
			summary.RemoteWins++
		}
	}

	return summary
}

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%d days", days)
}

// ValidateMergeResult checks if a merge result is valid.
func ValidateMergeResult(result *types.MergeResult) error {
	if result == nil {
		return fmt.Errorf("merge result cannot be nil")
	}
	if result.Winner == nil {
		return fmt.Errorf("merge result must have a winner")
	}
	if result.Resolution == "" {
		return fmt.Errorf("merge result must specify resolution type")
	}
	if result.Reason == "" {
		return fmt.Errorf("merge result must include reason")
	}

	validResolutions := map[string]bool{
		"local_wins":  true,
		"remote_wins": true,
		"merged":      true,
		"manual":      true,
	}
	if !validResolutions[result.Resolution] {
		return fmt.Errorf("invalid resolution type: %s", result.Resolution)
	}

	return nil
}

// IsContentDifferent checks if two items have different content.
// This is useful for detecting if a conflict is superficial (e.g., only metadata changed).
func IsContentDifferent(local, remote *types.KnowledgeItem) bool {
	if local.Title != remote.Title {
		return true
	}
	if local.Content != remote.Content {
		return true
	}

	// Compare tags (order-independent)
	if len(local.Tags) != len(remote.Tags) {
		return true
	}
	localTagSet := make(map[string]bool)
	for _, tag := range local.Tags {
		localTagSet[tag] = true
	}
	for _, tag := range remote.Tags {
		if !localTagSet[tag] {
			return true
		}
	}

	return false
}
