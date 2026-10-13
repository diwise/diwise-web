package rules

import (
	"strconv"

	apptransform "github.com/diwise/diwise-web/internal/application/transform"
)

// ValidateResultViewModel är validate-badgen (HTMX-del).
type ValidateResultViewModel struct {
	OK      bool
	Message string
}

type CompatibilityViewModel struct {
	CandidateName string
	Result        apptransform.CompatibilityResult
	ErrorMessage  string
}

// PreviewResultViewModel är preview-svaret (HTMX-del).
type PreviewResultViewModel struct {
	CandidateName string
	Result        apptransform.PreviewResult
	Event         apptransform.PreviewEvent
	Queried       bool
	ErrorMessage  string
}

// ObservationsString är antal utvärderade observationer som text.
func (m PreviewResultViewModel) ObservationsString() string {
	return strconv.Itoa(m.Result.Observations)
}
