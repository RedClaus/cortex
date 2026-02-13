// Package brain provides the cognitive architecture for CortexBrain.
//
// The Blackboard implements a Copy-on-Write (CoW) pattern for efficient cloning
// in parallel execution scenarios. This optimization reduces clone latency from
// O(N) to O(1) by using parent pointers and overlays instead of full copies.
package brain

import (
	"encoding/json"
	"sync"
	"sync/atomic"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

// MaxParentDepth is the maximum allowed parent chain depth before auto-flattening.
// This prevents unbounded chain growth which could cause:
//   - Stack overflow on recursive walks
//   - GC unable to reclaim intermediate parents
//   - O(depth) Get() degrading toward O(N)
//
// Value of 8 allows:
//   - Typical parallel execution (depth 1-2)
//   - Recursive replanning (depth 4-5)
//   - Headroom while bounding worst case
const MaxParentDepth = 8

// -----------------------------------------------------------------------------
// Interfaces
// -----------------------------------------------------------------------------

// BlackboardReader defines read-only access to the blackboard.
// Used by lobes that only need to read context.
type BlackboardReader interface {
	Get(key string) (interface{}, bool)
	GetString(key string) string
	GetFloat(key string) float64
	Keys() []string
	Summary() map[string]interface{}
}

// BlackboardWriter defines write access to the blackboard.
// Used by lobes that need to modify context.
type BlackboardWriter interface {
	Set(key string, value interface{})
	Delete(key string)
	Merge(result *LobeResult)
	AddMemory(mem Memory)
	AddEntity(ent Entity)
	SetUserState(state *UserState)
}

// BlackboardInterface defines the full blackboard contract.
// Both the legacy Blackboard and new AttentionBlackboard implement this.
type BlackboardInterface interface {
	BlackboardReader
	BlackboardWriter
	Clone() *Blackboard
	Flatten() *Blackboard
}

// -----------------------------------------------------------------------------
// Core Types
// -----------------------------------------------------------------------------

// overlayEntry wraps a value with tombstone support for deletions.
// This allows distinguishing between:
//   - A key with nil value (tombstone=false, value=nil)
//   - A deleted key (tombstone=true)
//
// The tombstone shadows any value in the parent chain.
type overlayEntry struct {
	value     interface{}
	tombstone bool // true = key was explicitly deleted in this overlay
}

// Blackboard is the shared memory space across all lobes in a single request.
// It uses a Copy-on-Write (CoW) pattern for efficient cloning in parallel execution.
//
// # Thread Safety
//
// All public methods are thread-safe via RWMutex. The parent chain is immutable
// once frozen. Only the overlay map is modified after creation.
//
// # Memory Model
//
//   - Clone() creates a shallow snapshot with parent reference: O(1)
//   - Get() walks parent chain: O(depth) where depth typically 1-2
//   - Set() writes to overlay only: O(1)
//   - Flatten() materializes full state: O(N) where N is total keys
//
// # Brain Alignment
//
// Like working memory maintaining pointers to long-term memory, with modifications
// tracked separately until consolidation. The frozen flag represents memory
// consolidation - once stored, a memory becomes read-only.
type Blackboard struct {
	mu sync.RWMutex

	// parent points to the immutable parent blackboard.
	// Once this blackboard is cloned, it becomes frozen and immutable.
	// Parent chain depth is bounded by MaxParentDepth.
	parent *Blackboard

	// overlay contains only keys modified in this blackboard instance.
	// Uses tombstones (entry with tombstone=true) for deletions.
	// Keys not in overlay are looked up in parent chain.
	overlay map[string]overlayEntry

	// frozen indicates this blackboard has been cloned and is now immutable.
	// All subsequent writes will panic (programming error - should clone first).
	// Using atomic.Bool for lock-free reads in the common path.
	frozen atomic.Bool

	// depth tracks the parent chain depth for this blackboard.
	// Used to trigger flattening when depth exceeds MaxParentDepth.
	// Root blackboard has depth 0.
	depth int

	// Structured extractions (eagerly copied on Clone for slice safety)
	// These are stored directly on the struct, not in the overlay.
	Memories  []Memory   `json:"memories"`
	Entities  []Entity   `json:"entities"`
	UserState *UserState `json:"user_state"`

	// Accumulated confidence (multiplicative across lobe results)
	OverallConfidence float64 `json:"overall_confidence"`

	// Conversation context for multi-turn tracking
	ConversationID string `json:"conversation_id"`
	TurnNumber     int    `json:"turn_number"`
}

// Memory represents a retrieved memory item from the memory system.
type Memory struct {
	ID        string  `json:"id"`
	Content   string  `json:"content"`
	Source    string  `json:"source"`
	Relevance float64 `json:"relevance"`
}

// Entity represents an extracted entity from text parsing.
type Entity struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// UserState represents current user modeling state.
type UserState struct {
	EstimatedMood   string  `json:"estimated_mood"`
	ExpertiseLevel  string  `json:"expertise_level"`
	PreferredTone   string  `json:"preferred_tone"`
	EngagementLevel float64 `json:"engagement_level"`
}

// -----------------------------------------------------------------------------
// Constructor
// -----------------------------------------------------------------------------

// NewBlackboard creates a new root Blackboard instance with initialized maps
// and default confidence of 1.0 (multiplicative identity).
//
// The returned blackboard has:
//   - depth = 0 (root)
//   - parent = nil
//   - empty overlay map
//   - empty slices for Memories and Entities
func NewBlackboard() *Blackboard {
	return &Blackboard{
		parent:            nil,
		overlay:           make(map[string]overlayEntry),
		depth:             0,
		Memories:          make([]Memory, 0),
		Entities:          make([]Entity, 0),
		OverallConfidence: 1.0,
	}
}

// -----------------------------------------------------------------------------
// Clone (O(1) Fast Path)
// -----------------------------------------------------------------------------

// Clone creates a copy-on-write snapshot of the blackboard.
// This is O(1) in the common case, only copying structured slices for safety.
//
// # Behavior
//
//   - If depth < MaxParentDepth: O(1) parent pointer + empty overlay
//   - If depth >= MaxParentDepth: O(N) flatten to prevent unbounded chains
//
// # Thread Safety
//
// Safe for concurrent calls. Marks the source blackboard as frozen.
// Once frozen, the blackboard becomes immutable - any write attempts will panic.
//
// # Usage Pattern
//
//	bb := NewBlackboard()
//	bb.Set("key", "value")
//	clone := bb.Clone()        // bb is now frozen
//	clone.Set("key", "new")    // OK - clone is not frozen
//	bb.Set("other", "x")       // PANIC - bb is frozen
func (b *Blackboard) Clone() *Blackboard {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Mark this blackboard as frozen (immutable from now on).
	// This is safe to do under RLock because frozen is atomic and
	// Set/Delete/etc acquire the write lock and check frozen.
	b.frozen.Store(true)

	// Check if we need to flatten due to excessive depth.
	// This amortizes the O(N) cost over many O(1) clones.
	if b.depth >= MaxParentDepth {
		return b.flattenLocked()
	}

	// O(1) clone: create new blackboard with parent pointer and empty overlay.
	// No data is copied - reads walk up to parent.
	clone := &Blackboard{
		parent:            b,
		overlay:           make(map[string]overlayEntry),
		depth:             b.depth + 1,
		OverallConfidence: b.OverallConfidence,
		ConversationID:    b.ConversationID,
		TurnNumber:        b.TurnNumber,
	}

	// Copy structured slices (required for slice safety).
	// Slices in Go share underlying arrays, so we must copy to prevent
	// mutations in the clone from affecting the frozen parent.
	// These are typically small (< 20 items) so copying is acceptable.
	if len(b.Memories) > 0 {
		clone.Memories = make([]Memory, len(b.Memories))
		copy(clone.Memories, b.Memories)
	} else {
		clone.Memories = make([]Memory, 0)
	}

	if len(b.Entities) > 0 {
		clone.Entities = make([]Entity, len(b.Entities))
		copy(clone.Entities, b.Entities)
	} else {
		clone.Entities = make([]Entity, 0)
	}

	// UserState is a small struct, deep copy it.
	if b.UserState != nil {
		us := *b.UserState
		clone.UserState = &us
	}

	return clone
}

// -----------------------------------------------------------------------------
// Get (Walk Parent Chain)
// -----------------------------------------------------------------------------

// Get retrieves a value from the blackboard, walking the parent chain if needed.
// Returns (value, true) if found, (nil, false) if not found or deleted.
//
// # Complexity
//
// O(depth) where depth is the parent chain length. In practice, depth is
// typically 1-2 for parallel execution branches.
//
// # Tombstone Handling
//
// If a key has been deleted in this layer (tombstone), Get returns (nil, false)
// even if the key exists in a parent. This shadows the parent value.
func (b *Blackboard) Get(key string) (interface{}, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.getLocked(key)
}

// getLocked performs the actual get without locking.
// Caller must hold at least a read lock.
func (b *Blackboard) getLocked(key string) (interface{}, bool) {
	// Check overlay first - this layer's modifications take precedence.
	if entry, ok := b.overlay[key]; ok {
		if entry.tombstone {
			// Key was explicitly deleted in this layer.
			// This shadows any value in parent chain.
			return nil, false
		}
		return entry.value, true
	}

	// Walk parent chain if we have a parent.
	// Parent is frozen (immutable), so it's safe to read without additional locks
	// from this goroutine's perspective. The parent's Get() will acquire its own lock.
	if b.parent != nil {
		return b.parent.Get(key)
	}

	// Not found in this layer or any parent.
	return nil, false
}

// GetString retrieves a string value from the blackboard.
// Returns empty string if key not found or value is not a string.
//
// This is a convenience method that handles type assertion internally.
func (b *Blackboard) GetString(key string) string {
	val, ok := b.Get(key)
	if !ok {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return ""
	}
	return str
}

// GetFloat retrieves a float64 value from the blackboard.
// Returns 0.0 if key not found or value is not a float64.
//
// This is a convenience method that handles type assertion internally.
func (b *Blackboard) GetFloat(key string) float64 {
	val, ok := b.Get(key)
	if !ok {
		return 0.0
	}
	f, ok := val.(float64)
	if !ok {
		return 0.0
	}
	return f
}

// -----------------------------------------------------------------------------
// Set (Write to Overlay Only)
// -----------------------------------------------------------------------------

// Set stores a value in the blackboard's overlay.
//
// # Frozen Check
//
// If the blackboard is frozen (has been cloned), this method panics.
// This is considered a programming error - the caller should have cloned
// the blackboard before attempting to modify it.
//
// # Complexity
//
// O(1) - writes only to the local overlay map.
//
// # Nil Values
//
// Setting a nil value is valid and different from deleting:
//   - Set("key", nil) stores nil as the value
//   - Delete("key") marks the key as deleted (tombstone)
func (b *Blackboard) Set(key string, value interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Fail fast if frozen (programming error - should clone first).
	// The panic provides a clear stack trace for debugging.
	if b.frozen.Load() {
		panic("blackboard: cannot Set on frozen blackboard; call Clone() first")
	}

	b.overlay[key] = overlayEntry{value: value, tombstone: false}
}

// -----------------------------------------------------------------------------
// Delete (Tombstone Marker)
// -----------------------------------------------------------------------------

// Delete removes a key from the blackboard by marking it as a tombstone.
// This shadows any value in the parent chain.
//
// # Frozen Check
//
// If the blackboard is frozen (has been cloned), this method panics.
//
// # Complexity
//
// O(1) - writes only to the local overlay map.
//
// # Behavior
//
// After Delete("key"):
//   - Get("key") returns (nil, false)
//   - Keys() excludes "key"
//   - Parent chain values are shadowed but not modified
func (b *Blackboard) Delete(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.frozen.Load() {
		panic("blackboard: cannot Delete on frozen blackboard; call Clone() first")
	}

	b.overlay[key] = overlayEntry{value: nil, tombstone: true}
}

// -----------------------------------------------------------------------------
// Keys (Merge with Deduplication)
// -----------------------------------------------------------------------------

// Keys returns all unique keys in the blackboard, including parent chain.
// Deleted keys (tombstones) are excluded from the result.
//
// # Complexity
//
// O(N) where N is total keys across all layers.
//
// # Order
//
// Keys are returned in no particular order. If consistent ordering is needed,
// the caller should sort the result.
func (b *Blackboard) Keys() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Track seen keys and deleted keys separately.
	// A key might be deleted in a child but exist in parent.
	seen := make(map[string]struct{})
	deleted := make(map[string]struct{})

	// Collect from this layer and walk up parent chain.
	b.collectKeysLocked(seen, deleted)

	// Convert to slice.
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	return keys
}

