// Package brain provides adapters to wire pkg/brain (Executive) with Cortex infrastructure.
// This file implements the bridge between cognitive lobes and the capability registrar.
package brain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/normanking/cortex/internal/registrar"
	"github.com/normanking/cortex/pkg/brain"
)

// RegisterLobesWithRegistrar registers all lobes from the Executive with the Registrar.
// This enables the registrar to discover and invoke cognitive capabilities.
//
// Brain alignment: Acts as the synaptic bridge connecting the cognitive lobes
// to the capability discovery system, enabling cross-region coordination.
func RegisterLobesWithRegistrar(exec *brain.Executive, r *registrar.Registrar) error {
	if exec == nil {
		return fmt.Errorf("executive is nil")
	}
	if r == nil {
		return fmt.Errorf("registrar is nil")
	}

	registry := exec.Registry()
	if registry == nil {
		return fmt.Errorf("executive registry is nil")
	}

	lobes := registry.All()
	if len(lobes) == 0 {
		log.Warn("[Brain] RegisterLobesWithRegistrar: no lobes registered in executive")
		return nil
	}

	var registered int
	var errs []error

	for _, lobe := range lobes {
		cap := lobeToCapability(lobe)
		if err := r.Register(cap); err != nil {
			errs = append(errs, fmt.Errorf("lobe %s: %w", lobe.ID(), err))
			continue
		}
		registered++
		log.Debug("[Brain] RegisterLobesWithRegistrar: registered lobe %s as capability", lobe.ID())
	}

	log.Info("[Brain] RegisterLobesWithRegistrar: registered %d/%d lobes", registered, len(lobes))

	if len(errs) > 0 {
		return fmt.Errorf("failed to register %d lobes: %v", len(errs), errs)
	}

	return nil
}

// lobeToCapability converts a brain.Lobe to a registrar.Capability.
func lobeToCapability(lobe brain.Lobe) *registrar.Capability {
	id := lobe.ID()

	return &registrar.Capability{
		ID:          registrar.CapabilityID("lobe-" + string(id)),
		Name:        "Cognitive Lobe: " + string(id),
		Description: lobeDescription(id),
		Version:     "1.0.0",
		Domain:      lobeDomain(id),
		Type:        registrar.TypeSkill,
		Tags:        lobeTags(id),
		InputTypes:  []string{"text", "lobe_input"},
		OutputTypes: []string{"lobe_result"},
		IntentPatterns: lobeIntentPatterns(id),
		Handler:     createLobeHandler(lobe),
	}
}

// lobeDomain maps a LobeID to its appropriate registrar Domain.
func lobeDomain(id brain.LobeID) registrar.Domain {
	switch id {
	// Code-related lobes
	case brain.LobeCoding, brain.LobeLogic:
		return registrar.DomainCode

	// Data/Memory-related lobes
	case brain.LobeMemory:
		return registrar.DomainData

	// Security-related lobes
	case brain.LobeSafety:
		return registrar.DomainSecurity

	// AI/Cognitive lobes
	case brain.LobeReasoning, brain.LobePlanning, brain.LobeCreativity,
		brain.LobeMetacognition, brain.LobeAttention:
		return registrar.DomainAI

	// Default for all other lobes
	default:
		return registrar.DomainGeneral
	}
}

// lobeDescription returns a human-readable description for a lobe.
func lobeDescription(id brain.LobeID) string {
	descriptions := map[brain.LobeID]string{
		brain.LobeVision:       "Processes visual input and image understanding",
		brain.LobeAudition:     "Handles auditory input processing",
		brain.LobeTextParsing:  "Parses and structures text input",
		brain.LobeMemory:       "Manages information storage and retrieval",
		brain.LobePlanning:     "Handles task decomposition and planning",
		brain.LobeCreativity:   "Generates ideas and creative thinking",
		brain.LobeReasoning:    "Performs logical deduction and reasoning",
		brain.LobeEmotion:      "Processes emotional state and affect",
		brain.LobeTheoryOfMind: "Understands others' perspectives and mental states",
		brain.LobeRapport:      "Manages social bonding and interaction",
		brain.LobeCoding:       "Handles software development tasks",
		brain.LobeLogic:        "Performs formal logic and mathematical reasoning",
		brain.LobeTemporal:     "Handles time-based reasoning",
		brain.LobeSpatial:      "Manages spatial reasoning and relationships",
		brain.LobeCausal:       "Analyzes cause-and-effect relationships",
		brain.LobeAttention:    "Manages focus and prioritization",
		brain.LobeMetacognition: "Thinks about thinking and self-reflection",
		brain.LobeInhibition:   "Controls impulses and response inhibition",
		brain.LobeSelfKnowledge: "Maintains self-awareness and capability knowledge",
		brain.LobeSafety:       "Performs safety checks and harm prevention",
	}

	if desc, ok := descriptions[id]; ok {
		return desc
	}
	return fmt.Sprintf("Cognitive lobe for %s processing", id)
}

