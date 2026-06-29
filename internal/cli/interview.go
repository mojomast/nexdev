package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mojomast/nexdev/internal/config"
	"github.com/mojomast/nexdev/internal/interview"
	"github.com/mojomast/nexdev/internal/provider"
	"github.com/mojomast/nexdev/internal/state"
	"github.com/mojomast/nexdev/internal/tui"
	"github.com/spf13/cobra"
)

var (
	interviewResume   bool
	interviewModel    string
	interviewProvider string
	interviewMode     string
	interviewTUI      bool
)

var interviewCmd = &cobra.Command{
	Use:   "interview",
	Short: "Start or resume project interview",
	Long: `Start a new project interview or resume an existing one.

The interview uses an AI assistant to have a natural conversation about your project,
gathering requirements through interactive dialogue rather than predefined questions.

Modes:
  --mode=chat    Interactive AI-driven conversation (default)
  --mode=guided  Traditional structured interview

The AI interviewer will guide you through:
  - Project essence (problem, users, value)
  - Technical constraints (stack, performance, scale)
  - Integration points (APIs, databases, auth)
  - Scope definition (MVP features, timeline)

Commands during chat:
  summary - Show current understanding
  done    - Complete interview early
  help    - Show available commands
  back    - Return to previous topic

The TUI provides a rich interface with chat history, keyboard shortcuts,
and real-time conversation tracking.`,
	RunE: runInterview,
}

func init() {
	interviewCmd.Flags().BoolVar(&interviewResume, "resume", false, "Resume existing interview")
	interviewCmd.Flags().StringVar(&interviewModel, "model", "", "Model to use for interview")
	interviewCmd.Flags().StringVar(&interviewProvider, "provider", "", "Provider to use for interview")
	interviewCmd.Flags().StringVar(&interviewMode, "mode", "chat", "Interview mode: 'chat' (AI-driven) or 'guided' (structured)")
	interviewCmd.Flags().BoolVar(&interviewTUI, "tui", true, "Use interactive TUI for interview")
}

func runInterview(cmd *cobra.Command, args []string) error {
	fmt.Println("🎤 Starting Project Interview...")
	fmt.Println("════════════════════════════════════════════════════════")
	fmt.Println()

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

	project, err := store.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("project not found. Please run 'geoffrussy init' first: %w", err)
	}

	providerName, modelName, err := selectProviderAndModel(cfgMgr, interviewProvider, interviewModel)
	if err != nil {
		return fmt.Errorf("failed to select provider and model: %w", err)
	}

	fmt.Printf("📦 Using Provider: %s\n", providerName)
	fmt.Printf("🤖 Using Model: %s\n", modelName)
	fmt.Printf("💬 Mode: %s\n", interviewMode)
	fmt.Println()

	bridge := provider.NewBridge()
	if err := setupProvider(bridge, cfgMgr, providerName); err != nil {
		return fmt.Errorf("failed to setup provider: %w", err)
	}

	prov, err := bridge.GetProvider(providerName)
	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}
	printProviderUsageSnapshot(providerName, prov)

	if interviewMode == "guided" {
		return runGuidedInterview(store, prov, modelName, projectID)
	}

	if interviewTUI {
		return runChatInterviewTUI(store, prov, modelName, projectID, project.Name, providerName)
	}

	return runChatInterview(store, prov, modelName, projectID)
}

func selectProviderAndModel(cfgMgr *config.Manager, preferredProvider, preferredModel string) (string, string, error) {
	cfg := cfgMgr.GetConfig()

	if len(cfg.APIKeys) == 0 && len(cfg.APIKeySources) == 0 {
		return "", "", fmt.Errorf("no API keys configured. Run 'geoffrussy config' to set up providers")
	}

	if preferredModel != "" && preferredProvider != "" {
		return preferredProvider, preferredModel, nil
	}

	if preferredModel != "" {
		providerName := guessProviderFromModel(preferredModel)
		if providerName == "" {
			if _, ok := cfg.APIKeys["requesty"]; ok {
				providerName = "requesty"
			} else if _, ok := cfg.APIKeys["openrouter"]; ok {
				providerName = "openrouter"
			}
		}
		if providerName != "" {
			if _, ok := cfg.APIKeys[providerName]; ok {
				return providerName, preferredModel, nil
			}
		}
	}

	defaultModel, err := cfgMgr.ResolveDefaultModel("interview.run")
	if err == nil && defaultModel != "" && preferredProvider == "" {
		providerName := guessProviderFromModel(defaultModel)
		if providerName == "" {
			for p := range cfg.APIKeys {
				providerName = p
				break
			}
		}
		if providerName != "" {
			return providerName, defaultModel, nil
		}
	}

	return interactiveProviderModelSelection(cfgMgr)
}

