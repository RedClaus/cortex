package block

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BLOCK ID GENERATION
// ═══════════════════════════════════════════════════════════════════════════════

// idCounter provides a thread-safe monotonic counter for ID uniqueness
var idCounter uint64

// GenerateID creates a unique block identifier.
// Format: blk_{timestamp_hex}_{counter}_{random_4bytes}
// Example: blk_18f5a2b3c_0001_a3f2
//
// The ID format ensures:
// - Chronological ordering via timestamp prefix
// - Uniqueness via counter + random suffix
// - Human readability via hex encoding
// - Collision resistance even under high concurrency
func GenerateID() string {
	// Get current timestamp in milliseconds
	timestamp := time.Now().UnixMilli()

	// Increment counter atomically
	counter := atomic.AddUint64(&idCounter, 1)

	// Generate 4 random bytes for additional uniqueness
	randomBytes := make([]byte, 4)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to counter-based suffix if crypto/rand fails
		randomBytes = []byte{
			byte(counter >> 24),
			byte(counter >> 16),
			byte(counter >> 8),
			byte(counter),
		}
	}

	// Format: blk_{timestamp}_{counter}_{random}
	return fmt.Sprintf("blk_%x_%04x_%s",
		timestamp,
		counter%0xFFFF,
		hex.EncodeToString(randomBytes),
	)
}

// GenerateChildID creates a child block ID that maintains parent relationship.
// Format: {parentID}_c{childIndex}
// Example: blk_18f5a2b3c_0001_a3f2_c0
func GenerateChildID(parentID string, childIndex int) string {
	return fmt.Sprintf("%s_c%d", parentID, childIndex)
}

// ParseBlockID extracts components from a block ID.
// Returns timestamp, counter, and random suffix.
// Returns zero values if the ID format is invalid.
func ParseBlockID(id string) (timestamp int64, counter uint16, random string, valid bool) {
	var tsHex string
	var counterHex uint16
	var randomStr string

	n, err := fmt.Sscanf(id, "blk_%s_%04x_%s", &tsHex, &counterHex, &randomStr)
	if err != nil || n != 3 {
		return 0, 0, "", false
	}

	// Parse timestamp from hex
	_, err = fmt.Sscanf(tsHex, "%x", &timestamp)
	if err != nil {
		return 0, 0, "", false
	}

	return timestamp, counterHex, randomStr, true
}

// IsValidBlockID checks if a string is a valid block ID format.
func IsValidBlockID(id string) bool {
	if len(id) < 20 { // Minimum valid length
		return false
	}
	_, _, _, valid := ParseBlockID(id)
	return valid
}

// CompareBlockIDs compares two block IDs chronologically.
// Returns:
//
//	-1 if a < b (a is older)
//	 0 if a == b
//	 1 if a > b (a is newer)
func CompareBlockIDs(a, b string) int {
	tsA, counterA, _, validA := ParseBlockID(a)
	tsB, counterB, _, validB := ParseBlockID(b)

	// Invalid IDs sort to the end
	if !validA && !validB {
		return 0
	}
	if !validA {
		return 1
	}
	if !validB {
		return -1
	}

	// Compare by timestamp first
	if tsA < tsB {
		return -1
	}
	if tsA > tsB {
		return 1
	}

	// Same timestamp, compare by counter
	if counterA < counterB {
		return -1
	}
	if counterA > counterB {
		return 1
	}

	return 0
}
