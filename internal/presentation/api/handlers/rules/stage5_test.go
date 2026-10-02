package rules

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestStage5RuleFormPreservesPriorityTargetAndConversion(t *testing.T) {
	form := map[string][]string{
		"kind": {"thing"}, "event": {"things.v1.relations"}, "type": {"source"}, "toType": {"target"}, "priority": {"7"},
		"e0_id": {"urn:example:{{id}}"}, "e0_type": {"Generic"}, "e0_p0_target": {"reading"}, "e0_p0_type": {"Number"},
		"e0_p0_sourceKind": {"const"}, "e0_p0_sourceValue": {"12"}, "e0_p0_unit": {"KEL"},
		"e0_p0_convert": {`{"from":"Cel","to":"KEL","offset":273.15}`},
	}
	rule, err := parseRuleForm(form)
	if err != nil {
		t.Fatal(err)
	}
	if rule.Priority != 7 || rule.Match.ToType != "target" || rule.Entities[0].Properties[0].Convert == nil || rule.Entities[0].Properties[0].Convert.Offset != 273.15 {
		t.Fatalf("form lost stage5 fields: %+v", rule)
	}
	raw, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"priority":7`, `"toType":"target"`, `"convert":`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("JSON lost %s: %s", key, raw)
		}
	}
	back := rule
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, rule) {
		t.Fatal("JSON round trip changed rule")
	}
	form["priority"] = []string{"not-an-integer"}
	if _, err := parseRuleForm(form); err == nil {
		t.Fatal("malformed priority accepted")
	}
	form["priority"] = []string{"-2"}
	form["e0_p0_convert"] = []string{`{"from":"Cel","to":"KEL","typo":1}`}
	if _, err := parseRuleForm(form); err == nil {
		t.Fatal("unknown conversion field accepted")
	}
	form["e0_p0_convert"] = []string{`{"from":"Cel","to":"KEL"} {}`}
	if _, err := parseRuleForm(form); err == nil {
		t.Fatal("multiple conversion documents accepted")
	}
}