// collectKeysLocked collects keys recursively from this layer and parents.
// Must hold read lock on b.
func (b *Blackboard) collectKeysLocked(seen, deleted map[string]struct{}) {
	// Process overlay entries in this layer.
	for key, entry := range b.overlay {
		// Only process if we haven't seen this key in a child layer.
		// Child layer values (or deletions) take precedence.
		if _, alreadySeen := seen[key]; !alreadySeen {
			if _, alreadyDeleted := deleted[key]; !alreadyDeleted {
				if entry.tombstone {
					// Mark as deleted so parent values are shadowed.
					deleted[key] = struct{}{}
				} else {
					// Add to seen keys.
					seen[key] = struct{}{}
				}
			}
		}
	}

	// Walk parent chain.
	if b.parent != nil {
		b.parent.collectKeysFromParent(seen, deleted)
	}
}

// collectKeysFromParent is called on frozen parents.
// Each parent acquires its own lock.
func (b *Blackboard) collectKeysFromParent(seen, deleted map[string]struct{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for key, entry := range b.overlay {
		// Only process if not already seen or deleted by a child.
		if _, alreadySeen := seen[key]; !alreadySeen {
			if _, wasDeleted := deleted[key]; !wasDeleted {
				if entry.tombstone {
					deleted[key] = struct{}{}
				} else {
					seen[key] = struct{}{}
				}
			}
		}
	}

	// Continue up the chain.
	if b.parent != nil {
		b.parent.collectKeysFromParent(seen, deleted)
	}
}

// keysLocked returns keys without acquiring lock (caller must hold lock).
func (b *Blackboard) keysLocked() []string {
	seen := make(map[string]struct{})
	deleted := make(map[string]struct{})
	b.collectKeysLocked(seen, deleted)
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	return keys
}

// -----------------------------------------------------------------------------
// Flatten (Materialize Full State)
// -----------------------------------------------------------------------------

// Flatten creates a new root blackboard with all data materialized.
// This collapses the parent chain into a single-layer blackboard.
//
// # Use Cases
//
//   - Depth exceeds MaxParentDepth (called automatically by Clone)
//   - Serializing to disk/network (JSON encoding)
//   - Reclaiming parent chain memory
//   - Creating an independent snapshot
//
// # Complexity
//
// O(N) where N is total keys across all layers.
//
// # Memory
//
// After flattening, the old parent chain becomes eligible for GC
// (assuming no other references).
func (b *Blackboard) Flatten() *Blackboard {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.flattenLocked()
}

// flattenLocked performs flatten without locking.
// Caller must hold at least a read lock.
func (b *Blackboard) flattenLocked() *Blackboard {
	flat := NewBlackboard()
	flat.OverallConfidence = b.OverallConfidence
	flat.ConversationID = b.ConversationID
	flat.TurnNumber = b.TurnNumber

	// Copy slices (already making a new root, so need fresh copies).
	if len(b.Memories) > 0 {
		flat.Memories = make([]Memory, len(b.Memories))
		copy(flat.Memories, b.Memories)
	}
	if len(b.Entities) > 0 {
		flat.Entities = make([]Entity, len(b.Entities))
		copy(flat.Entities, b.Entities)
	}
	if b.UserState != nil {
		us := *b.UserState
		flat.UserState = &us
	}

	// Materialize all key-value pairs from the entire chain.
	allKeys := b.keysLocked()
	for _, key := range allKeys {
		if val, ok := b.getLocked(key); ok {
			flat.overlay[key] = overlayEntry{value: val, tombstone: false}
		}
	}

	return flat
}

// -----------------------------------------------------------------------------
// Merge (Lobe Result Integration)
// -----------------------------------------------------------------------------

// Merge integrates a LobeResult into the blackboard.
// Stores the lobe's content keyed by its LobeID and updates confidence.
//
// # Frozen Check
//
// If the blackboard is frozen (has been cloned), this method panics.
//
// # Confidence Update
//
// If result.Confidence >= 0, it is multiplied into OverallConfidence.
// This provides a multiplicative confidence model where each lobe's
// confidence reduces the overall confidence.
func (b *Blackboard) Merge(result *LobeResult) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.frozen.Load() {
		panic("blackboard: cannot Merge on frozen blackboard; call Clone() first")
	}

	// Store lobe result content keyed by lobe ID.
	b.overlay[string(result.LobeID)] = overlayEntry{
		value:     result.Content,
		tombstone: false,
	}

	// Update multiplicative confidence.
	if result.Confidence >= 0 {
		b.OverallConfidence *= result.Confidence
	}
}

