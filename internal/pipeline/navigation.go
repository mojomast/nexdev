package pipeline

import (
	"fmt"
	"strings"
)

type RequirementKey string

const (
	RequirementProjectExists                        RequirementKey = "project_exists"
	RequirementInterviewDataExists                  RequirementKey = "interview_data_exists"
	RequirementRepoAnalysisExists                   RequirementKey = "repo_analysis_exists"
	RequirementDesignDraftExists                    RequirementKey = "design_draft_exists"
	RequirementLatestHivemindSynthesisExists        RequirementKey = "latest_hivemind_synthesis_exists"
	RequirementValidationPassedOrWarningsAccepted   RequirementKey = "validation_passed_or_warnings_accepted"
	RequirementPhaseSketchExists                    RequirementKey = "phase_sketch_exists"
	RequirementDetailedPlanExists                   RequirementKey = "detailed_plan_exists"
	RequirementReviewedApprovedPlanExists           RequirementKey = "reviewed_approved_plan_exists"
	RequirementDevelopHasNoRunningTasks             RequirementKey = "develop_has_no_running_tasks"
	RequirementVerifyCompleteOrExplicitlySkipped    RequirementKey = "verify_complete_or_explicitly_skipped"
	RequirementHandoffExists                        RequirementKey = "handoff_exists"
	RequirementAllRequiredReportsExist              RequirementKey = "all_required_reports_exist"
	RequirementActiveDevelopRun                     RequirementKey = "active_develop_run"
	RequirementActiveBlockerOrManualOperatorRequest RequirementKey = "active_blocker_or_manual_operator_request"
)

// PrerequisiteSnapshot is an import-cycle-free view of durable state and artifacts.
type PrerequisiteSnapshot struct {
	Satisfied     map[RequirementKey]bool
	SkippedStages map[Stage]bool
}

func NewPrerequisiteSnapshot(requirements ...RequirementKey) PrerequisiteSnapshot {
	snapshot := PrerequisiteSnapshot{Satisfied: map[RequirementKey]bool{}}
	for _, requirement := range requirements {
		snapshot.Satisfied[requirement] = true
	}
	return snapshot
}

func (s PrerequisiteSnapshot) Has(requirement RequirementKey) bool {
	return s.Satisfied != nil && s.Satisfied[requirement]
}

func (s PrerequisiteSnapshot) StageSkipped(stage Stage) bool {
	return s.SkippedStages != nil && s.SkippedStages[stage]
}

type PrerequisiteError struct {
	Stage   Stage
	Missing []RequirementKey
}

func (e *PrerequisiteError) Error() string {
	parts := make([]string, 0, len(e.Missing))
	for _, requirement := range e.Missing {
		parts = append(parts, string(requirement))
	}
	return fmt.Sprintf("stage %s missing prerequisites: %s", e.Stage, strings.Join(parts, ", "))
}

func RequiredPrerequisites(target Stage, snapshot PrerequisiteSnapshot) ([]RequirementKey, error) {
	switch target {
	case StageInit:
		return nil, nil
	case StageRepoAnalyze, StageInterview:
		return []RequirementKey{RequirementProjectExists}, nil
	case StageComplexity:
		return []RequirementKey{RequirementInterviewDataExists}, nil
	case StageDesign:
		return []RequirementKey{RequirementInterviewDataExists, RequirementRepoAnalysisExists}, nil
	case StageHivemind:
		return []RequirementKey{RequirementDesignDraftExists}, nil
	case StageValidate:
		required := []RequirementKey{RequirementDesignDraftExists}
		if !snapshot.StageSkipped(StageHivemind) {
			required = append(required, RequirementLatestHivemindSynthesisExists)
		}
		return required, nil
	case StagePlanSketch:
		return []RequirementKey{RequirementValidationPassedOrWarningsAccepted}, nil
	case StagePlanDetail:
		return []RequirementKey{RequirementPhaseSketchExists}, nil
	case StageReview:
		return []RequirementKey{RequirementDetailedPlanExists}, nil
	case StageDevelop:
		return []RequirementKey{RequirementReviewedApprovedPlanExists}, nil
	case StageVerify:
		return []RequirementKey{RequirementDevelopHasNoRunningTasks}, nil
	case StageHandoff:
		return []RequirementKey{RequirementVerifyCompleteOrExplicitlySkipped}, nil
	case StageComplete:
		return []RequirementKey{RequirementHandoffExists, RequirementAllRequiredReportsExist}, nil
	case StageDetour:
		return []RequirementKey{RequirementActiveDevelopRun, RequirementActiveBlockerOrManualOperatorRequest}, nil
	default:
		return nil, fmt.Errorf("unknown target stage %q", target)
	}
}

func ValidatePrerequisites(target Stage, snapshot PrerequisiteSnapshot) error {
	required, err := RequiredPrerequisites(target, snapshot)
	if err != nil {
		return err
	}
	missing := make([]RequirementKey, 0, len(required))
	for _, requirement := range required {
		if !snapshot.Has(requirement) {
			missing = append(missing, requirement)
		}
	}
	if len(missing) > 0 {
		return &PrerequisiteError{Stage: target, Missing: missing}
	}
	return nil
}

func CanNavigateStage(from, to Stage) bool {
	if to == StageDetour {
		return from == StageDevelop
	}
	if from == StageDetour {
		return to == StageDevelop
	}
	if !IsCanonicalStage(from) || !IsCanonicalStage(to) || from == to {
		return false
	}
	fromIndex, _ := StageIndex(from)
	toIndex, _ := StageIndex(to)
	return toIndex == fromIndex+1 || toIndex < fromIndex
}

func ValidateStageNavigation(from, to Stage, snapshot PrerequisiteSnapshot) error {
	if !IsKnownStage(from) {
		return fmt.Errorf("unknown source stage %q", from)
	}
	if !IsKnownStage(to) {
		return fmt.Errorf("unknown target stage %q", to)
	}
	if !CanNavigateStage(from, to) {
		return fmt.Errorf("invalid stage navigation %s -> %s", from, to)
	}
	return ValidatePrerequisites(to, snapshot)
}