// lobeTags returns relevant tags for a lobe.
func lobeTags(id brain.LobeID) []string {
	tags := []string{"lobe", "cognitive", string(id)}

	// Add layer-specific tags
	switch id {
	case brain.LobeVision, brain.LobeAudition, brain.LobeTextParsing:
		tags = append(tags, "perception")
	case brain.LobeMemory, brain.LobePlanning, brain.LobeCreativity, brain.LobeReasoning:
		tags = append(tags, "cognitive-layer")
	case brain.LobeEmotion, brain.LobeTheoryOfMind, brain.LobeRapport:
		tags = append(tags, "social-emotional")
	case brain.LobeCoding, brain.LobeLogic, brain.LobeTemporal, brain.LobeSpatial, brain.LobeCausal:
		tags = append(tags, "specialized-reasoning")
	case brain.LobeAttention, brain.LobeMetacognition, brain.LobeInhibition, brain.LobeSelfKnowledge:
		tags = append(tags, "executive-function")
	case brain.LobeSafety:
		tags = append(tags, "safety", "regulation")
	}

	return tags
}

// lobeIntentPatterns returns intent matching patterns for a lobe.
func lobeIntentPatterns(id brain.LobeID) []string {
	patterns := map[brain.LobeID][]string{
		brain.LobeVision:       {"analyze image", "describe picture", "what do you see"},
		brain.LobeAudition:     {"transcribe audio", "listen to", "audio analysis"},
		brain.LobeTextParsing:  {"parse text", "extract information", "structure data"},
		brain.LobeMemory:       {"remember", "recall", "what did I say about", "search memory"},
		brain.LobePlanning:     {"plan", "break down task", "create steps", "how should I"},
		brain.LobeCreativity:   {"brainstorm", "generate ideas", "creative", "imagine"},
		brain.LobeReasoning:    {"reason about", "think through", "analyze", "explain why"},
		brain.LobeEmotion:      {"how are you feeling", "emotion", "sentiment"},
		brain.LobeTheoryOfMind: {"what do they think", "perspective", "understand their view"},
		brain.LobeRapport:      {"build rapport", "social", "relationship"},
		brain.LobeCoding:       {"write code", "debug", "implement", "program", "function"},
		brain.LobeLogic:        {"prove", "logical", "mathematical", "deduce"},
		brain.LobeTemporal:     {"timeline", "schedule", "when", "temporal"},
		brain.LobeSpatial:      {"where", "location", "spatial", "position"},
		brain.LobeCausal:       {"cause", "effect", "why did", "because"},
		brain.LobeAttention:    {"focus on", "prioritize", "important"},
		brain.LobeMetacognition: {"think about thinking", "reflect", "self-assess"},
		brain.LobeInhibition:   {"stop", "cancel", "inhibit", "prevent"},
		brain.LobeSelfKnowledge: {"what can you do", "capabilities", "who are you"},
		brain.LobeSafety:       {"is this safe", "check safety", "harm", "risk"},
	}

	if p, ok := patterns[id]; ok {
		return p
	}
	return []string{string(id)}
}

// createLobeHandler creates a registrar handler that invokes the lobe.
func createLobeHandler(lobe brain.Lobe) registrar.CapabilityHandler {
	return func(ctx context.Context, input registrar.CapabilityInput) (registrar.CapabilityOutput, error) {
		// Extract the raw input from the capability input
		var rawInput string
		if err := json.Unmarshal(input.Data, &rawInput); err != nil {
			// If not a string, try to get as a map and extract "input" field
			var inputMap map[string]interface{}
			if err := json.Unmarshal(input.Data, &inputMap); err == nil {
				if inp, ok := inputMap["input"].(string); ok {
					rawInput = inp
				}
			}
		}

		// Create lobe input
		lobeInput := brain.LobeInput{
			RawInput: rawInput,
		}

		// Create a minimal blackboard for the lobe
		bb := brain.NewBlackboard()

		// Process through the lobe
		result, err := lobe.Process(ctx, lobeInput, bb)
		if err != nil {
			return registrar.CapabilityOutput{
				Success: false,
				Error:   err.Error(),
			}, err
		}

		// Marshal the result
		resultData, err := json.Marshal(result)
		if err != nil {
			return registrar.CapabilityOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal result: %v", err),
			}, err
		}

		return registrar.CapabilityOutput{
			Type:    "lobe_result",
			Data:    resultData,
			Success: true,
		}, nil
	}
}
