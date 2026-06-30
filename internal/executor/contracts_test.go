package executor

import (
	"context"
	"testing"

	"github.com/mojomast/nexdev/internal/contract"
	"github.com/mojomast/nexdev/internal/pipeline"
	"github.com/mojomast/nexdev/internal/steering"
)

func TestTaskUpdateEventMapping(t *testing.T) {
	expected := map[UpdateType]string{
		TaskStarted:   contract.EventTypeTaskStarted,
		TaskProgress:  contract.EventTypeTaskProgress,
		TaskCompleted: contract.EventTypeTaskCompleted,
		TaskError:     contract.EventTypeTaskError,
		TaskBlocked:   contract.EventTypeTaskBlocked,
		TaskPaused:    contract.EventTypeTaskPaused,
		TaskResumed:   contract.EventTypeTaskResumed,
		TaskSkipped:   contract.EventTypeTaskSkipped,
	}

	if len(TaskUpdateEventMapping) != len(expected) {
		t.Fatalf("expected %d task update mappings, got %d", len(expected), len(TaskUpdateEventMapping))
	}

	for updateType, eventType := range expected {
		mapping, ok := EventMappingForTaskUpdate(updateType)
		if !ok {
			t.Fatalf("missing mapping for %q", updateType)
		}
		if mapping.EventType != eventType {
			t.Fatalf("mapping for %q = %q, want %q", updateType, mapping.EventType, eventType)
		}
		if mapping.Source != contract.EventSourceExecutor {
			t.Fatalf("mapping for %q source = %q", updateType, mapping.Source)
		}
		if mapping.Stage != pipeline.StageDevelop {
			t.Fatalf("mapping for %q stage = %q", updateType, mapping.Stage)
		}
	}
}

func TestExecutorControlInterfaceCompiles(t *testing.T) {
	var _ Control = fakeControl{}
}

type fakeControl struct{}

func (fakeControl) CurrentTask(context.Context) (*CurrentTaskSnapshot, error) { return nil, nil }
func (fakeControl) Pause(context.Context, string) error                       { return nil }
func (fakeControl) Resume(context.Context) error                              { return nil }
func (fakeControl) Cancel(context.Context, string) error                      { return nil }
func (fakeControl) SkipTask(context.Context, string, string) error            { return nil }
func (fakeControl) SetSteeringContext(context.Context, string, steering.Message) error {
	return nil
}
