package workflow

import "testing"

func TestWorkflowLimits(t *testing.T) {
	if (Definition{Trigger: "join", Actions: []Action{{Type: "shell"}}}).Validate() == nil {
		t.Fatal("unsafe action allowed")
	}
	if !Matches([]Condition{{Field: "role", Op: "eq", Value: "member"}}, map[string]string{"role": "member"}) {
		t.Fatal("condition")
	}
}
