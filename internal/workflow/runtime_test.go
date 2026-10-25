package workflow

import "testing"

func TestDefinitionBlocksArbitraryExecution(t *testing.T) {
	d := Definition{Trigger: "rss.item.new", Actions: []Action{{Type: "exec"}}}
	if d.Validate() == nil {
		t.Fatal("arbitrary execution action allowed")
	}
}
