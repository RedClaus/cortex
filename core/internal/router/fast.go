package router

import (
	"regexp"
	"strings"
)

// FastClassifier implements regex-based intent classification.
// It's designed for speed (~1ms) and handles ~85-90% of requests confidently.
type FastClassifier struct {
	patterns map[TaskType][]*compiledPattern
}

// compiledPattern holds a pre-compiled regex with its weight.
type compiledPattern struct {
	regex  *regexp.Regexp
	weight float64 // Higher weight = stronger signal
}

// NewFastClassifier creates a new regex-based classifier with optimized patterns.
func NewFastClassifier() *FastClassifier {
	return &FastClassifier{
		patterns: buildPatterns(),
	}
}

// Classify analyzes input and returns the best task type with confidence.
// Returns TaskGeneral with low confidence if no strong match is found.
func (c *FastClassifier) Classify(input string) (TaskType, float64) {
	lower := strings.ToLower(input)

	// Calculate weighted scores per task type
	scores := make(map[TaskType]float64)
	matchCounts := make(map[TaskType]int)

	for taskType, patterns := range c.patterns {
		for _, p := range patterns {
			if p.regex.MatchString(lower) {
				scores[taskType] += p.weight
				matchCounts[taskType]++
			}
		}
	}

	// Find best match
	var bestType TaskType = TaskGeneral
	var bestScore float64
	var totalScore float64

	for taskType, score := range scores {
		totalScore += score
		if score > bestScore {
			bestScore = score
			bestType = taskType
		}
	}

	// Calculate confidence
	if totalScore == 0 {
		// No patterns matched - return general with low confidence
		return TaskGeneral, 0.4
	}

	// Base confidence is the proportion of the best score
	confidence := bestScore / totalScore

	// Boost confidence based on match quality
	if len(scores) == 1 {
		// Only one task type matched - high confidence
		confidence = min(confidence+0.25, 1.0)
	}

	if matchCounts[bestType] >= 2 {
		// Multiple patterns matched for the same type - boost confidence
		confidence = min(confidence+0.1, 1.0)
	}

	// Penalize if multiple task types have similar scores
	if len(scores) > 1 {
		secondBest := findSecondBest(scores, bestType)
		if secondBest > 0 && (bestScore-secondBest)/bestScore < 0.3 {
			// Close competition - reduce confidence
			confidence *= 0.8
		}
	}

	return bestType, confidence
}

// ClassifyWithMatches returns the classification along with matched patterns.
// Useful for debugging and explanation.
func (c *FastClassifier) ClassifyWithMatches(input string) (TaskType, float64, []string) {
	lower := strings.ToLower(input)
	matches := []string{}

	scores := make(map[TaskType]float64)

	for taskType, patterns := range c.patterns {
		for _, p := range patterns {
			if p.regex.MatchString(lower) {
				scores[taskType] += p.weight
				matches = append(matches, p.regex.String())
			}
		}
	}

	taskType, confidence := c.Classify(input)
	return taskType, confidence, matches
}

// findSecondBest returns the second highest score.
func findSecondBest(scores map[TaskType]float64, best TaskType) float64 {
	var second float64
	for taskType, score := range scores {
		if taskType != best && score > second {
			second = score
		}
	}
	return second
}

