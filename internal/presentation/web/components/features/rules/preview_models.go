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

// PreviewResultViewModel är preview-svaret (HTMX-del).
type PreviewResultViewModel struct {
	Result       apptransform.PreviewResult
	Queried      bool
	ErrorMessage string
}

// ObservationsString är antal utvärderade observationer som text.
func (m PreviewResultViewModel) ObservationsString() string {
	return strconv.Itoa(m.Result.Observations)
}
