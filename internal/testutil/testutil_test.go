package testutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/controlplane"
)

func TestFakeClock(t *testing.T) {
	start := time.Date(2026, 6, 30, 12, 0, 0, 123, time.FixedZone("test", -7*60*60))
	clock := NewFakeClock(start)
	if got := clock.Now(); got.Location() != time.UTC || got.Format(time.RFC3339Nano) != "2026-06-30T19:00:00.000000123Z" {
		t.Fatalf("Now() = %s, want UTC normalized start", got.Format(time.RFC3339Nano))
	}
	if got := clock.Advance(2 * time.Second); got.Format(time.RFC3339Nano) != "2026-06-30T19:00:02.000000123Z" {
		t.Fatalf("Advance() = %s", got.Format(time.RFC3339Nano))
	}
}

func TestFakeIDGenerator(t *testing.T) {
	ids := NewFakeIDGenerator()
	got := []string{ids.ProjectID(), ids.ProjectID(), ids.RunID(), ids.EventID()}
	want := []string{
		"proj_00000000000000000000000001",
		"proj_00000000000000000000000002",
		"run_00000000000000000000000001",
		"evt_00000000000000000000000001",
	}
	AssertStableIDSequence(t, got, want)
}

func TestTempProject(t *testing.T) {
	project := TempProject(t)
	for _, path := range []string{project.Root, project.NexdevDir, project.ArtifactsDir, project.StateDir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", path)
		}
	}
	for _, path := range []string{project.ConfigPath, project.ReadmePath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(project.Root, ".env")); !os.IsNotExist(err) {
		t.Fatalf("TempProject created .env or could not stat it: %v", err)
	}
}

func TestEventRecorder(t *testing.T) {
	recorder := NewEventRecorder()
	recorder.Record(contract.EventEnvelope{EventID: "evt_2", Sequence: 2, Type: contract.EventTypeTaskCompleted, RunID: "run_1"})
	recorder.Record(contract.EventEnvelope{EventID: "evt_1", Sequence: 1, Type: contract.EventTypeTaskStarted, RunID: "run_1"})
	recorder.AssertTypesBySequence(t, contract.EventTypeTaskStarted, contract.EventTypeTaskCompleted)

	recorder = NewEventRecorder()
	recorder.Record(contract.EventEnvelope{EventID: "evt_1", Sequence: 1, Type: contract.EventTypeTaskStarted, RunID: "run_1"})
	recorder.Record(contract.EventEnvelope{EventID: "evt_2", Sequence: 2, Type: contract.EventTypeTaskCompleted, RunID: "run_1"})
	recorder.AssertMonotonicSequences(t)
}

func TestAuthFixtures(t *testing.T) {
	roles := AuthRoles()
	if len(roles) != 3 || roles[0] != controlplane.RoleObserver || roles[2] != controlplane.RoleAdmin {
		t.Fatalf("AuthRoles() = %#v", roles)
	}
	if got := RequiredRouteRole(t, "POST", "/runs"); got != controlplane.RoleOperator {
		t.Fatalf("RequiredRouteRole() = %s, want %s", got, controlplane.RoleOperator)
	}
}
