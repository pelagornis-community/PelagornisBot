package workflow

import (
	"fmt"
	"time"
)

type Definition struct {
	Trigger    string      `json:"trigger"`
	Conditions []Condition `json:"conditions"`
	Actions    []Action    `json:"actions"`
}
type Condition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}
type Action struct {
	Type  string        `json:"type"`
	Delay time.Duration `json:"delay"`
}

func (d Definition) Validate() error {
	if d.Trigger == "" {
		return fmt.Errorf("trigger required")
	}
	if len(d.Actions) == 0 || len(d.Actions) > 10 {
		return fmt.Errorf("1-10 actions required")
	}
	for _, a := range d.Actions {
		switch a.Type {
		case "send_message", "add_role", "remove_role", "publish_panel", "create_ticket", "notify_staff", "save_faq_candidate", "award_reputation", "wait_for_approval", "delay":
		default:
			return fmt.Errorf("action %q not allowed", a.Type)
		}
	}
	return nil
}
func Matches(cs []Condition, e map[string]string) bool {
	for _, c := range cs {
		v := e[c.Field]
		if c.Op == "eq" && v != c.Value {
			return false
		}
		if c.Op == "contains" && !contains(v, c.Value) {
			return false
		}
	}
	return true
}
func contains(a, b string) bool {
	for i := 0; i+len(b) <= len(a); i++ {
		if a[i:i+len(b)] == b {
			return true
		}
	}
	return b == ""
}
