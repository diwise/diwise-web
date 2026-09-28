package rules

// RuleRowViewModel är en regelrad i listan.
type RuleRowViewModel struct {
	ID      string
	ShortID string
	Kind    string
	Summary string
	Tenant  string
	Source  string
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
