package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/design"
	"github.com/mojomast/nexdev/internal/devplan"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/spf13/cobra"
)

var (
	planModel   string
	planMerge   string
	planSplit   string
	planReorder bool
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate or manipulate development plan",
	Long: `Generate development plan from architecture or manipulate
existing plan by merging, splitting, or reordering phases.`,
	RunE: runPlan,
}

func init() {
	planCmd.Flags().StringVar(&planModel, "model", "", "Model to use for plan generation")
	planCmd.Flags().StringVar(&planMerge, "merge", "", "Merge phases (format: 1,2)")
	planCmd.Flags().StringVar(&planSplit, "split", "", "Split phase (format: 1:3 - split phase 1 at task 3)")
	planCmd.Flags().BoolVar(&planReorder, "reorder", false, "Reorder phases interactively")
}

func runPlan(cmd *cobra.Command, args []string) error {
	fmt.Println("📋 Development Plan Management...")

	// Initialize infrastructure
	cfgMgr := config.NewManager()
	if err := cfgMgr.Load(nil); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	projectID := filepath.Base(cwd)

	dbPath := filepath.Join(cwd, ".geoffrussy", "state.db")
	store, err := state.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open state store: %w", err)
	}
	defer store.Close()

	// Check if project exists
	_, err = store.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found: %w. Please run 'geoffrussy init' first", err)
	}

	// Determine operation mode
	isManipulation := planMerge != "" || planSplit != "" || planReorder

	if isManipulation {
		return handlePlanManipulation(store, projectID)
	}

	return handlePlanGeneration(store, cfgMgr, projectID)
}

// handlePlanGeneration generates a new plan
func handlePlanGeneration(store *state.Store, cfgMgr *config.Manager, projectID string) error {
	fmt.Println("   Generating development plan...")

	// Load architecture
	arch, err := store.GetArchitecture(projectID)
	if err != nil {
		return fmt.Errorf("failed to load architecture: %w. Run 'geoffrussy design' first", err)
	}

	// Load interview data
	interviewData, err := store.GetInterviewData(projectID)
	if err != nil {
		return fmt.Errorf("failed to load interview data: %w", err)
	}

	// Reconstruct a minimal design.Architecture for generation
	// We extract "System Overview" section from arch.Content as it's used by the generator
	systemOverview := extractSystemOverview(arch.Content)
	designArch := &design.Architecture{
		SystemOverview: systemOverview,
	}

	// Setup provider
	prov, modelName, providerName, err := setupPlanProvider(cfgMgr, planModel)
	if err != nil {
		return err
	}
	fmt.Printf("   Using model: %s\n", modelName)
	printProviderUsageSnapshot(providerName, prov)

	generator := devplan.NewGenerator(prov, modelName, store)

	phases, err := generator.GeneratePhases(designArch, interviewData)
	if err != nil {
		return fmt.Errorf("failed to generate phases: %w", err)
	}

	fmt.Printf("   Generated %d phases.\n", len(phases))

	// Save phases
	for i := range phases {
		// Ensure ID is set
		if phases[i].ID == "" {
			phases[i].ID = fmt.Sprintf("phase-%d", phases[i].Number)
		}

		statePhase, stateTasks, err := convertDevPlanToState(generator, &phases[i], projectID)
		if err != nil {
			return fmt.Errorf("failed to convert phase %d: %w", phases[i].Number, err)
		}

		if err := store.SavePhase(statePhase); err != nil {
			return fmt.Errorf("failed to save phase %d: %w", phases[i].Number, err)
		}

		for _, task := range stateTasks {
			if err := store.SaveTask(task); err != nil {
				return fmt.Errorf("failed to save task %s: %w", task.ID, err)
			}
		}
	}

	// Update project stage
	if err := store.UpdateProjectStage(projectID, state.StagePlan); err != nil {
		return fmt.Errorf("failed to update project stage: %w", err)
	}

	if len(phases) > 0 {
		project, err := store.GetProject(projectID)
		if err == nil {
			project.CurrentPhase = phases[0].ID
			if updateErr := store.UpdateProject(project); updateErr != nil {
				return fmt.Errorf("failed to update current phase: %w", updateErr)
			}
		}
	}

	fmt.Println("✅ Plan generated and saved successfully!")
	return nil
}

func extractSystemOverview(content string) string {
	// Simple extraction: look for "## System Overview" and take text until next "## "
	lines := strings.Split(content, "\n")
	var overviewBuilder strings.Builder
	inOverview := false

	for _, line := range lines {
		if strings.HasPrefix(line, "## System Overview") {
			inOverview = true
			continue
		}
		if inOverview {
			if strings.HasPrefix(line, "## ") {
				break
			}
			overviewBuilder.WriteString(line)
			overviewBuilder.WriteString("\n")
		}
	}
	return strings.TrimSpace(overviewBuilder.String())
}

