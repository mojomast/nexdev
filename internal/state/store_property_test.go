package state

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty4_StatePreservationRoundTrip tests Property 4: State Preservation Round-Trip
// **Validates: Requirements 14.1, 14.2, 14.3**
//
// Property: For any project state (interview answers, architecture, DevPlan, task progress),
// saving the state then loading it should produce an equivalent state with no data loss.
func TestProperty4_StatePreservationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100 // Minimum 100 iterations as per spec
	properties := gopter.NewProperties(parameters)

	// Test Project round-trip
	properties.Property("Project: saving then loading preserves data", prop.ForAll(
		func(project *Project) bool {
			store, err := NewStore(":memory:")
			if err != nil {
				t.Logf("Failed to create store: %v", err)
				return false
			}
			defer store.Close()

			// Save project
			err = store.CreateProject(project)
			if err != nil {
				t.Logf("Failed to save project: %v", err)
				return false
			}

			// Load project
			loaded, err := store.GetProject(project.ID)
			if err != nil {
				t.Logf("Failed to load project: %v", err)
				return false
			}

			// Compare - timestamps may have slight differences due to database precision
			return projectsEqual(project, loaded)
		},
		genProject(),
	))

	// Test InterviewData round-trip
	properties.Property("InterviewData: saving then loading preserves data", prop.ForAll(
		func(projectID string, data *InterviewData) bool {
			store, err := NewStore(":memory:")
			if err != nil {
				t.Logf("Failed to create store: %v", err)
				return false
			}
			defer store.Close()

			// Create project first (foreign key requirement)
			project := &Project{
				ID:           projectID,
				Name:         "Test Project",
				CreatedAt:    time.Now(),
				CurrentStage: StageInterview,
			}
			err = store.CreateProject(project)
			if err != nil {
				t.Logf("Failed to create project: %v", err)
				return false
			}

			// Save interview data
			err = store.SaveInterviewData(projectID, data)
			if err != nil {
				t.Logf("Failed to save interview data: %v", err)
				return false
			}

			// Load interview data
			loaded, err := store.GetInterviewData(projectID)
			if err != nil {
				t.Logf("Failed to load interview data: %v", err)
				return false
			}

			// Compare
			return interviewDataEqual(data, loaded)
		},
		gen.Identifier(),
		genInterviewData(),
	))

	// Test Architecture round-trip
	properties.Property("Architecture: saving then loading preserves data", prop.ForAll(
		func(projectID string, arch *Architecture) bool {
			store, err := NewStore(":memory:")
			if err != nil {
				t.Logf("Failed to create store: %v", err)
				return false
			}
			defer store.Close()

			// Create project first
			project := &Project{
				ID:           projectID,
				Name:         "Test Project",
				CreatedAt:    time.Now(),
				CurrentStage: StageDesign,
			}
			err = store.CreateProject(project)
			if err != nil {
				t.Logf("Failed to create project: %v", err)
				return false
			}

			// Set architecture project ID to match
			arch.ProjectID = projectID

			// Save architecture
			err = store.SaveArchitecture(projectID, arch)
			if err != nil {
				t.Logf("Failed to save architecture: %v", err)
				return false
			}

			// Load architecture
			loaded, err := store.GetArchitecture(projectID)
			if err != nil {
				t.Logf("Failed to load architecture: %v", err)
				return false
			}

			// Compare
			return architectureEqual(arch, loaded)
		},
		gen.Identifier(),
		genArchitecture(),
	))

	// Test Phase round-trip
	properties.Property("Phase: saving then loading preserves data", prop.ForAll(
		func(projectID string, phase *Phase) bool {
			store, err := NewStore(":memory:")
			if err != nil {
				t.Logf("Failed to create store: %v", err)
				return false
			}
			defer store.Close()

			// Create project first
			project := &Project{
				ID:           projectID,
				Name:         "Test Project",
				CreatedAt:    time.Now(),
				CurrentStage: StagePlan,
			}
			err = store.CreateProject(project)
			if err != nil {
				t.Logf("Failed to create project: %v", err)
				return false
			}

			// Set phase project ID
			phase.ProjectID = projectID

			// Save phase
			err = store.SavePhase(phase)
			if err != nil {
				t.Logf("Failed to save phase: %v", err)
				return false
			}

			// Load phase
			loaded, err := store.GetPhase(phase.ID)
			if err != nil {
				t.Logf("Failed to load phase: %v", err)
				return false
			}

			// Compare
			return phaseEqual(phase, loaded)
		},
		gen.Identifier(),
		genPhase(),
	))

	// Test Task round-trip
	properties.Property("Task: saving then loading preserves data", prop.ForAll(
		func(projectID string, phaseID string, task *Task) bool {
			store, err := NewStore(":memory:")
			if err != nil {
				t.Logf("Failed to create store: %v", err)
				return false
			}
			defer store.Close()

			// Create project first
			project := &Project{
				ID:           projectID,
				Name:         "Test Project",
				CreatedAt:    time.Now(),
				CurrentStage: StageDevelop,
			}
			err = store.CreateProject(project)
			if err != nil {
				t.Logf("Failed to create project: %v", err)
				return false
			}

			// Create phase
			phase := &Phase{
				ID:        phaseID,
				ProjectID: projectID,
				Number:    1,
				Title:     "Test Phase",
				Content:   "Test content",
				Status:    PhaseInProgress,
				CreatedAt: time.Now(),
			}
			err = store.SavePhase(phase)
			if err != nil {
				t.Logf("Failed to create phase: %v", err)
				return false
			}

			// Set task phase ID
			task.PhaseID = phaseID

			// Save task
			err = store.SaveTask(task)
			if err != nil {
				t.Logf("Failed to save task: %v", err)
				return false
			}

			// Load task
			loaded, err := store.GetTask(task.ID)
			if err != nil {
				t.Logf("Failed to load task: %v", err)
				return false
			}

			// Compare
			return taskEqual(task, loaded)
		},
		gen.Identifier(),
		gen.Identifier(),
		genTask(),
	))

	properties.TestingRun(t)
}

