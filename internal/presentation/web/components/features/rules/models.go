package rules

import (
	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	"strconv"
)

// RuleRowViewModel är en regelrad i listan.
type RuleRowViewModel struct {
	ID       string
	ShortID  string
	Kind     string
	Summary  string
	Tenant   string
	Source   string
	SeedKey  string
	Revision int64
}

// RulesPageViewModel är listmodellen för /rules.
type RulesPageViewModel struct {
	Tenants             []string
	Kind                string
	Event               string
	Tenant              string
	Source              string
	Search              string
	Rules               []RuleRowViewModel
	Notice              string
	ErrorMessage        string
	TransformConfigured bool
}

// RuleDetailsViewModel är detalj/redigera-sidan för /rules/{id}.
type RuleDetailsViewModel struct {
	Model               apptransform.Model
	IsSeed              bool
	Tenants             []string
	ConfirmDelete       bool
	ErrorMessage        string
	TransformConfigured bool
}

// RevisionString är revisionen som dolt formulärfält (If-Match rev-n).
func (m RuleDetailsViewModel) RevisionString() string {
	return strconv.FormatInt(m.Model.Revision, 10)
}