// -----------------------------------------------------------------------------
// Memory/Entity/UserState Operations
// -----------------------------------------------------------------------------

// AddMemory appends a memory item to the blackboard.
//
// # Frozen Check
//
// If the blackboard is frozen (has been cloned), this method panics.
func (b *Blackboard) AddMemory(mem Memory) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.frozen.Load() {
		panic("blackboard: cannot AddMemory on frozen blackboard; call Clone() first")
	}
	b.Memories = append(b.Memories, mem)
}

// AddEntity appends an entity to the blackboard.
//
// # Frozen Check
//
// If the blackboard is frozen (has been cloned), this method panics.
func (b *Blackboard) AddEntity(ent Entity) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.frozen.Load() {
		panic("blackboard: cannot AddEntity on frozen blackboard; call Clone() first")
	}
	b.Entities = append(b.Entities, ent)
}

// SetUserState updates the user state in the blackboard.
//
// # Frozen Check
//
// If the blackboard is frozen (has been cloned), this method panics.
func (b *Blackboard) SetUserState(state *UserState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.frozen.Load() {
		panic("blackboard: cannot SetUserState on frozen blackboard; call Clone() first")
	}
	b.UserState = state
}

// -----------------------------------------------------------------------------
// Summary and Serialization
// -----------------------------------------------------------------------------