// Generators for property-based testing

func genProject() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.AlphaString(),
		genStage(),
		gen.Identifier(),
	).Map(func(vals []interface{}) *Project {
		return &Project{
			ID:           vals[0].(string),
			Name:         vals[1].(string),
			CreatedAt:    time.Now().Truncate(time.Second), // Truncate to avoid precision issues
			CurrentStage: vals[2].(Stage),
			CurrentPhase: vals[3].(string),
		}
	})
}

func genStage() gopter.Gen {
	return gen.OneConstOf(
		StageInit,
		StageInterview,
		StageDesign,
		StagePlan,
		StageReview,
		StageDevelop,
		StageComplete,
	)
}

func genInterviewData() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.SliceOf(gen.AlphaString()),
		gen.SliceOf(gen.AlphaString()),
		genTechStack(),
		gen.SliceOf(genIntegration()),
		genScope(),
		gen.SliceOf(gen.AlphaString()),
		gen.SliceOf(gen.AlphaString()),
		gen.SliceOf(gen.AlphaString()),
		gen.SliceOf(genRefinement()),
	).Map(func(vals []interface{}) *InterviewData {
		return &InterviewData{
			ProjectID:         vals[0].(string),
			ProjectName:       vals[1].(string),
			CreatedAt:         time.Now().Truncate(time.Second),
			ProblemStatement:  vals[2].(string),
			TargetUsers:       vals[3].([]string),
			SuccessMetrics:    vals[4].([]string),
			TechnicalStack:    vals[5].(TechStack),
			Integrations:      vals[6].([]Integration),
			Scope:             vals[7].(Scope),
			Constraints:       vals[8].([]string),
			Assumptions:       vals[9].([]string),
			Unknowns:          vals[10].([]string),
			RefinementHistory: vals[11].([]Refinement),
		}
	})
}

func genTechStack() gopter.Gen {
	return gopter.CombineGens(
		genTechChoice(),
		genTechChoice(),
		genTechChoice(),
		genTechChoice(),
		genTechChoice(),
	).Map(func(vals []interface{}) TechStack {
		return TechStack{
			Backend:        vals[0].(TechChoice),
			Frontend:       vals[1].(TechChoice),
			Database:       vals[2].(TechChoice),
			Cache:          vals[3].(TechChoice),
			Infrastructure: vals[4].(TechChoice),
		}
	})
}

