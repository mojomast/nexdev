package provider

import "fmt"

// Slot identifies a Nexdev stage provider slot. Slots are intentionally separate
// from pipeline stages because some stages use multiple provider roles.
type Slot string

const (
	SlotInterview         Slot = "interview"
	SlotComplexity        Slot = "complexity"
	SlotDesign            Slot = "design"
	SlotHivemindVoice     Slot = "hivemind_voice"
	SlotHivemindSynthesis Slot = "hivemind_synthesis"
	SlotValidate          Slot = "validate"
	SlotPlanSketch        Slot = "plan_sketch"
	SlotPlanDetail        Slot = "plan_detail"
	SlotReview            Slot = "review"
	SlotDevelop           Slot = "develop"
	SlotVerifyRepair      Slot = "verify_repair"
	SlotHandoff           Slot = "handoff"
)

// AllSlots is the SPEC 11.2 provider slot list in canonical order.
var AllSlots = []Slot{
	SlotInterview,
	SlotComplexity,
	SlotDesign,
	SlotHivemindVoice,
	SlotHivemindSynthesis,
	SlotValidate,
	SlotPlanSketch,
	SlotPlanDetail,
	SlotReview,
	SlotDevelop,
	SlotVerifyRepair,
	SlotHandoff,
}

// Selection is a configured provider/model pair. Empty slot selections inherit
// the primary selection.
type Selection struct {
	Provider string
	Model    string
}

// Empty reports whether a stage slot has no override configured.
func (s Selection) Empty() bool {
	return s.Provider == "" && s.Model == ""
}

// Route is the resolved provider selection for a slot.
type Route struct {
	Slot     Slot
	Provider string
	Model    string
}

// Router resolves Nexdev provider slots to registered provider names and models.
// It does not perform provider calls; call wrappers remain inside this package.
type Router struct {
	primary  Selection
	slots    map[Slot]Selection
	registry map[string]ProviderFactory
}

// NewRouter creates a router backed by the package provider registry.
func NewRouter(primary Selection, slots map[Slot]Selection) (*Router, error) {
	return NewRouterWithRegistry(primary, slots, Registry)
}

// NewRouterWithRegistry creates a router with an explicit registry for tests or
// app wiring. Provider factories are validated by name but not instantiated.
func NewRouterWithRegistry(primary Selection, slots map[Slot]Selection, registry map[string]ProviderFactory) (*Router, error) {
	r := &Router{
		primary:  primary,
		slots:    make(map[Slot]Selection, len(slots)),
		registry: registry,
	}

	if primary.Provider == "" {
		return nil, fmt.Errorf("primary provider cannot be empty")
	}
	if !r.providerKnown(primary.Provider) {
		return nil, fmt.Errorf("unknown provider %q", primary.Provider)
	}

	for slot, selection := range slots {
		if !IsKnownSlot(slot) {
			return nil, fmt.Errorf("unknown provider slot %q", slot)
		}
		if selection.Empty() {
			r.slots[slot] = selection
			continue
		}
		if selection.Provider != "" && !r.providerKnown(selection.Provider) {
			return nil, fmt.Errorf("unknown provider %q for slot %q", selection.Provider, slot)
		}
		r.slots[slot] = selection
	}

	return r, nil
}

// Resolve returns the provider/model selection for a slot, inheriting primary
// fields when the slot selection omits them.
func (r *Router) Resolve(slot Slot) (Route, error) {
	if r == nil {
		return Route{}, fmt.Errorf("provider router is nil")
	}
	if !IsKnownSlot(slot) {
		return Route{}, fmt.Errorf("unknown provider slot %q", slot)
	}

	selection := r.primary
	if slotSelection, ok := r.slots[slot]; ok && !slotSelection.Empty() {
		if slotSelection.Provider != "" {
			selection.Provider = slotSelection.Provider
		}
		if slotSelection.Model != "" {
			selection.Model = slotSelection.Model
		}
	}

	if !r.providerKnown(selection.Provider) {
		return Route{}, fmt.Errorf("unknown provider %q for slot %q", selection.Provider, slot)
	}

	return Route{Slot: slot, Provider: selection.Provider, Model: selection.Model}, nil
}

// IsKnownSlot reports whether slot is one of the SPEC 11.2 provider slots.
func IsKnownSlot(slot Slot) bool {
	for _, candidate := range AllSlots {
		if candidate == slot {
			return true
		}
	}
	return false
}

func (r *Router) providerKnown(name string) bool {
	if r.registry == nil {
		return false
	}
	_, ok := r.registry[name]
	return ok
}
