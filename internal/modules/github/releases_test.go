package github

import "testing"

func TestRejectsInvalidRepo(t *testing.T) {
	if validRepo("bad") || validRepo("a/../b") {
		t.Fatal("repository allowlist validation failed")
	}
}
