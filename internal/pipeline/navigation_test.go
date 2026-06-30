package pipeline

import (
	"errors"
	"reflect"
	"testing"
)

func TestRequiredPrerequisitesMatchSpec(t *testing.T) {
	cases := []struct {
		stage Stage
		want  []RequirementKey
	}{
		{StageInit, nil},
		{StageRepoAnalyze, []RequirementKey{RequirementProjectExists}},
		{StageInterview, []RequirementKey{RequirementProjectExists}},
		{StageComplexity, []RequirementKey{RequirementInterviewDataExists}},
		{StageDesign, []RequirementKey{RequirementInterviewDataExists, RequirementRepoAnalysisExists}},
		{StageHivemind, []RequirementKey{RequirementDesignDraftExists}},
		{StageValidate, []RequirementKey{RequirementDesignDraftExists, RequirementLatestHivemindSynthesisExists}},
		{StagePlanSketch, []RequirementKey{RequirementValidationPassedOrWarningsAccepted}},
		{StagePlanDetail, []RequirementKey{RequirementPhaseSketchExists}},
		{StageReview, []RequirementKey{RequirementDetailedPlanExists}},
		{StageDevelop, []RequirementKey{RequirementReviewedApprovedPlanExists}},
		{StageVerify, []RequirementKey{RequirementDevelopHasNoRunningTasks}},
		{StageHandoff, []RequirementKey{RequirementVerifyCompleteOrExplicitlySkipped}},
		{StageComplete, []RequirementKey{RequirementHandoffExists, RequirementAllRequiredReportsExist}},
		{StageDetour, []RequirementKey{RequirementActiveDevelopRun, RequirementActiveBlockerOrManualOperatorRequest}},
	}

	for _, tc := range cases {
		t.Run(string(tc.stage), func(t *testing.T) {
			got, err := RequiredPrerequisites(tc.stage, PrerequisiteSnapshot{})
			if err != nil {
				t.Fatalf("RequiredPrerequisites returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("prerequisites mismatch\ngot:  %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

func TestValidatePrerequisitesAllowsSkippedHivemind(t *testing.T) {
	snapshot := PrerequisiteSnapshot{
		Satisfied: map[RequirementKey]bool{
			RequirementDesignDraftExists: true,
		},
		SkippedStages: map[Stage]bool{
			StageHivemind: true,
		},
	}

	if err := ValidatePrerequisites(StageValidate, snapshot); err != nil {
		t.Fatalf("validate should not require synthesis when hivemind skipped: %v", err)
	}
}

func TestValidatePrerequisitesReportsMissingRequirements(t *testing.T) {
	err := ValidatePrerequisites(StageDesign, NewPrerequisiteSnapshot(RequirementInterviewDataExists))
	if err == nil {
		t.Fatal("expected missing prerequisite error")
	}

	var prereqErr *PrerequisiteError
	if !errors.As(err, &prereqErr) {
		t.Fatalf("expected PrerequisiteError, got %T", err)
	}
	if prereqErr.Stage != StageDesign {
		t.Fatalf("stage = %s, want %s", prereqErr.Stage, StageDesign)
	}
	if !reflect.DeepEqual(prereqErr.Missing, []RequirementKey{RequirementRepoAnalysisExists}) {
		t.Fatalf("missing = %#v, want repo analysis", prereqErr.Missing)
	}
}

func TestValidateStageNavigation(t *testing.T) {
	full := NewPrerequisiteSnapshot(
		RequirementProjectExists,
		RequirementActiveDevelopRun,
		RequirementActiveBlockerOrManualOperatorRequest,
	)

	if err := ValidateStageNavigation(StageInit, StageRepoAnalyze, full); err != nil {
		t.Fatalf("init -> repo_analyze should be valid: %v", err)
	}
	if err := ValidateStageNavigation(StageDevelop, StageDetour, full); err != nil {
		t.Fatalf("develop -> detour should be valid: %v", err)
	}
	if err := ValidateStageNavigation(StageDetour, StageDevelop, NewPrerequisiteSnapshot(RequirementReviewedApprovedPlanExists)); err != nil {
		t.Fatalf("detour -> develop should be valid: %v", err)
	}
	if err := ValidateStageNavigation(StageInit, StageComplexity, full); err == nil {
		t.Fatal("skipping canonical stages should be invalid")
	}
	if err := ValidateStageNavigation(StageReview, StageDetour, full); err == nil {
		t.Fatal("detour from non-develop should be invalid")
	}
}
