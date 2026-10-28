package commands

import "testing"

func TestDefinitionsUseDashboardsInsteadOfSlashOptions(t *testing.T) {
	for _, command := range Definitions() {
		if len(command.Options) != 0 {
			t.Fatalf("/%s still exposes %d slash-command option(s)", command.Name, len(command.Options))
		}
	}
}
