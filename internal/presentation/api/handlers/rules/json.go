package rules

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Reject unsupported fields rather than silently dropping them through the
// typed client. Backend validation remains authoritative for rule semantics.
func decodeRuleJSON(text string, dst any) error {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("rule JSON: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("rule JSON must contain exactly one document")
	}
	return nil
}