func genTechChoice() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
	).Map(func(vals []interface{}) TechChoice {
		return TechChoice{
			Language:  vals[0].(string),
			Framework: vals[1].(string),
			Version:   vals[2].(string),
			Rationale: vals[3].(string),
		}
	})
}

func genIntegration() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString(),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.Bool(),
	).Map(func(vals []interface{}) Integration {
		return Integration{
			Name:     vals[0].(string),
			Type:     vals[1].(string),
			Purpose:  vals[2].(string),
			Required: vals[3].(bool),
		}
	})
}

func genScope() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOf(gen.AlphaString()),
		gen.SliceOf(gen.AlphaString()),
		gen.AlphaString(),
		gen.AlphaString(),
	).Map(func(vals []interface{}) Scope {
		return Scope{
			MVPFeatures:    vals[0].([]string),
			Phase2Features: vals[1].([]string),
			Timeline:       vals[2].(string),
			Resources:      vals[3].(string),
		}
	})
}

func genRefinement() gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, 10),
		gen.SliceOf(gen.AlphaString()),
		gen.AlphaString(),
	).Map(func(vals []interface{}) Refinement {
		return Refinement{
			Iteration:  vals[0].(int),
			Timestamp:  time.Now().Truncate(time.Second),
			Changes:    vals[1].([]string),
			ApprovedBy: vals[2].(string),
		}
	})
}

func genArchitecture() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	).Map(func(vals []interface{}) *Architecture {
		return &Architecture{
			ProjectID: vals[0].(string),
			Content:   vals[1].(string),
			CreatedAt: time.Now().Truncate(time.Second),
		}
	})
}

func genPhase() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.Identifier(),
		gen.IntRange(0, 20),
		gen.AlphaString(),
		gen.AlphaString(),
		genPhaseStatus(),
		genOptionalTime(),
		genOptionalTime(),
	).Map(func(vals []interface{}) *Phase {
		return &Phase{
			ID:          vals[0].(string),
			ProjectID:   vals[1].(string),
			Number:      vals[2].(int),
			Title:       vals[3].(string),
			Content:     vals[4].(string),
			Status:      vals[5].(PhaseStatus),
			CreatedAt:   time.Now().Truncate(time.Second),
			StartedAt:   vals[6].(*time.Time),
			CompletedAt: vals[7].(*time.Time),
		}
	})
}

func genPhaseStatus() gopter.Gen {
	return gen.OneConstOf(
		PhaseNotStarted,
		PhaseInProgress,
		PhaseCompleted,
		PhaseBlocked,
	)
}

func genTask() gopter.Gen {
	return gopter.CombineGens(
		gen.Identifier(),
		gen.Identifier(),
		gen.AlphaString(),
		gen.AlphaString(),
		genTaskStatus(),
		genOptionalTime(),
		genOptionalTime(),
	).Map(func(vals []interface{}) *Task {
		return &Task{
			ID:          vals[0].(string),
			PhaseID:     vals[1].(string),
			Number:      vals[2].(string),
			Description: vals[3].(string),
			Status:      vals[4].(TaskStatus),
			StartedAt:   vals[5].(*time.Time),
			CompletedAt: vals[6].(*time.Time),
		}
	})
}

func genTaskStatus() gopter.Gen {
	return gen.OneConstOf(
		TaskNotStarted,
		TaskInProgress,
		TaskCompleted,
		TaskBlocked,
		TaskSkipped,
	)
}

func genOptionalTime() gopter.Gen {
	return gen.Bool().Map(func(hasTime bool) *time.Time {
		if hasTime {
			t := time.Now().Truncate(time.Second)
			return &t
		}
		return nil
	})
}

// Equality comparison functions

func projectsEqual(a, b *Project) bool {
	if a.ID != b.ID {
		return false
	}
	if a.Name != b.Name {
		return false
	}
	if a.CurrentStage != b.CurrentStage {
		return false
	}
	if a.CurrentPhase != b.CurrentPhase {
		return false
	}
	// Compare timestamps with tolerance for database precision
	if !timesEqual(a.CreatedAt, b.CreatedAt) {
		return false
	}
	return true
}

