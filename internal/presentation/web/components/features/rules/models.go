package rules

// RuleRowViewModel är en regelrad i listan.
type RuleRowViewModel struct {
	ID      string
	Name    string
	ShortID string
	Kind    string
	Summary string
	Tenant  string
	Source  string
}

// DisplayName keeps the full identity; visual truncation belongs to CSS.
func (m RuleRowViewModel) DisplayName() string {
	return ruleDisplayName(m.Name, m.ID)
}

func ruleDisplayName(name, id string) string {
	if name != "" {
		return name
	}
	return id
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
