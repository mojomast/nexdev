package cli

import (
	"fmt"
	"strings"
	"testing"
)

func buildProgressBarLegacy(percent int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	barLength := 40
	filled := percent * barLength / 100
	bar := "  ["
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += fmt.Sprintf("] %d%%", percent)
	return bar
}

func buildProgressBarOptimized(percent int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	barLength := 40
	filled := percent * barLength / 100

	bar := "  [" + strings.Repeat("█", filled) + strings.Repeat("░", barLength-filled) + fmt.Sprintf("] %d%%", percent)
	return bar
}

func BenchmarkDisplayProgressBarLegacy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buildProgressBarLegacy(50)
	}
}

func BenchmarkDisplayProgressBarOptimized(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buildProgressBarOptimized(50)
	}
}
