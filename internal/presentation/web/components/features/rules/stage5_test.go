package rules

import (
	"bytes"
	"strings"
	"testing"

	apptransform "github.com/diwise/diwise-web/internal/application/transform"
)

type stage5Localizer struct{}

func (stage5Localizer) Get(key string) string                           { return key }
func (stage5Localizer) GetWithData(key string, _ map[string]any) string { return key }

func TestStage5RenderedFormIncludesRuleFields(t *testing.T) {
	model := RuleFormViewModel{TransformConfigured: true, IsNew: true, Rule: apptransform.Rule{Priority: 7, Match: apptransform.Match{ToType: "target"}, Entities: []apptransform.Entity{{Properties: []apptransform.Property{{Target: "reading", Type: "Number", Convert: &apptransform.Conversion{From: "Cel", To: "KEL", Offset: 273.15}}}}}}}
	var out bytes.Buffer
	if err := RuleFormPage(stage5Localizer{}, model).Render(t.Context(), &out); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`name="priority"`, `value="7"`, `name="toType"`, `value="target"`, `name="e0_p0_convert"`, `273.15`} {
		if !strings.Contains(out.String(), field) {
			t.Fatalf("rendered form lost %s", field)
		}
	}
}