func interactiveProviderModelSelection(cfgMgr *config.Manager) (string, string, error) {
	cfg := cfgMgr.GetConfig()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          Select Provider & Model for Interview             ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("Fetching available models...")
	allModels, snapshots, err := loadAllProviderModels(cfgMgr, cfg)
	if err != nil || len(allModels) == 0 {
		return selectProviderFallback(cfgMgr, cfg)
	}

	if len(snapshots) > 0 {
		fmt.Println("\nProvider Status:")
		for name, snap := range snapshots {
			status := "✓"
			if snap.rate != nil && snap.rate.RequestsRemaining == 0 {
				status = "⚠ rate limited"
			}
			fmt.Printf("   %s: %s\n", name, status)
		}
	}

	favorites := cfgMgr.GetFavoriteModels()
	var favoriteModels []provider.Model
	var otherModels []provider.Model

	for _, m := range allModels {
		isFav := false
		for _, fav := range favorites {
			if fav == m.Name {
				isFav = true
				break
			}
		}
		if isFav {
			favoriteModels = append(favoriteModels, m)
		} else {
			otherModels = append(otherModels, m)
		}
	}

	sort.Slice(favoriteModels, func(i, j int) bool {
		return favoriteModels[i].Name < favoriteModels[j].Name
	})
	sort.Slice(otherModels, func(i, j int) bool {
		if otherModels[i].Provider != otherModels[j].Provider {
			return otherModels[i].Provider < otherModels[j].Provider
		}
		return otherModels[i].Name < otherModels[j].Name
	})

	sortedModels := append(favoriteModels, otherModels...)

	fmt.Println("\nAvailable Models:")
	fmt.Println("─────────────────────────────────────────────────────")

	displayLimit := 20
	if len(sortedModels) < displayLimit {
		displayLimit = len(sortedModels)
	}

	for i := 0; i < displayLimit; i++ {
		m := sortedModels[i]
		prefix := "   "
		for _, fav := range favorites {
			if fav == m.Name {
				prefix = "⭐ "
				break
			}
		}
		fmt.Printf("  %2d) %s%s (%s)\n", i+1, prefix, m.Name, strings.Title(m.Provider))
	}

	if len(sortedModels) > displayLimit {
		fmt.Printf("\n  ... and %d more models (type name to select)\n", len(sortedModels)-displayLimit)
	}

	fmt.Printf("\nSelect model (1-%d) or type model name: ", displayLimit)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if index := 0; input != "" {
		if _, err := fmt.Sscanf(input, "%d", &index); err == nil && index >= 1 && index <= displayLimit {
			selected := sortedModels[index-1]
			return selected.Provider, selected.Name, nil
		}

		for _, m := range sortedModels {
			if strings.EqualFold(m.Name, input) || strings.Contains(strings.ToLower(m.Name), strings.ToLower(input)) {
				return m.Provider, m.Name, nil
			}
		}
	}

	return selectProviderFallback(cfgMgr, cfg)
}

func selectProviderFallback(cfgMgr *config.Manager, cfg *config.Config) (string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nSelect a provider:")
	providers := make([]string, 0)
	for p := range cfg.APIKeys {
		providers = append(providers, p)
	}
	for p := range cfg.APIKeySources {
		found := false
		for _, existing := range providers {
			if existing == p {
				found = true
				break
			}
		}
		if !found {
			providers = append(providers, p)
		}
	}

	if len(providers) == 0 {
		return "", "", fmt.Errorf("no providers configured")
	}

	sort.Strings(providers)

	for i, p := range providers {
		fmt.Printf("  %d) %s\n", i+1, strings.Title(p))
	}

	fmt.Print("\nSelection: ")
	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	index := 0
	if _, err := fmt.Sscanf(selection, "%d", &index); err != nil || index < 1 || index > len(providers) {
		return "", "", fmt.Errorf("invalid selection")
	}

	selectedProvider := providers[index-1]
	defaultModels := map[string]string{
		"openai":     "gpt-4o",
		"anthropic":  "claude-sonnet-4-20250514",
		"groq":       "llama-3.1-70b-versatile",
		"mistral":    "mistral-large-latest",
		"openrouter": "anthropic/claude-sonnet-4",
		"requesty":   "openai/gpt-4o",
		"ollama":     "llama3.2",
		"together":   "meta-llama/Llama-3-70b-chat-hf",
		"deepinfra":  "meta-llama/Llama-3-70b-chat-hf",
		"fireworks":  "accounts/fireworks/models/llama-v3-70b-chat",
		"perplexity": "llama-3.1-sonar-large-128k-online",
		"kimi":       "moonshot-v1-8k",
		"zai":        "z-coder-v1",
	}

	model := defaultModels[selectedProvider]
	if model == "" {
		model = "gpt-4o"
	}

	fmt.Printf("\nEnter model name (default: %s): ", model)
	modelInput, _ := reader.ReadString('\n')
	modelInput = strings.TrimSpace(modelInput)

	if modelInput != "" {
		model = modelInput
	}

	return selectedProvider, model, nil
}

