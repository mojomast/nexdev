package provider

import (
	"reflect"
	"testing"
)

func TestProviderSlotsMatchSpec(t *testing.T) {
	want := []Slot{
		Slot("interview"),
		Slot("complexity"),
		Slot("design"),
		Slot("hivemind_voice"),
		Slot("hivemind_synthesis"),
		Slot("validate"),
		Slot("plan_sketch"),
		Slot("plan_detail"),
		Slot("review"),
		Slot("develop"),
		Slot("verify_repair"),
		Slot("handoff"),
	}

	if !reflect.DeepEqual(AllSlots, want) {
		t.Fatalf("provider slots mismatch\ngot:  %#v\nwant: %#v", AllSlots, want)
	}

	for _, slot := range want {
		if !IsKnownSlot(slot) {
			t.Fatalf("slot %q was not recognized", slot)
		}
	}
}

func TestRouterInheritsPrimaryForEmptySlot(t *testing.T) {
	router, err := NewRouterWithRegistry(
		Selection{Provider: "primary", Model: "primary-model"},
		map[Slot]Selection{SlotDesign: {}},
		testProviderRegistry(),
	)
	if err != nil {
		t.Fatalf("NewRouterWithRegistry failed: %v", err)
	}

	route, err := router.Resolve(SlotDesign)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	want := Route{Slot: SlotDesign, Provider: "primary", Model: "primary-model"}
	if route != want {
		t.Fatalf("route mismatch\ngot:  %#v\nwant: %#v", route, want)
	}
}

func TestRouterUsesConfiguredSlotProvider(t *testing.T) {
	router, err := NewRouterWithRegistry(
		Selection{Provider: "primary", Model: "primary-model"},
		map[Slot]Selection{SlotDevelop: {Provider: "develop", Model: "develop-model"}},
		testProviderRegistry(),
	)
	if err != nil {
		t.Fatalf("NewRouterWithRegistry failed: %v", err)
	}

	route, err := router.Resolve(SlotDevelop)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	want := Route{Slot: SlotDevelop, Provider: "develop", Model: "develop-model"}
	if route != want {
		t.Fatalf("route mismatch\ngot:  %#v\nwant: %#v", route, want)
	}
}

func TestRouterSlotModelOverrideInheritsPrimaryProvider(t *testing.T) {
	router, err := NewRouterWithRegistry(
		Selection{Provider: "primary", Model: "primary-model"},
		map[Slot]Selection{SlotReview: {Model: "review-model"}},
		testProviderRegistry(),
	)
	if err != nil {
		t.Fatalf("NewRouterWithRegistry failed: %v", err)
	}

	route, err := router.Resolve(SlotReview)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	want := Route{Slot: SlotReview, Provider: "primary", Model: "review-model"}
	if route != want {
		t.Fatalf("route mismatch\ngot:  %#v\nwant: %#v", route, want)
	}
}

func TestRouterRejectsUnknownSlot(t *testing.T) {
	_, err := NewRouterWithRegistry(
		Selection{Provider: "primary", Model: "primary-model"},
		map[Slot]Selection{Slot("not_a_slot"): {Provider: "develop"}},
		testProviderRegistry(),
	)
	if err == nil {
		t.Fatal("expected unknown slot error")
	}

	router, err := NewRouterWithRegistry(
		Selection{Provider: "primary", Model: "primary-model"},
		nil,
		testProviderRegistry(),
	)
	if err != nil {
		t.Fatalf("NewRouterWithRegistry failed: %v", err)
	}

	if _, err := router.Resolve(Slot("not_a_slot")); err == nil {
		t.Fatal("expected unknown slot error from Resolve")
	}
}

func TestRouterRejectsUnknownProvider(t *testing.T) {
	if _, err := NewRouterWithRegistry(Selection{Provider: "missing"}, nil, testProviderRegistry()); err == nil {
		t.Fatal("expected unknown primary provider error")
	}

	if _, err := NewRouterWithRegistry(
		Selection{Provider: "primary"},
		map[Slot]Selection{SlotHandoff: {Provider: "missing"}},
		testProviderRegistry(),
	); err == nil {
		t.Fatal("expected unknown slot provider error")
	}
}

func TestRegistryStillCreatesKnownProviders(t *testing.T) {
	for _, name := range []string{"anthropic", "openai", "ollama"} {
		provider, err := CreateProvider(name)
		if err != nil {
			t.Fatalf("CreateProvider(%q) failed: %v", name, err)
		}
		if provider == nil {
			t.Fatalf("CreateProvider(%q) returned nil", name)
		}
	}
}

func testProviderRegistry() map[string]ProviderFactory {
	return map[string]ProviderFactory{
		"primary": func() Provider { return nil },
		"develop": func() Provider { return nil },
	}
}
