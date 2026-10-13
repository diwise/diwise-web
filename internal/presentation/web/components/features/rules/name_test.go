package rules

import (
	"bytes"
	"strings"
	"testing"

	apptransform "github.com/diwise/diwise-web/internal/application/transform"
)

func TestRuleListNamesAndIDFallback(t *testing.T) {
	name := strings.Repeat("Long free text ", 30) + "<&>"
	var out bytes.Buffer
	model := RulesPageViewModel{TransformConfigured: true, Rules: []RuleRowViewModel{
		{ID: "named-id", Name: name}, {ID: "full-fallback-id", ShortID: "short"},
	}}
	if err := RulesPage(stage5Localizer{}, model).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`title="` + strings.ReplaceAll(name, "<&>", "&lt;&amp;&gt;") + `"`, `truncate`, `>full-fallback-id</a>`, `href="/rules/named-id"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in list", want)
		}
	}
}

func TestRuleDetailHeadingNameAndFallback(t *testing.T) {
	for _, name := range []string{"", "Readable rule name"} {
		model := RuleFormViewModel{ID: "rule-id", TransformConfigured: true, Rule: apptransform.Rule{Name: name}}
		want := name
		if want == "" {
			want = model.ID
		}
		if model.Heading(stage5Localizer{}) != "rules_rule "+want {
			t.Fatal("heading lost identity")
		}
		var out bytes.Buffer
		if err := RuleFormPage(stage5Localizer{}, model).Render(t.Context(), &out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `name="name" value="`+name+`"`) {
			t.Fatal("name not prefilled")
		}
	}
}
