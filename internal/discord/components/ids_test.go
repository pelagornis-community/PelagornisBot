package components

import "testing"

func TestSignedID(t *testing.T) {
	raw := Sign("a secret", ID{"v1", "form", "g", "f", "u"})
	got, e := Parse("a secret", raw)
	if e != nil || got.Action != "form" {
		t.Fatal("signed ID failed")
	}
	if _, e = Parse("other", raw); e == nil {
		t.Fatal("forgery accepted")
	}
}
