package pipeline

import (
	"context"
	"reflect"
	"testing"
)

func TestCanonicalStageOrder(t *testing.T) {
	want := []Stage{
		StageInit,
		StageRepoAnalyze,
		StageInterview,
		StageComplexity,
		StageDesign,
		StageHivemind,
		StageValidate,
		StagePlanSketch,
		StagePlanDetail,
		StageReview,
		StageDevelop,
		StageVerify,
		StageHandoff,
		StageComplete,
	}

	if !reflect.DeepEqual(CanonicalStages, want) {
		t.Fatalf("canonical stages mismatch\ngot:  %#v\nwant: %#v", CanonicalStages, want)
	}

	for i, stage := range want {
		got, ok := StageIndex(stage)
		if !ok {
			t.Fatalf("stage %s should be indexed", stage)
		}
		if got != i {
			t.Fatalf("stage %s index = %d, want %d", stage, got, i)
		}
	}
}

func TestDetourPseudoStage(t *testing.T) {
	if IsCanonicalStage(StageDetour) {
		t.Fatal("detour must not be canonical")
	}
	if !IsPseudoStage(StageDetour) || !IsKnownStage(StageDetour) {
		t.Fatal("detour must be known pseudo-stage")
	}
	if !CanNavigateStage(StageDevelop, StageDetour) {
		t.Fatal("develop must navigate to detour")
	}
	if !CanNavigateStage(StageDetour, StageDevelop) {
		t.Fatal("detour must return to develop")
	}
	if CanNavigateStage(StageReview, StageDetour) {
		t.Fatal("detour must be reachable only from develop")
	}
	if CanNavigateStage(StageDetour, StageVerify) {
		t.Fatal("detour must return only to develop")
	}
}

type compileOnlyStage struct{}

func (compileOnlyStage) Name() Stage                              { return StageInit }
func (compileOnlyStage) Run(context.Context, StageEnv) error      { return nil }
func (compileOnlyStage) Validate(context.Context, StageEnv) error { return nil }
func (compileOnlyStage) Resume(context.Context, StageEnv) error   { return nil }

func TestPipelineStageInterfaceCompiles(t *testing.T) {
	var _ PipelineStage = compileOnlyStage{}
}