func interviewDataEqual(a, b *InterviewData) bool {
	if a.ProjectID != b.ProjectID {
		return false
	}
	if a.ProjectName != b.ProjectName {
		return false
	}
	if a.ProblemStatement != b.ProblemStatement {
		return false
	}
	if !stringSlicesEqual(a.TargetUsers, b.TargetUsers) {
		return false
	}
	if !stringSlicesEqual(a.SuccessMetrics, b.SuccessMetrics) {
		return false
	}
	if !techStackEqual(a.TechnicalStack, b.TechnicalStack) {
		return false
	}
	if !integrationsEqual(a.Integrations, b.Integrations) {
		return false
	}
	if !scopeEqual(a.Scope, b.Scope) {
		return false
	}
	if !stringSlicesEqual(a.Constraints, b.Constraints) {
		return false
	}
	if !stringSlicesEqual(a.Assumptions, b.Assumptions) {
		return false
	}
	if !stringSlicesEqual(a.Unknowns, b.Unknowns) {
		return false
	}
	if !refinementsEqual(a.RefinementHistory, b.RefinementHistory) {
		return false
	}
	return true
}

func techStackEqual(a, b TechStack) bool {
	return techChoiceEqual(a.Backend, b.Backend) &&
		techChoiceEqual(a.Frontend, b.Frontend) &&
		techChoiceEqual(a.Database, b.Database) &&
		techChoiceEqual(a.Cache, b.Cache) &&
		techChoiceEqual(a.Infrastructure, b.Infrastructure)
}

func techChoiceEqual(a, b TechChoice) bool {
	return a.Language == b.Language &&
		a.Framework == b.Framework &&
		a.Version == b.Version &&
		a.Rationale == b.Rationale
}

func integrationsEqual(a, b []Integration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name ||
			a[i].Type != b[i].Type ||
			a[i].Purpose != b[i].Purpose ||
			a[i].Required != b[i].Required {
			return false
		}
	}
	return true
}

func scopeEqual(a, b Scope) bool {
	return stringSlicesEqual(a.MVPFeatures, b.MVPFeatures) &&
		stringSlicesEqual(a.Phase2Features, b.Phase2Features) &&
		a.Timeline == b.Timeline &&
		a.Resources == b.Resources
}

func refinementsEqual(a, b []Refinement) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Iteration != b[i].Iteration ||
			!stringSlicesEqual(a[i].Changes, b[i].Changes) ||
			a[i].ApprovedBy != b[i].ApprovedBy {
			return false
		}
	}
	return true
}

func architectureEqual(a, b *Architecture) bool {
	if a.ProjectID != b.ProjectID {
		return false
	}
	if a.Content != b.Content {
		return false
	}
	if !timesEqual(a.CreatedAt, b.CreatedAt) {
		return false
	}
	return true
}

func phaseEqual(a, b *Phase) bool {
	if a.ID != b.ID {
		return false
	}
	if a.ProjectID != b.ProjectID {
		return false
	}
	if a.Number != b.Number {
		return false
	}
	if a.Title != b.Title {
		return false
	}
	if a.Content != b.Content {
		return false
	}
	if a.Status != b.Status {
		return false
	}
	if !timesEqual(a.CreatedAt, b.CreatedAt) {
		return false
	}
	if !optionalTimesEqual(a.StartedAt, b.StartedAt) {
		return false
	}
	if !optionalTimesEqual(a.CompletedAt, b.CompletedAt) {
		return false
	}
	return true
}

func taskEqual(a, b *Task) bool {
	if a.ID != b.ID {
		return false
	}
	if a.PhaseID != b.PhaseID {
		return false
	}
	if a.Number != b.Number {
		return false
	}
	if a.Description != b.Description {
		return false
	}
	if a.Status != b.Status {
		return false
	}
	if !optionalTimesEqual(a.StartedAt, b.StartedAt) {
		return false
	}
	if !optionalTimesEqual(a.CompletedAt, b.CompletedAt) {
		return false
	}
	return true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func timesEqual(a, b time.Time) bool {
	// Allow 1 second tolerance for database precision
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	return diff <= time.Second
}

func optionalTimesEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return timesEqual(*a, *b)
}
