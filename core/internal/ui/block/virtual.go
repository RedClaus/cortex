// Package block provides virtualized rendering for efficient display of long conversations.
package block

import (
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════════
// VIRTUAL LIST RENDERER
// ═══════════════════════════════════════════════════════════════════════════════

// VirtualListRenderer provides efficient rendering for long block lists.
// It only renders blocks that are visible within the viewport, dramatically
// improving performance for conversations with hundreds or thousands of blocks.
type VirtualListRenderer struct {
	// renderer is the underlying block renderer
	renderer *BlockRenderer

	// cache stores pre-rendered block content
	cache *RenderCache

	// heightEstimates stores estimated heights for each block
	heightEstimates map[string]int

	// defaultHeight is the fallback height for blocks not yet measured
	defaultHeight int

	// viewportHeight is the visible area height
	viewportHeight int

	// scrollOffset is the current scroll position (in lines)
	scrollOffset int

	// overscan is the number of extra blocks to render above/below viewport
	overscan int

	mu sync.RWMutex
}

// VirtualRenderResult contains the rendered output and metadata.
type VirtualRenderResult struct {
	// Content is the rendered string for the viewport
	Content string

	// TotalHeight is the estimated total height of all blocks
	TotalHeight int

	// VisibleRange indicates which blocks are rendered
	FirstVisibleIndex int
	LastVisibleIndex  int

	// BlockCount is the total number of blocks
	BlockCount int
}

// NewVirtualListRenderer creates a new virtualized renderer.
func NewVirtualListRenderer(renderer *BlockRenderer, viewportHeight int) *VirtualListRenderer {
	return &VirtualListRenderer{
		renderer:        renderer,
		cache:           DefaultRenderCache(),
		heightEstimates: make(map[string]int),
		defaultHeight:   5,  // Conservative default: header + 2 lines + footer + spacing
		viewportHeight:  viewportHeight,
		overscan:        3, // Render 3 extra blocks above/below for smooth scrolling
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VIRTUAL RENDERING
// ═══════════════════════════════════════════════════════════════════════════════

// RenderVirtual renders only the visible blocks for the given scroll position.
func (v *VirtualListRenderer) RenderVirtual(blocks []*Block, scrollOffset int) VirtualRenderResult {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.scrollOffset = scrollOffset

	if len(blocks) == 0 {
		return VirtualRenderResult{
			Content:     "",
			TotalHeight: 0,
			BlockCount:  0,
		}
	}

	// Calculate which blocks are visible
	firstVisible, lastVisible := v.calculateVisibleRange(blocks)

	// Render visible blocks
	var rendered strings.Builder
	renderedHeight := 0

	// Add top spacer for blocks above viewport
	topSpacerHeight := v.calculateHeightBefore(blocks, firstVisible)
	if topSpacerHeight > 0 {
		rendered.WriteString(strings.Repeat("\n", topSpacerHeight))
	}

	// Render visible blocks
	for i := firstVisible; i <= lastVisible && i < len(blocks); i++ {
		b := blocks[i]
		blockContent := v.renderBlockCached(b)

		// Update height estimate based on actual render
		actualHeight := strings.Count(blockContent, "\n") + 1
		v.heightEstimates[b.ID] = actualHeight

		rendered.WriteString(blockContent)
		rendered.WriteString("\n")
		renderedHeight += actualHeight
	}

	// Calculate total height for scroll bar calculations
	totalHeight := v.calculateTotalHeight(blocks)

	return VirtualRenderResult{
		Content:           rendered.String(),
		TotalHeight:       totalHeight,
		FirstVisibleIndex: firstVisible,
		LastVisibleIndex:  lastVisible,
		BlockCount:        len(blocks),
	}
}

// calculateVisibleRange determines which blocks should be rendered.
func (v *VirtualListRenderer) calculateVisibleRange(blocks []*Block) (first, last int) {
	if len(blocks) == 0 {
		return 0, 0
	}

	// Find first visible block
	cumulativeHeight := 0
	first = 0
	for i, b := range blocks {
		height := v.getBlockHeight(b)
		if cumulativeHeight+height > v.scrollOffset {
			first = i
			break
		}
		cumulativeHeight += height
	}

	// Apply overscan above (but not below 0)
	first -= v.overscan
	if first < 0 {
		first = 0
	}

	// Find last visible block
	cumulativeHeight = 0
	for i := 0; i < first; i++ {
		cumulativeHeight += v.getBlockHeight(blocks[i])
	}

	last = first
	viewportEnd := v.scrollOffset + v.viewportHeight
	for i := first; i < len(blocks); i++ {
		if cumulativeHeight > viewportEnd {
			last = i
			break
		}
		cumulativeHeight += v.getBlockHeight(blocks[i])
		last = i
	}

	// Apply overscan below
	last += v.overscan
	if last >= len(blocks) {
		last = len(blocks) - 1
	}

	return first, last
}

// getBlockHeight returns the estimated height for a block.
func (v *VirtualListRenderer) getBlockHeight(b *Block) int {
	if height, ok := v.heightEstimates[b.ID]; ok {
		return height
	}

	// Estimate based on block type and content
	estimated := v.estimateBlockHeight(b)
	v.heightEstimates[b.ID] = estimated
	return estimated
}

// estimateBlockHeight provides a reasonable height estimate for a block.
func (v *VirtualListRenderer) estimateBlockHeight(b *Block) int {
	// Base height: header + footer + padding
	base := 4

	// Add content-based estimate
	if b.Content != "" {
		lines := strings.Count(b.Content, "\n") + 1
		// Assume ~80 chars per line for wrapping estimate
		contentLength := len(b.Content)
		wrappedLines := (contentLength / 80) + 1
		if lines > wrappedLines {
			base += lines
		} else {
			base += wrappedLines
		}
	}

	// Tool blocks are taller due to input/output sections
	if b.Type == BlockTypeTool {
		base += 6 // Extra for input/output sections
	}

	// Code blocks may be taller
	if b.Type == BlockTypeCode {
		base += 2 // Syntax highlighting header
	}

	// Collapsed blocks are shorter
	if b.Collapsed {
		return 2 // Just header line
	}

	return base
}

// calculateHeightBefore calculates total height of blocks before index.
func (v *VirtualListRenderer) calculateHeightBefore(blocks []*Block, index int) int {
	height := 0
	for i := 0; i < index && i < len(blocks); i++ {
		height += v.getBlockHeight(blocks[i])
	}
	return height
}

// calculateTotalHeight calculates total height of all blocks.
func (v *VirtualListRenderer) calculateTotalHeight(blocks []*Block) int {
	height := 0
	for _, b := range blocks {
		height += v.getBlockHeight(b)
	}
	return height
}

// renderBlockCached renders a block using the cache when possible.
func (v *VirtualListRenderer) renderBlockCached(b *Block) string {
	// Don't cache streaming blocks
	if b.State == BlockStateStreaming {
		return v.renderer.RenderBlock(b)
	}

	// Generate cache key
	version := uint64(0)
	if !b.NeedsRender {
		version = 1
	}

	// Check cache
	if cached, ok := v.cache.Get(b.ID, v.renderer.width, version); ok {
		return cached
	}

	// Render and cache
	rendered := v.renderer.RenderBlock(b)
	v.cache.Set(b.ID, rendered, v.renderer.width, version)

	return rendered
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// SetViewportHeight updates the viewport height.
func (v *VirtualListRenderer) SetViewportHeight(height int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.viewportHeight = height
}

// SetWidth updates the render width and invalidates cache.
func (v *VirtualListRenderer) SetWidth(width int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.renderer.SetWidth(width)
	v.cache.InvalidateAll()
	// Clear height estimates since they depend on width
	v.heightEstimates = make(map[string]int)
}

// SetOverscan sets the number of extra blocks to render.
func (v *VirtualListRenderer) SetOverscan(overscan int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.overscan = overscan
}

// InvalidateBlock marks a block for re-rendering.
func (v *VirtualListRenderer) InvalidateBlock(blockID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cache.Invalidate(blockID)
	delete(v.heightEstimates, blockID)
}

// InvalidateAll clears all cached renders and height estimates.
func (v *VirtualListRenderer) InvalidateAll() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cache.InvalidateAll()
	v.heightEstimates = make(map[string]int)
}

// Stats returns rendering statistics.
func (v *VirtualListRenderer) Stats() VirtualRenderStats {
	v.mu.RLock()
	defer v.mu.RUnlock()

	cacheStats := v.cache.Stats()
	return VirtualRenderStats{
		CacheSize:        cacheStats.Size,
		CacheHits:        cacheStats.Hits,
		CacheMisses:      cacheStats.Misses,
		CacheHitRate:     cacheStats.HitRate,
		HeightEstimates:  len(v.heightEstimates),
		ViewportHeight:   v.viewportHeight,
		Overscan:         v.overscan,
	}
}

// VirtualRenderStats contains performance statistics.
type VirtualRenderStats struct {
	CacheSize       int
	CacheHits       uint64
	CacheMisses     uint64
	CacheHitRate    float64
	HeightEstimates int
	ViewportHeight  int
	Overscan        int
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCROLL POSITION HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// GetScrollPositionForBlock calculates the scroll offset to show a specific block.
func (v *VirtualListRenderer) GetScrollPositionForBlock(blocks []*Block, blockID string) int {
	v.mu.RLock()
	defer v.mu.RUnlock()

	height := 0
	for _, b := range blocks {
		if b.ID == blockID {
			// Center the block in viewport if possible
			center := height - (v.viewportHeight / 2)
			if center < 0 {
				center = 0
			}
			return center
		}
		height += v.getBlockHeight(b)
	}

	return 0 // Block not found
}

// GetBlockAtPosition returns the block ID at a given scroll position.
func (v *VirtualListRenderer) GetBlockAtPosition(blocks []*Block, position int) string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	height := 0
	for _, b := range blocks {
		blockHeight := v.getBlockHeight(b)
		if height+blockHeight > position {
			return b.ID
		}
		height += blockHeight
	}

	// Return last block if position is past end
	if len(blocks) > 0 {
		return blocks[len(blocks)-1].ID
	}
	return ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// FULL RENDER (NON-VIRTUAL FALLBACK)
// ═══════════════════════════════════════════════════════════════════════════════

// RenderAll renders all blocks without virtualization (for small lists).
func (v *VirtualListRenderer) RenderAll(blocks []*Block) string {
	if len(blocks) == 0 {
		return ""
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	var rendered strings.Builder
	for _, b := range blocks {
		blockContent := v.renderBlockCached(b)

		// Update height estimate
		actualHeight := strings.Count(blockContent, "\n") + 1
		v.heightEstimates[b.ID] = actualHeight

		rendered.WriteString(blockContent)
		rendered.WriteString("\n")
	}

	return rendered.String()
}

// ShouldUseVirtualization returns true if the block list is large enough
// to benefit from virtualization.
func ShouldUseVirtualization(blockCount int) bool {
	// Virtualization overhead isn't worth it for small lists
	return blockCount > 50
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCROLL INDICATOR
// ═══════════════════════════════════════════════════════════════════════════════

// RenderScrollIndicator renders a visual scroll position indicator.
func (v *VirtualListRenderer) RenderScrollIndicator(result VirtualRenderResult, style lipgloss.Style) string {
	if result.TotalHeight <= v.viewportHeight {
		return "" // No scrolling needed
	}

	// Calculate scroll percentage
	maxScroll := result.TotalHeight - v.viewportHeight
	if maxScroll <= 0 {
		return ""
	}

	scrollPercent := float64(v.scrollOffset) / float64(maxScroll)
	if scrollPercent > 1 {
		scrollPercent = 1
	}

	// Render indicator
	indicator := strings.Builder{}
	if v.scrollOffset > 0 {
		indicator.WriteString("↑ ")
	}
	indicator.WriteString(strings.Repeat("░", int(scrollPercent*10)))
	indicator.WriteString(strings.Repeat("█", 10-int(scrollPercent*10)))
	if v.scrollOffset < maxScroll {
		indicator.WriteString(" ↓")
	}

	return style.Render(indicator.String())
}
