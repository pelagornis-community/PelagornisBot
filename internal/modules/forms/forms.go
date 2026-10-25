package forms

import (
	"fmt"
	"strings"
)

type Field struct {
	ID, Label, Type string
	Required        bool
	Min, Max        int
	Options         []string
}
type Schema struct {
	Title  string
	Fields []Field
}

func (s Schema) Validate() error {
	if len(s.Fields) == 0 || len(s.Fields) > 25 {
		return fmt.Errorf("forms require 1-25 fields")
	}
	for _, f := range s.Fields {
		if f.ID == "" || len(f.Label) > 45 {
			return fmt.Errorf("invalid field")
		}
		if f.Type != "short_text" && f.Type != "paragraph" && f.Type != "single_select" && f.Type != "multi_select" && f.Type != "checkbox" && f.Type != "radio" {
			return fmt.Errorf("unsupported field type %q", f.Type)
		}
		if (f.Type == "single_select" || f.Type == "multi_select" || f.Type == "radio") && (len(f.Options) == 0 || len(f.Options) > 25) {
			return fmt.Errorf("invalid options")
		}
	}
	return nil
}
func ValidateAnswers(s Schema, a map[string]string) error {
	for _, f := range s.Fields {
		v := strings.TrimSpace(a[f.ID])
		if f.Required && v == "" {
			return fmt.Errorf("%s is required", f.ID)
		}
		if f.Min > 0 && len(v) < f.Min {
			return fmt.Errorf("%s too short", f.ID)
		}
		if f.Max > 0 && len(v) > f.Max {
			return fmt.Errorf("%s too long", f.ID)
		}
	}
	return nil
}