// handlePlanManipulation handles plan manipulation
func handlePlanManipulation(store *state.Store, projectID string) error {
	fmt.Println("   Manipulating development plan...")

	// Load existing phases
	statePhases, err := store.ListPhases(projectID)
	if err != nil {
		return fmt.Errorf("failed to load phases: %w", err)
	}

	if len(statePhases) == 0 {
		return fmt.Errorf("no phases found. Run 'geoffrussy plan' (without manipulation flags) first to generate a plan")
	}

	// Convert to devplan phases
	phases, err := convertStatePhasesToDevplan(store, statePhases)
	if err != nil {
		return fmt.Errorf("failed to convert phases: %w", err)
	}

	// Create generator (needed for manipulation methods)
	generator := devplan.NewGenerator(nil, "", store)

	if planMerge != "" {
		return executeMerge(store, generator, phases, projectID, planMerge)
	}

	if planSplit != "" {
		return executeSplit(store, generator, phases, projectID, planSplit)
	}

	if planReorder {
		return executeReorder(store, generator, phases, projectID)
	}

	return nil
}

func executeMerge(store *state.Store, generator *devplan.Generator, phases []devplan.Phase, projectID, mergeSpec string) error {
	parts := strings.Split(mergeSpec, ",")
	if len(parts) != 2 {
		return fmt.Errorf("invalid merge format. Use 'phase1,phase2' (e.g. 1,2)")
	}

	idx1, err1 := strconv.Atoi(parts[0])
	idx2, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return fmt.Errorf("invalid phase numbers: %s", mergeSpec)
	}

	var phase1, phase2 *devplan.Phase
	var p1Idx, p2Idx int

	for i := range phases {
		if phases[i].Number == idx1 {
			phase1 = &phases[i]
			p1Idx = i
		}
		if phases[i].Number == idx2 {
			phase2 = &phases[i]
			p2Idx = i
		}
	}

	if phase1 == nil || phase2 == nil {
		return fmt.Errorf("one or both phases not found")
	}
	if p1Idx == p2Idx {
		return fmt.Errorf("cannot merge the same phase")
	}

	merged, err := generator.MergePhases(phase1, phase2)
	if err != nil {
		return fmt.Errorf("failed to merge phases: %w", err)
	}

	insertIdx := p1Idx
	removeIdx := p2Idx
	if p2Idx < p1Idx {
		insertIdx = p2Idx
		removeIdx = p1Idx
	}

	finalPhases := make([]devplan.Phase, 0, len(phases)-1)
	for i, p := range phases {
		switch i {
		case insertIdx:
			finalPhases = append(finalPhases, *merged)
		case removeIdx:
			continue
		default:
			finalPhases = append(finalPhases, p)
		}
	}

	return saveAllPhases(store, generator, finalPhases, projectID)
}

func executeSplit(store *state.Store, generator *devplan.Generator, phases []devplan.Phase, projectID, splitSpec string) error {
	parts := strings.Split(splitSpec, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid split format. Use 'phase:taskIndex' (e.g. 1:3)")
	}

	phaseNum, err1 := strconv.Atoi(parts[0])
	splitPoint, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return fmt.Errorf("invalid split parameters")
	}

	var targetPhase *devplan.Phase
	var targetIdx int
	found := false
	for i := range phases {
		if phases[i].Number == phaseNum {
			targetPhase = &phases[i]
			targetIdx = i
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("phase %d not found", phaseNum)
	}

	// Convert 1-based task index to split point
	adjustedSplitPoint := splitPoint - 1

	splitPhases, err := generator.SplitPhase(targetPhase, adjustedSplitPoint)
	if err != nil {
		return fmt.Errorf("failed to split phase: %w", err)
	}

	// Construct new list
	newPhases := []devplan.Phase{}
	for i, p := range phases {
		if i == targetIdx {
			for _, sp := range splitPhases {
				newPhases = append(newPhases, *sp)
			}
		} else {
			newPhases = append(newPhases, p)
		}
	}

	return saveAllPhases(store, generator, newPhases, projectID)
}

func executeReorder(store *state.Store, generator *devplan.Generator, phases []devplan.Phase, projectID string) error {
	fmt.Println("Current Phases:")
	for _, p := range phases {
		fmt.Printf(" %d: %s\n", p.Number, p.Title)
	}

	fmt.Print("\nEnter new order (comma-separated indices): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	parts := strings.Split(input, ",")
	newOrder := make([]int, len(parts))
	for i, p := range parts {
		idx, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return fmt.Errorf("invalid index: %s", p)
		}
		newOrder[i] = idx
	}

	reordered, err := generator.ReorderPhases(phases, newOrder)
	if err != nil {
		return fmt.Errorf("failed to reorder phases: %w", err)
	}

	return saveAllPhases(store, generator, reordered, projectID)
}

