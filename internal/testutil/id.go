package testutil

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// FakeIDGenerator returns stable, sortable IDs with caller-supplied prefixes.
type FakeIDGenerator struct {
	mu       sync.Mutex
	counters map[string]int64
}

func NewFakeIDGenerator() *FakeIDGenerator {
	return &FakeIDGenerator{counters: make(map[string]int64)}
}

func (g *FakeIDGenerator) Next(prefix string) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	prefix = strings.TrimSuffix(prefix, "_")
	g.counters[prefix]++
	return fmt.Sprintf("%s_%026d", prefix, g.counters[prefix])
}

func (g *FakeIDGenerator) ProjectID() string { return g.Next("proj") }
func (g *FakeIDGenerator) RunID() string     { return g.Next("run") }
func (g *FakeIDGenerator) EventID() string   { return g.Next("evt") }
func (g *FakeIDGenerator) TokenID() string   { return g.Next("tok") }

func AssertStableIDSequence(t testing.TB, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ID count = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("ID[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
