package common

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ExtractJSON extracts JSON content from a string, handling markdown code fences
func ExtractJSON(content string) (string, error) {
	// Try to find JSON in markdown code fences
	// Pattern 1: ```json ... ```
	jsonPattern := "```json\\s*\\n([\\s\\S]*?)\\n```"
	re := regexp.MustCompile(jsonPattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1]), nil
	}

	// Pattern 2: ``` ... ``` (generic code fence)
	genericPattern := "```\\s*\\n([\\s\\S]*?)\\n```"
	re = regexp.MustCompile(genericPattern)
	matches = re.FindStringSubmatch(content)
	if len(matches) > 1 {
		extracted := strings.TrimSpace(matches[1])
		// Verify it looks like JSON (starts with { or [)
		if strings.HasPrefix(extracted, "{") || strings.HasPrefix(extracted, "[") {
			return extracted, nil
		}
	}

	// Pattern 3: No code fences, but looks like JSON object or array
	trimmed := strings.TrimSpace(content)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		return trimmed, nil
	}

	// Pattern 4: Embedded JSON (find first {/[ and last }/])
	startBrace := strings.Index(content, "{")
	startBracket := strings.Index(content, "[")

	start := -1
	if startBrace != -1 && (startBracket == -1 || startBrace < startBracket) {
		start = startBrace
	} else if startBracket != -1 {
		start = startBracket
	}

	if start != -1 {
		// Find matching end
		end := -1
		lastBrace := strings.LastIndex(content, "}")
		lastBracket := strings.LastIndex(content, "]")

		if lastBrace > start && (lastBracket == -1 || lastBrace > lastBracket) {
			end = lastBrace
		} else if lastBracket > start {
			end = lastBracket
		}

		if end != -1 {
			return strings.TrimSpace(content[start : end+1]), nil
		}
	}

	return "", fmt.Errorf("no valid JSON found in content")
}

// ParseJSON parses JSON content into a struct, handling markdown extraction if needed
func ParseJSON(content string, v interface{}) error {
	// Try direct parsing first
	err := json.Unmarshal([]byte(content), v)
	if err == nil {
		return nil
	}

	// Try extracting JSON
	extracted, extractErr := ExtractJSON(content)
	if extractErr != nil {
		return fmt.Errorf("failed to extract JSON: %w (original error: %v)", extractErr, err)
	}

	// Try parsing extracted JSON
	if err := json.Unmarshal([]byte(extracted), v); err != nil {
		return fmt.Errorf("failed to parse extracted JSON: %w", err)
	}

	return nil
}