// buildPatterns creates the regex patterns for each task type.
// Patterns are weighted: higher weight = stronger signal.
func buildPatterns() map[TaskType][]*compiledPattern {
	return map[TaskType][]*compiledPattern{
		// Code Review patterns
		TaskReview: {
			{regexp.MustCompile(`\b(review|audit|check)\s+(this|my|the|code)`), 1.0},
			{regexp.MustCompile(`\b(code\s+review|pr\s+review|pull\s+request\s+review)\b`), 1.2},
			{regexp.MustCompile(`\b(look\s+at|examine|inspect)\s+(this|my|the)\s+(code|file|changes)`), 0.9},
			{regexp.MustCompile(`\bwhat\s+do\s+you\s+think\s+(of|about)\b`), 0.7},
			{regexp.MustCompile(`\b(feedback|suggestions)\s+(on|for)\b`), 0.8},
			{regexp.MustCompile(`\bis\s+this\s+(code|approach|implementation)\s+(good|ok|correct)\b`), 0.8},
		},

		// Debug patterns
		TaskDebug: {
			{regexp.MustCompile(`\b(debug|fix|error|bug|issue|broken|failing)\b`), 1.0},
			{regexp.MustCompile(`\b(crash|exception|traceback|stacktrace)\b`), 1.1},
			{regexp.MustCompile(`\b(why\s+(is|does|did|doesn't|won't|isn't))\b`), 0.9},
			{regexp.MustCompile(`\b(not\s+working|doesn't\s+work|won't\s+work)\b`), 1.0},
			{regexp.MustCompile(`\b(wrong|incorrect|unexpected)\s+(output|result|behavior)\b`), 0.9},
			{regexp.MustCompile(`\b(help|fix)\s+(me\s+)?(with\s+)?(this\s+)?error\b`), 1.0},
			{regexp.MustCompile(`\b(what's|what\s+is)\s+wrong\b`), 0.8},
			{regexp.MustCompile(`\berror:\s*`), 1.2},
			{regexp.MustCompile(`\bfailed\s+(to|with)\b`), 0.9},
		},

		// Planning/Architecture patterns
		TaskPlanning: {
			{regexp.MustCompile(`\b(plan|design|architect|structure)\b`), 1.0},
			{regexp.MustCompile(`\b(should\s+i|how\s+should\s+i)\b`), 0.8},
			{regexp.MustCompile(`\b(best\s+(approach|way|practice|method))\b`), 0.9},
			{regexp.MustCompile(`\b(recommend|suggest|advise)\b`), 0.7},
			{regexp.MustCompile(`\b(strategy|roadmap|architecture)\b`), 1.0},
			{regexp.MustCompile(`\bhow\s+(would|could)\s+you\s+(design|structure|organize)\b`), 1.0},
			{regexp.MustCompile(`\bwhat\s+(would|should)\s+be\s+the\s+best\b`), 0.8},
			{regexp.MustCompile(`\b(trade-?offs?|pros?\s+and\s+cons?)\b`), 0.9},
		},

		// Code Generation patterns
		TaskCodeGen: {
			{regexp.MustCompile(`\b(write|create|generate|implement|build|make)\s+(a|an|the|me)?\s*(new\s+)?(function|class|component|module|file|code|script|program)\b`), 1.2},
			{regexp.MustCompile(`\b(write|create|generate|implement|build|make)\s+.{0,30}(function|class|component|module|file|method)\b`), 1.0}, // More flexible pattern
			{regexp.MustCompile(`\b(add|create)\s+(a|an)?\s*(new\s+)?(feature|functionality|endpoint|api|route)\b`), 1.0},
			{regexp.MustCompile(`\bcan\s+you\s+(write|create|make|build)\b`), 0.9},
			{regexp.MustCompile(`\b(implement|code)\s+(this|the|a)\b`), 0.9},
			{regexp.MustCompile(`\bwrite\s+(me\s+)?(some\s+)?code\b`), 1.1},
			{regexp.MustCompile(`\bnew\s+.{0,20}(file|function|class|method|component)\b`), 0.9}, // More flexible for "new X component"
		},

		// Infrastructure/DevOps patterns
		TaskInfrastructure: {
			{regexp.MustCompile(`\b(configure|deploy|provision|setup)\s+(the\s+)?(server|cluster|network|infrastructure)\b`), 1.2},
			{regexp.MustCompile(`\bdeploy\s+(to\s+)?(production|staging|prod|dev)\b`), 1.1}, // Deploy to production
			{regexp.MustCompile(`\b(ssh|server|switch|router|firewall|load\s*balancer)\b`), 1.0},
			{regexp.MustCompile(`\b(network|vlan|bgp|ospf|dns|dhcp|subnet|ip\s+address)\b`), 1.1},
			{regexp.MustCompile(`\b(docker|kubernetes|k8s|helm|terraform|ansible|puppet|chef)\b`), 1.0},
			{regexp.MustCompile(`\b(aws|gcp|azure|cloud|ec2|s3|lambda|ecs|eks)\b`), 0.9},
			{regexp.MustCompile(`\b(ci/?cd|pipeline|jenkins|github\s+actions|gitlab\s+ci)\b`), 0.9},
			{regexp.MustCompile(`\b(nginx|apache|haproxy|envoy|traefik)\b`), 0.8},
			{regexp.MustCompile(`\b(linux|ubuntu|centos|debian|rhel|bash|shell)\b`), 0.7},
		},

		// Explanation patterns
		TaskExplain: {
			{regexp.MustCompile(`\b(explain|what\s+is|what\s+does|how\s+does)\b`), 1.0},
			{regexp.MustCompile(`\b(tell\s+me\s+about|describe|walk\s+me\s+through)\b`), 0.9},
			{regexp.MustCompile(`\b(what\s+are|why\s+is|why\s+does)\b`), 0.8},
			{regexp.MustCompile(`\b(can\s+you\s+explain|help\s+me\s+understand)\b`), 1.0},
			{regexp.MustCompile(`\b(meaning|purpose|reason)\s+(of|for|behind)\b`), 0.8},
			{regexp.MustCompile(`\bhow\s+(does|do)\s+(this|it|the)\s+work\b`), 0.9},
		},

		// Refactor patterns
		TaskRefactor: {
			{regexp.MustCompile(`\b(refactor|restructure|reorganize|clean\s*up)\b`), 1.2},
			{regexp.MustCompile(`\b(improve|optimize|simplify)\s+(this|the|my)\s+(code|function|class|structure)\b`), 1.1}, // Added "structure"
			{regexp.MustCompile(`\bimprove\s+.{0,20}(code|structure|readability)\b`), 1.0}, // More flexible improve pattern
			{regexp.MustCompile(`\b(make\s+(this|it)\s+(cleaner|better|more\s+readable))\b`), 0.9},
			{regexp.MustCompile(`\b(extract|inline|rename|move)\s+(method|function|class|variable)\b`), 1.0},
			{regexp.MustCompile(`\b(reduce|remove)\s+(duplication|complexity|code\s+smell)\b`), 0.9},
			{regexp.MustCompile(`\b(dry|solid|kiss)\b`), 0.7},
		},
	}
}

