package mcp

import "testing"

func TestLegacyStdioRegistrationIsDisabledForNexdevM11(t *testing.T) {
	tools := NewToolRegistry()
	resources := NewResourceRegistry()

	if err := NewSimpleToolHandlers(nil, t.TempDir()).RegisterBasicTools(tools); err != nil {
		t.Fatal(err)
	}
	if err := NewInterviewHandlers(nil).RegisterHandlers(tools); err != nil {
		t.Fatal(err)
	}
	if err := NewDesignHandlers(nil).RegisterHandlers(tools); err != nil {
		t.Fatal(err)
	}
	if err := NewPlanHandlers(nil).RegisterHandlers(tools); err != nil {
		t.Fatal(err)
	}
	if err := NewExecHandlers(nil).RegisterHandlers(tools); err != nil {
		t.Fatal(err)
	}
	if got := tools.ListTools(); len(got) != 0 {
		t.Fatalf("legacy stdio tools registered = %#v, want none", got)
	}

	if err := NewResourceHandlers(nil, t.TempDir()).RegisterAllResources(resources); err != nil {
		t.Fatal(err)
	}
	if got := resources.ListResources(); len(got) != 0 {
		t.Fatalf("legacy stdio resources registered = %#v, want none", got)
	}
}
