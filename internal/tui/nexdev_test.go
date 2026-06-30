package tui

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mojomast/nexdev/internal/contract"
)

type fakeClient struct {
	snapshot Snapshot
	disabled bool
	paused   int
	resumed  int
	skipped  int
	steered  int
	detours  int
	canceled int
}

func (f *fakeClient) Snapshot(context.Context) (Snapshot, error) { return f.snapshot, nil }
func (f *fakeClient) Pause(context.Context, string) error {
	if f.disabled {
		return errors.New("service_unavailable: executor control is not wired")
	}
	f.paused++
	return nil
}
func (f *fakeClient) Resume(context.Context) error {
	if f.disabled {
		return errors.New("service_unavailable: executor control is not wired")
	}
	f.resumed++
	return nil
}
func (f *fakeClient) Skip(context.Context, string, string) error {
	if f.disabled {
		return errors.New("service_unavailable: executor control is not wired")
	}
	f.skipped++
	return nil
}
func (f *fakeClient) Steer(context.Context, string, string) error {
	if f.disabled {
		return errors.New("service_unavailable: steering input is deferred")
	}
	f.steered++
	return nil
}
func (f *fakeClient) RequestDetour(context.Context, contract.DetourRequest) (contract.DetourResult, error) {
	if f.disabled {
		return contract.DetourResult{}, errors.New("service_unavailable: detour manager is not wired")
	}
	f.detours++
	return contract.DetourResult{ID: "detour_1"}, nil
}
func (f *fakeClient) Cancel(context.Context, string) error {
	if f.disabled {
		return errors.New("service_unavailable: executor control is not wired")
	}
	f.canceled++
	return nil
}

func TestModelRefreshAndRenderFakeRunState(t *testing.T) {
	client := &fakeClient{snapshot: fixtureSnapshot()}
	model := NewModel(client)
	msg := model.Init()().(snapshotMsg)
	updated, _ := model.Update(msg)
	m := updated.(Model)

	view := m.View()
	for _, want := range []string{"Nexdev Terminal Control", "run_1", "develop", "current task: T1.1", "tasks: 1 total", "provider summary service is not wired"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestViewNavigationAndEventRenderingRedactsSecrets(t *testing.T) {
	client := &fakeClient{snapshot: fixtureSnapshot()}
	model := NewModel(client)
	updated, _ := model.Update(snapshotMsg{snapshot: client.snapshot})
	m := updated.(Model)
	updated, _ = m.Update(keyMsg("2"))
	m = updated.(Model)

	view := m.View()
	if strings.Contains(view, "sk-test-secret") || strings.Contains(view, "Bearer token-secret") {
		t.Fatalf("view leaked secret:\n%s", view)
	}
	if !strings.Contains(view, "[REDACTED]") {
		t.Fatalf("view did not show redaction marker:\n%s", view)
	}
}

func TestDisabledActionsSurfaceStructuredStatus(t *testing.T) {
	client := &fakeClient{snapshot: fixtureSnapshot(), disabled: true}
	model := NewModel(client)
	updated, cmd := model.Update(snapshotMsg{snapshot: client.snapshot})
	m := updated.(Model)
	updated, cmd = m.Update(keyMsg("p"))
	if cmd == nil {
		t.Fatal("expected action command")
	}
	updated, _ = updated.(Model).Update(cmd())
	m = updated.(Model)

	if client.paused != 0 {
		t.Fatalf("disabled action should not mutate service count, got %d", client.paused)
	}
	if !strings.Contains(m.View(), "service_unavailable") {
		t.Fatalf("disabled status not rendered:\n%s", m.View())
	}
}

func TestQuitDoesNotCancelRun(t *testing.T) {
	client := &fakeClient{snapshot: fixtureSnapshot()}
	model := NewModel(client)
	updated, cmd := model.Update(snapshotMsg{snapshot: client.snapshot})
	m := updated.(Model)
	updated, cmd = m.Update(keyMsg("q"))
	m = updated.(Model)

	if !m.quitting || cmd == nil {
		t.Fatalf("quit did not set quitting and tea quit command")
	}
	if client.canceled != 0 {
		t.Fatalf("quit canceled run unexpectedly")
	}
}

func TestCancelRequiresExplicitConfirmation(t *testing.T) {
	client := &fakeClient{snapshot: fixtureSnapshot()}
	model := NewModel(client)
	updated, _ := model.Update(snapshotMsg{snapshot: client.snapshot})
	m := updated.(Model)
	updated, cmd := m.Update(keyMsg("c"))
	m = updated.(Model)
	if cmd != nil || client.canceled != 0 || m.confirm != "cancel" {
		t.Fatalf("cancel should wait for confirmation")
	}
	updated, cmd = m.Update(keyMsg("y"))
	if cmd == nil {
		t.Fatal("expected confirmed cancel command")
	}
	updated, _ = updated.(Model).Update(cmd())
	m = updated.(Model)
	if client.canceled != 1 {
		t.Fatalf("confirmed cancel count = %d", client.canceled)
	}
	if m.confirm != "" {
		t.Fatalf("confirmation not cleared")
	}
}

func TestSkipRequiresExplicitConfirmation(t *testing.T) {
	client := &fakeClient{snapshot: fixtureSnapshot()}
	model := NewModel(client)
	updated, _ := model.Update(snapshotMsg{snapshot: client.snapshot})
	m := updated.(Model)
	updated, cmd := m.Update(keyMsg("k"))
	m = updated.(Model)
	if cmd != nil || client.skipped != 0 || m.confirm != "skip" {
		t.Fatalf("skip should wait for confirmation")
	}
	updated, cmd = m.Update(keyMsg("y"))
	if cmd == nil {
		t.Fatal("expected confirmed skip command")
	}
	updated, _ = updated.(Model).Update(cmd())
	if client.skipped != 1 {
		t.Fatalf("confirmed skip count = %d", client.skipped)
	}
}

func fixtureSnapshot() Snapshot {
	payload, _ := json.Marshal(map[string]string{"message": "started with api_key=sk-test-secret and Authorization: Bearer token-secret"})
	return Snapshot{
		ProjectID: "proj_1",
		Run:       RunSummary{RunID: "run_1", Status: "running", CurrentStage: "develop", CurrentTask: "T1.1", Cost: "$0.01"},
		Tasks:     []TaskSummary{{ID: "T1.1", PhaseID: "phase_001", Title: "Implement TUI", Status: "running", RiskLevel: "medium"}},
		Events:    []contract.EventEnvelope{{EventID: "evt_1", Sequence: 1, Type: contract.EventTypeTaskStarted, ProjectID: "proj_1", RunID: "run_1", TaskID: "T1.1", Timestamp: time.Now(), Payload: payload}},
		Blockers:  []BlockerSummary{{ID: "blk_1", TaskID: "T1.1", Reason: "needs_input", Description: "operator token=secret", Status: "open"}},
		Artifacts: []ArtifactSummary{{ID: "art_1", Kind: "devplan", Path: ".nexdev/artifacts/devplan.md", Version: 1}},
		Config:    ConfigSummary{Profile: "dev", Bind: "127.0.0.1:7432", Redacted: true},
	}
}

func keyMsg(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
