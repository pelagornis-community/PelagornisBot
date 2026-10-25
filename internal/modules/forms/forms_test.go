package forms

import "testing"

func TestRequiredAnswer(t *testing.T) {
	s := Schema{Fields: []Field{{ID: "name", Label: "Name", Type: "short_text", Required: true}}}
	if e := ValidateAnswers(s, map[string]string{}); e == nil {
		t.Fatal("required field accepted")
	}
}
