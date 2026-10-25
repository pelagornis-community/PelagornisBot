package feeds

import "testing"

func TestKeyAndCanonicalURL(t *testing.T) {
	if Key("", "https://example.com/a?x=1#z", "") != "https://example.com/a" {
		t.Fatal("canonical dedupe key")
	}
	if Key("", "", "") != "" {
		t.Fatal("empty item must not produce a dedupe key")
	}
}