// ExtractMention checks if the input contains an explicit @mention.
// Returns the mention name and the remaining input, or empty strings if none found.
func ExtractMention(input string) (mention string, remaining string) {
	// Pattern: @name at the start of input
	mentionRegex := regexp.MustCompile(`^@(\w+)\s+(.*)$`)
	matches := mentionRegex.FindStringSubmatch(strings.TrimSpace(input))

	if len(matches) == 3 {
		return strings.ToLower(matches[1]), matches[2]
	}

	return "", input
}

// MentionToTaskType maps @mentions to task types.
var mentionToTaskType = map[string]TaskType{
	"review":    TaskReview,
	"debug":     TaskDebug,
	"fix":       TaskDebug,
	"plan":      TaskPlanning,
	"architect": TaskPlanning,
	"code":      TaskCodeGen,
	"write":     TaskCodeGen,
	"generate":  TaskCodeGen,
	"infra":     TaskInfrastructure,
	"devops":    TaskInfrastructure,
	"explain":   TaskExplain,
	"refactor":  TaskRefactor,
}

// GetTaskTypeFromMention returns the TaskType for a given @mention.
func GetTaskTypeFromMention(mention string) (TaskType, bool) {
	taskType, ok := mentionToTaskType[strings.ToLower(mention)]
	return taskType, ok
}

// min returns the smaller of two float64 values.
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
