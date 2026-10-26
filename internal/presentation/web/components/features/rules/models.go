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
	// Disabled styr aktiv/inaktiv (backend disabled-flagga). Revision bär
	// CAS-stake för toggle utan att läsa om sidan.
	Disabled bool
	Revision int64
	// ControlGroup grupperar vad regeln styr (thing: match.type,
	// measurement: object/sensorType/device). Används för sortering så att
	// t.ex. alla container-regler hamnar tillsammans.
	ControlGroup string
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
	Status              string
	Rules               []RuleRowViewModel
	Notice              string
	ErrorMessage        string
	TransformConfigured bool
}
