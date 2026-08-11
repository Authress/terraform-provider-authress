package authress

import "fmt"

// FieldMismatch describes a mismatch between planned and existing resource fields.
type FieldMismatch struct {
	Field    string
	Expected string
	Actual   string
}

func (m FieldMismatch) String() string {
	return fmt.Sprintf("field %q: expected %q, got %q", m.Field, m.Expected, m.Actual)
}

// compareField checks if expected matches actual for a named field.
// Returns a FieldMismatch if they differ, nil if they match.
func compareField(field, expected, actual string) *FieldMismatch {
	if expected != actual {
		return &FieldMismatch{Field: field, Expected: expected, Actual: actual}
	}
	return nil
}

// collectMismatches aggregates non-nil mismatches into a slice.
// Returns nil if all fields match (adoption is safe).
func collectMismatches(checks ...*FieldMismatch) []FieldMismatch {
	var mismatches []FieldMismatch
	for _, check := range checks {
		if check != nil {
			mismatches = append(mismatches, *check)
		}
	}
	if len(mismatches) == 0 {
		return nil
	}
	return mismatches
}

// formatMismatches builds a human-readable diagnostic message from mismatches.
func formatMismatches(resourceType, resourceId string, mismatches []FieldMismatch) string {
	msg := fmt.Sprintf("Resource %s %q already exists but differs from configuration:\n", resourceType, resourceId)
	for _, m := range mismatches {
		msg += fmt.Sprintf("  - %s\n", m.String())
	}
	msg += "\nTo adopt this resource, update your configuration to match the existing resource, or delete/recreate it."
	return msg
}
