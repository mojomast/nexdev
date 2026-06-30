package contract

import "testing"

func TestRequiredEventTypes(t *testing.T) {
	want := []string{
		"heartbeat",
		"run_started",
		"run_status",
		"stage_transition",
		"stage_status",
		"content_delta",
		"provider_call_started",
		"provider_call_completed",
		"provider_call_failed",
		"artifact_updated",
		"plan_updated",
		"review_required",
		"review_completed",
		"task_started",
		"task_progress",
		"task_completed",
		"task_error",
		"task_blocked",
		"task_paused",
		"task_resumed",
		"task_skipped",
		"steering_added",
		"detour_requested",
		"detour_created",
		"detour_failed",
		"blocker_created",
		"blocker_resolved",
		"verify_started",
		"verify_command_output",
		"verify_completed",
		"git_event",
		"cost_update",
		"security_warning",
		"pipeline_error",
		"done",
	}

	if EventContractVersion != "nexdev-event-v1" {
		t.Fatalf("event contract version = %q", EventContractVersion)
	}
	if len(RequiredEventTypes) != len(want) {
		t.Fatalf("event type count = %d, want %d", len(RequiredEventTypes), len(want))
	}
	seen := map[string]bool{}
	for _, got := range RequiredEventTypes {
		if seen[got] {
			t.Fatalf("duplicate event type %q", got)
		}
		seen[got] = true
	}
	for _, name := range want {
		if !seen[name] {
			t.Fatalf("missing event type %q", name)
		}
	}
}

func TestRequiredEventSources(t *testing.T) {
	want := []string{"core", "executor", "worker", "tui", "api", "mcp"}
	if len(RequiredEventSources) != len(want) {
		t.Fatalf("event source count = %d, want %d", len(RequiredEventSources), len(want))
	}
	seen := map[string]bool{}
	for _, got := range RequiredEventSources {
		seen[got] = true
	}
	for _, name := range want {
		if !seen[name] {
			t.Fatalf("missing event source %q", name)
		}
	}
}
