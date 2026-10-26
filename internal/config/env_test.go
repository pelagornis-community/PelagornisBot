package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvLoadsUnsetValuesWithoutOverridingEnvironment(t *testing.T) {
	t.Setenv("PELA_TEST_EXISTING", "from-environment")
	t.Setenv("PELA_TEST_NEW", "")
	if err := os.Unsetenv("PELA_TEST_NEW"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# comment\nPELA_TEST_EXISTING=from-file\nPELA_TEST_NEW=\"loaded value\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("PELA_TEST_EXISTING"); got != "from-environment" {
		t.Fatalf("existing environment was overwritten: %q", got)
	}
	if got := os.Getenv("PELA_TEST_NEW"); got != "loaded value" {
		t.Fatalf("new environment = %q", got)
	}
}