func saveAllPhases(store *state.Store, generator *devplan.Generator, phases []devplan.Phase, projectID string) error {
	// First, list existing phases to delete them
	existing, err := store.ListPhases(projectID)
	if err == nil {
		for _, p := range existing {
			_ = store.DeletePhase(p.ID)
		}
	}

	for i := range phases {
		// Ensure consistent ID and numbering
		phases[i].Number = i
		phases[i].ID = fmt.Sprintf("phase-%d", i)

		// Update task numbering
		for j := range phases[i].Tasks {
			phases[i].Tasks[j].Number = fmt.Sprintf("%d.%d", i, j+1)
		}

		statePhase, stateTasks, err := convertDevPlanToState(generator, &phases[i], projectID)
		if err != nil {
			return err
		}
		if err := store.SavePhase(statePhase); err != nil {
			return err
		}
		for _, task := range stateTasks {
			if err := store.SaveTask(task); err != nil {
				return err
			}
		}
	}

	fmt.Println("✅ Plan updated successfully!")
	return nil
}

// Helpers

func setupPlanProvider(cfgMgr *config.Manager, model string) (provider.Provider, string, string, error) {
	providerName, modelName, err := getProviderAndModel(cfgMgr, "devplan.generate", model)
	if err != nil {
		return nil, "", "", err
	}

	bridge := provider.NewBridge()
	if err := setupProvider(bridge, cfgMgr, providerName); err != nil {
		return nil, "", "", err
	}

	prov, err := bridge.GetProvider(providerName)
	if err != nil {
		return nil, "", "", err
	}

	return prov, modelName, providerName, nil
}

// convertStatePhasesToDevplan converts state phases to devplan phases
func convertStatePhasesToDevplan(store *state.Store, statePhases []*state.Phase) ([]devplan.Phase, error) {
	phases := make([]devplan.Phase, len(statePhases))

	for i, sp := range statePhases {
		// Parse structured data from Markdown content
		parsedPhase, err := devplan.ParsePhaseMarkdown(sp.Content)
		if err != nil {
			// If parsing fails, create a minimal phase from DB data
			// This might happen if content is corrupted or empty
			fmt.Printf("Warning: failed to parse phase %d content: %v. Using fallback.\n", sp.Number, err)
			phases[i] = devplan.Phase{
				ID:        sp.ID,
				Number:    sp.Number,
				Title:     sp.Title,
				Status:    devplan.PhaseStatus(sp.Status),
				CreatedAt: sp.CreatedAt,
			}
		} else {
			phases[i] = *parsedPhase
			// Override fields that should come from DB source of truth
			phases[i].ID = sp.ID
			phases[i].Number = sp.Number
			phases[i].Title = sp.Title
			phases[i].Status = devplan.PhaseStatus(sp.Status)
			phases[i].CreatedAt = sp.CreatedAt
		}

		// Load tasks from DB to get their IDs and Status (source of truth)
		dbTasks, err := store.ListTasks(sp.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list tasks for phase %s: %w", sp.ID, err)
		}

		// Map DB tasks to devplan tasks
		// The challenge: DevPlan tasks from Markdown might differ from DB tasks if manually edited
		// We try to match by Number

		dbTaskMap := make(map[string]state.Task)
		for _, t := range dbTasks {
			dbTaskMap[t.Number] = t
		}

		for j, t := range phases[i].Tasks {
			if dbTask, exists := dbTaskMap[t.Number]; exists {
				phases[i].Tasks[j].ID = dbTask.ID
				phases[i].Tasks[j].Status = devplan.TaskStatus(dbTask.Status)
			}
		}
	}

	return phases, nil
}

// convertDevPlanToState converts devplan phase to state phase and tasks
func convertDevPlanToState(g *devplan.Generator, phase *devplan.Phase, projectID string) (*state.Phase, []*state.Task, error) {
	// Generate markdown content
	content, err := g.ExportPhaseMarkdown(phase)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to export phase markdown: %w", err)
	}

	statePhase := &state.Phase{
		ID:        phase.ID,
		ProjectID: projectID,
		Number:    phase.Number,
		Title:     phase.Title,
		Content:   content,
		Status:    state.PhaseStatus(phase.Status),
		CreatedAt: phase.CreatedAt,
	}

	var stateTasks []*state.Task
	for _, t := range phase.Tasks {
		// If ID is empty, generate one?
		// Ideally ID should be preserved if it existed.
		// If it's a new task, we might need to generate an ID.
		// For now, if ID is empty, we assume the caller or store handles it,
		// but Store expects ID.
		taskID := t.ID
		if taskID == "" {
			taskID = fmt.Sprintf("task-%s-%s", phase.ID, strings.ReplaceAll(t.Number, ".", "-"))
		}

		stateTask := &state.Task{
			ID:          taskID,
			PhaseID:     phase.ID,
			Number:      t.Number,
			Description: t.Description,
			Status:      state.TaskStatus(t.Status),
		}
		stateTasks = append(stateTasks, stateTask)
	}

	return statePhase, stateTasks, nil
}