func runChatInterviewTUI(store *state.Store, prov provider.Provider, modelName, projectID, projectName, providerName string) error {
	engine := interview.NewChatEngine(store, prov, modelName)
	var session *interview.ChatSession
	var err error

	if interviewResume {
		session, err = engine.LoadChatSession(projectID)
		if err != nil {
			session = engine.StartChatSession(projectID)
		}
	} else {
		session = engine.StartChatSession(projectID)
	}

	model := tui.NewInterviewModel(projectName, providerName, modelName)

	if len(session.Messages) == 0 {
		greeting := engine.GetGreeting()
		session.Messages = append(session.Messages, interview.ChatMessage{
			Role:      "assistant",
			Content:   greeting,
			Timestamp: session.StartedAt,
		})
	}

	for _, msg := range session.Messages {
		model.AddMessage(msg.Role, msg.Content)
	}
	if session.Completed {
		model.SetCompleted(true)
	}

	model.SetOnSendMessage(func(input string) (string, error) {
		response, err := engine.SendMessage(session, input)
		if err != nil {
			return "", err
		}

		if err := engine.SaveChatSession(session); err != nil {
			return "", err
		}

		if session.Completed {
			model.SetCompleted(true)
			if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
				return "", err
			}
		}

		return response, nil
	})

	model.SetOnComplete(func() error {
		session.Completed = true
		if err := engine.SaveChatSession(session); err != nil {
			return err
		}
		return store.UpdateProjectStage(projectID, state.StageDesign)
	})

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if finalModel, ok := finalModel.(tui.InterviewModel); ok {
		if finalModel.IsCompleted() {
			fmt.Println("\n════════════════════════════════════════════════════════")
			fmt.Println("✅ Interview completed successfully!")
			fmt.Println("════════════════════════════════════════════════════════")
			fmt.Println("\n💡 Next steps:")
			fmt.Println("   Run 'geoffrussy design' to generate architecture")
		} else {
			fmt.Println("\n👋 Interview paused. Run 'geoffrussy interview --resume' to continue.")
		}
	}

	return nil
}

