package router

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTER TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// This file defines types specific to the semantic router.
// Core cognitive types are in internal/cognitive/types.go

// Note: Most routing types are defined in internal/cognitive/types.go
// to be accessible to other packages:
//
// - RouteDecision (template, novel, fallback)
// - ModelTier (local, mid, advanced, frontier)
// - RoutingResult
// - TemplateMatch
// - SimilarityLevel
// - Embedding operations (CosineSimilarity, Normalize)
//
// This file only contains router-internal types.
