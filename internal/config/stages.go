package config

import "strings"

var stageAliases = map[string]string{
	"plan": "devplan",
}

var knownStageKeys = []string{
	"interview.run",
	"interview.followup",
	"interview.analysis",
	"interview.defaults",
	"design.generate",
	"design.refine",
	"devplan.generate",
	"review.phase",
	"develop.execute",
	"develop.blocker_analyze",
	"interview",
	"design",
	"devplan",
	"review",
	"develop",
}

func normalizeStage(stage string) string {
	stage = strings.TrimSpace(strings.ToLower(stage))
	if alias, ok := stageAliases[stage]; ok {
		return alias
	}
	return stage
}

func getStageCandidates(stage string) []string {
	stage = normalizeStage(stage)
	if stage == "" {
		return nil
	}

	candidates := []string{stage}

	parts := strings.Split(stage, ".")
	for len(parts) > 1 {
		parts = parts[:len(parts)-1]
		candidates = append(candidates, strings.Join(parts, "."))
	}

	if stage == "devplan" {
		candidates = append(candidates, "plan")
	}

	seen := make(map[string]bool, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		result = append(result, c)
	}

	return result
}

func KnownStageKeys() []string {
	keys := make([]string, len(knownStageKeys))
	copy(keys, knownStageKeys)
	return keys
}