// Summary returns a snapshot of the blackboard's state for logging or debugging.
// This flattens the data portion for complete state representation.
//
// The returned map includes:
//   - conversation_id, turn_number, overall_confidence
//   - memory_count, entity_count
//   - depth, frozen (CoW metadata)
//   - user_state (if set)
//   - data (flattened key-value pairs)
func (b *Blackboard) Summary() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	summary := make(map[string]interface{})
	summary["conversation_id"] = b.ConversationID
	summary["turn_number"] = b.TurnNumber
	summary["overall_confidence"] = b.OverallConfidence
	summary["memory_count"] = len(b.Memories)
	summary["entity_count"] = len(b.Entities)
	summary["depth"] = b.depth
	summary["frozen"] = b.frozen.Load()

	if b.UserState != nil {
		summary["user_state"] = *b.UserState
	}

	// Flatten data for complete view.
	dataCopy := make(map[string]interface{})
	allKeys := b.keysLocked()
	for _, key := range allKeys {
		if val, ok := b.getLocked(key); ok {
			dataCopy[key] = val
		}
	}
	summary["data"] = dataCopy

	return summary
}

// MarshalJSON flattens the blackboard for JSON serialization.
// The parent chain is collapsed into a single data map.
//
// This ensures serialized blackboards can be deserialized without
// maintaining parent pointers (which are not serializable).
func (b *Blackboard) MarshalJSON() ([]byte, error) {
	flat := b.Flatten()

	// Create serializable representation.
	data := struct {
		Data              map[string]interface{} `json:"data"`
		Memories          []Memory               `json:"memories"`
		Entities          []Entity               `json:"entities"`
		UserState         *UserState             `json:"user_state,omitempty"`
		OverallConfidence float64                `json:"overall_confidence"`
		ConversationID    string                 `json:"conversation_id"`
		TurnNumber        int                    `json:"turn_number"`
	}{
		Data:              make(map[string]interface{}),
		Memories:          flat.Memories,
		Entities:          flat.Entities,
		UserState:         flat.UserState,
		OverallConfidence: flat.OverallConfidence,
		ConversationID:    flat.ConversationID,
		TurnNumber:        flat.TurnNumber,
	}

	// Extract data from overlay (flat has no parent).
	for key, entry := range flat.overlay {
		if !entry.tombstone {
			data.Data[key] = entry.value
		}
	}

	return json.Marshal(data)
}

// -----------------------------------------------------------------------------
// Diagnostic Methods
// -----------------------------------------------------------------------------

// Depth returns the current parent chain depth.
// Root blackboards have depth 0. Each Clone() increments depth by 1
// (unless flattening occurs at MaxParentDepth).
func (b *Blackboard) Depth() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.depth
}

// IsFrozen returns whether this blackboard has been cloned and is now immutable.
// A frozen blackboard will panic on any write operation.
func (b *Blackboard) IsFrozen() bool {
	return b.frozen.Load()
}

// Parent returns the parent blackboard, or nil if this is a root.
// Used primarily for testing and debugging.
func (b *Blackboard) Parent() *Blackboard {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.parent
}
