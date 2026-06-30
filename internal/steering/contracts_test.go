package steering

import "testing"

func TestSteeringSourcesAndSafetyContract(t *testing.T) {
	for _, source := range []Source{SourceCLI, SourceAPI, SourceTUI, SourceMCP} {
		if !IsKnownSource(source) {
			t.Fatalf("expected source %q to be known", source)
		}
	}

	if IsKnownSource(Source("repo_doc")) {
		t.Fatal("repo docs must not be accepted as steering sources")
	}

	if SafetyPolicyOverrideAllowed {
		t.Fatal("steering must not be able to override safety policy")
	}
}

func TestSteeringStoreInterfaceCompiles(t *testing.T) {
	var _ Store = fakeStore{}
}

type fakeStore struct{}

func (fakeStore) AppendSteeringMessage(Message) error { return nil }

func (fakeStore) SteeringContext(string, string, string, int) (Context, error) {
	return Context{SafetyPinned: true}, nil
}
