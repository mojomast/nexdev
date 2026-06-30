package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mojomast/nexdev/internal/executor"
	"github.com/mojomast/nexdev/internal/state"
)

type DevelopStageConfig struct {
	ProjectRoot string
	Worker      executor.TaskWorker
}

type DevelopStage struct {
	config  DevelopStageConfig
	reports []executor.TaskReport
}

func NewDevelopStage(cfg DevelopStageConfig) *DevelopStage {
	return &DevelopStage{config: cfg}
}

func (s *DevelopStage) Name() Stage { return StageDevelop }

func (s *DevelopStage) Validate(ctx context.Context, env StageEnv) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if env.Project == nil {
		return fmt.Errorf("develop requires project")
	}
	if env.Run == nil {
		return fmt.Errorf("develop requires run")
	}
	if _, ok := env.Store.(*state.Store); !ok {
		return fmt.Errorf("develop requires *state.Store")
	}
	if err := s.requireReviewApproval(); err != nil {
		return err
	}
	return nil
}

func (s *DevelopStage) Run(ctx context.Context, env StageEnv) error {
	if err := s.Validate(ctx, env); err != nil {
		return err
	}
	store := env.Store.(*state.Store)
	bridge, err := executor.NewNexdevExecutor(executor.NexdevExecutorConfig{Store: store, ProjectID: env.Project.ProjectID(), RunID: env.Run.RunID(), ProjectRoot: s.projectRoot(), Worker: s.config.Worker})
	if err != nil {
		return err
	}
	reports, err := bridge.RunPending(ctx)
	s.reports = reports
	return err
}

func (s *DevelopStage) Resume(ctx context.Context, env StageEnv) error { return s.Run(ctx, env) }

func (s *DevelopStage) Output(ctx context.Context, env StageEnv) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return map[string]any{"task_reports": s.reports, "task_report_count": len(s.reports)}, nil
}

func (s *DevelopStage) requireReviewApproval() error {
	data, err := os.ReadFile(filepath.Join(s.projectRoot(), reviewApprovalArtifactRelPath))
	if err != nil {
		return fmt.Errorf("develop requires reviewed approved plan: %w", err)
	}
	var marker struct {
		Marker   string `json:"marker"`
		Approved bool   `json:"approved"`
	}
	if err := json.Unmarshal(data, &marker); err != nil {
		return fmt.Errorf("develop review approval marker is invalid: %w", err)
	}
	if marker.Marker != reviewApprovedMarker || !marker.Approved {
		return fmt.Errorf("develop requires %s review approval marker", reviewApprovedMarker)
	}
	return nil
}

func (s *DevelopStage) projectRoot() string {
	if s.config.ProjectRoot == "" {
		return "."
	}
	return s.config.ProjectRoot
}
