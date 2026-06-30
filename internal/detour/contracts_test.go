package detour

import (
	"context"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/pipeline"
)

func TestDetourContractInterfacesCompile(t *testing.T) {
	var _ Generator = fakeGenerator{}
	var _ Splicer = fakeSplicer{}
	var _ DepthPolicy = fakeDepthPolicy{}
}

func TestDetourContractUsesSharedTypes(t *testing.T) {
	req := Request{Source: contract.DetourSourceBlockerAuto}
	result := Result{NewTasks: []contract.TaskSpec{{ID: "D1.01"}}, Depth: 1}
	splice := SpliceResult{EventStage: pipeline.StageDetour, SplicedAfter: "T1.01"}

	if req.Source != contract.DetourSourceBlockerAuto {
		t.Fatalf("detour request source did not use shared contract source")
	}
	if len(result.NewTasks) != 1 || result.NewTasks[0].ID != "D1.01" {
		t.Fatalf("detour result did not use shared TaskSpec")
	}
	if splice.EventStage != pipeline.StageDetour {
		t.Fatalf("splice event stage = %q, want %q", splice.EventStage, pipeline.StageDetour)
	}
}

func TestDepthContractDefault(t *testing.T) {
	check := fakeDepthPolicy{}.CheckDepth(DefaultMaxDepth, DefaultMaxDepth)
	if check.Decision != DepthDecisionBlock {
		t.Fatalf("depth at max should block, got %q", check.Decision)
	}
	if check.BlockerReason != "detour_depth_exceeded" {
		t.Fatalf("depth blocker reason = %q", check.BlockerReason)
	}
}

type fakeGenerator struct{}

func (fakeGenerator) GenerateDetour(context.Context, RequestContext) (contract.DetourResult, error) {
	return contract.DetourResult{}, nil
}

type fakeSplicer struct{}

func (fakeSplicer) SpliceDetour(context.Context, SpliceRequest) (SpliceResult, error) {
	return SpliceResult{}, nil
}

type fakeDepthPolicy struct{}

func (fakeDepthPolicy) CheckDepth(currentDepth, maxDepth int) DepthCheck {
	if currentDepth >= maxDepth {
		return DepthCheck{CurrentDepth: currentDepth, MaxDepth: maxDepth, Decision: DepthDecisionBlock, BlockerReason: "detour_depth_exceeded"}
	}
	return DepthCheck{CurrentDepth: currentDepth, MaxDepth: maxDepth, Decision: DepthDecisionAllow}
}