func runChatInterview(store *state.Store, prov provider.Provider, modelName, projectID string) error {
	engine := interview.NewChatEngine(store, prov, modelName)
	var session *interview.ChatSession
	var err error

	if interviewResume {
		fmt.Println("🔄 Resuming interview from previous session...")
		session, err = engine.LoadChatSession(projectID)
		if err != nil {
			fmt.Println("   Could not load previous session, starting fresh...")
			session = engine.StartChatSession(projectID)
		} else if len(session.Messages) > 0 {
			fmt.Println("   Previous conversation loaded. Continuing...")
			fmt.Println()
			fmt.Println("─────────────────────────────────────────────────────────")
			for i, msg := range session.Messages {
				if i > len(session.Messages)-6 && i >= 0 {
					switch msg.Role {
					case "user":
						fmt.Printf("👤 You: %s\n", msg.Content)
					case "assistant":
						fmt.Printf("🤖 AI: %s\n", msg.Content)
					}
				}
			}
			fmt.Println("─────────────────────────────────────────────────────────")
			fmt.Println()
		}
	} else {
		fmt.Println("🆕 Starting new interview session...")
		session = engine.StartChatSession(projectID)
	}

	reader := bufio.NewReader(os.Stdin)

	if len(session.Messages) == 0 {
		greeting := engine.GetGreeting()
		session.Messages = append(session.Messages, interview.ChatMessage{
			Role:      "assistant",
			Content:   greeting,
			Timestamp: session.StartedAt,
		})
		fmt.Printf("\n🤖 AI: %s\n", greeting)
	}

	fmt.Println("\n💡 Commands: 'summary', 'done', 'help'")
	fmt.Println()

	for {
		fmt.Print("👤 You: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if input == "help" {
			fmt.Println("\n📖 Available commands:")
			fmt.Println("   summary - Show what I've learned so far")
			fmt.Println("   done    - Complete the interview")
			fmt.Println("   back    - Go back to a previous topic")
			fmt.Println("   help    - Show this help message")
			fmt.Println()
			continue
		}

		if input == "back" {
			fmt.Println("\n🤖 AI: What topic would you like to revisit?")
			fmt.Println("   - project (problem, users, value)")
			fmt.Println("   - technical (stack, performance)")
			fmt.Println("   - integration (APIs, database, auth)")
			fmt.Println("   - scope (features, timeline)")
			fmt.Println()
			continue
		}

		response, err := engine.SendMessage(session, input)
		if err != nil {
			fmt.Printf("⚠️ Error: %v\n", err)
			continue
		}

		fmt.Printf("\n🤖 AI: %s\n\n", response)

		if err := engine.SaveChatSession(session); err != nil {
			fmt.Printf("⚠️ Warning: Could not save session: %v\n", err)
		}

		if session.Completed {
			if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
				fmt.Printf("⚠️ Warning: Could not update project stage: %v\n", err)
			}

			fmt.Println("\n════════════════════════════════════════════════════════")
			fmt.Println("✅ Interview completed successfully!")
			fmt.Println("════════════════════════════════════════════════════════")
			fmt.Println("\n💡 Next steps:")
			fmt.Println("   Run 'geoffrussy design' to generate architecture")
			return nil
		}
	}
}

func runGuidedInterview(store *state.Store, prov provider.Provider, modelName, projectID string) error {
	engine := interview.NewEngine(store, prov, modelName)

	var session *interview.InterviewSession
	var err error

	if interviewResume {
		fmt.Println("🔄 Resuming interview from previous session...")
		session, err = engine.ResumeInterview(projectID)
		if err != nil {
			return fmt.Errorf("failed to resume interview: %w", err)
		}
	} else {
		fmt.Println("🆕 Starting new interview session...")
		session, err = engine.StartInterview(projectID)
		if err != nil {
			return fmt.Errorf("failed to start interview: %w", err)
		}
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		question, err := engine.GetNextQuestion(session)
		if err != nil {
			return fmt.Errorf("failed to get next question: %w", err)
		}

		if question == nil {
			if err := engine.SaveSession(session); err != nil {
				return fmt.Errorf("failed to save completed session: %w", err)
			}
			if err := store.UpdateProjectStage(projectID, state.StageDesign); err != nil {
				return fmt.Errorf("failed to update project stage: %w", err)
			}

			complete, missing := engine.ValidateCompleteness(session)
			if complete {
				fmt.Println("════════════════════════════════════════════════════════")
				fmt.Println("✅ Interview completed successfully!")

				summary, err := engine.GenerateSummary(session)
				if err != nil {
					return fmt.Errorf("failed to generate summary: %w", err)
				}

				fmt.Println("\n📊 Interview Summary")
				fmt.Println("─────────────────────────────────────────────")
				fmt.Println(summary)

				fmt.Println("\n💡 Next steps:")
				fmt.Println("   Run 'geoffrussy design' to generate architecture")
			} else {
				fmt.Println("⚠️  Interview is incomplete. Missing required answers:")
				for _, m := range missing {
					fmt.Printf("   - %s\n", m)
				}
			}

			return nil
		}

		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("Phase %s - Question %d\n", session.CurrentPhase, session.CurrentQuestion)
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Printf("\n%s\n\n", question.Text)

		fmt.Printf("Your answer (or 'help' for suggestions, 'back' to go back): ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(answer)

		if answer == "back" {
			fmt.Println("⏮️  Going to previous question...")
			continue
		}

		if answer == "help" {
			fmt.Println("\n💡 Suggestions:")
			defaultAns, err := engine.ProposeDefault(*question)
			if err == nil && defaultAns != "" {
				fmt.Printf("   Suggested answer: %s\n", defaultAns)
			}
			fmt.Println("   - Be specific about your problem")
			fmt.Println("   - Mention your target users")
			fmt.Println()
			continue
		}

		if err := engine.RecordAnswer(session, question.ID, answer); err != nil {
			return fmt.Errorf("failed to record answer: %w", err)
		}

		if err := engine.SaveSession(session); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		fmt.Println("✅ Answer saved!")
	}
}
