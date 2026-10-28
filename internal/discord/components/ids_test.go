package components

import "testing"

func TestSignedID(t *testing.T) {
	raw := Sign("a secret", ID{"v1", "form_start", "123456789012345678", "00112233445566778899aabbccddeeff", "123456789012345679"})
	got, e := Parse("a secret", raw)
	if e != nil || got.Action != "form_start" || len(raw) > 100 {
		t.Fatal("signed ID failed")
	}
	if _, e = Parse("other", raw); e == nil {
		t.Fatal("forgery accepted")
	}
}

func TestSetupProjectCategoryID(t *testing.T) {
	raw := Sign("a secret", ID{"v1", "setup_project_category_set", "123456789012345678", "", ""})
	got, err := Parse("a secret", raw)
	if err != nil || got.Action != "setup_project_category_set" || got.Guild != "123456789012345678" {
		t.Fatal("project setup component ID failed")
	}
}
