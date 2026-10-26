package rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	featurerules "github.com/diwise/diwise-web/internal/presentation/web/components/features/rules"
	"github.com/matryer/is"
)

func TestRuleControlGroupGroupsByType(t *testing.T) {
	is := is.New(t)
	thing := apptransform.Rule{Match: apptransform.Match{Kind: "thing", Type: "container"}}
	is.Equal("container", ruleControlGroup(thing))
	meas := apptransform.Rule{Match: apptransform.Match{Kind: "measurement", Object: "urn:oma:lwm2m:ext:3303"}}
	is.Equal("urn:oma:lwm2m:ext:3303", ruleControlGroup(meas))
}

func TestSortRuleRowsActiveFirstGrouped(t *testing.T) {
	is := is.New(t)
	rows := []featurerules.RuleRowViewModel{
		{ID: "c-inactive", Disabled: true, ControlGroup: "container", Name: "c"},
		{ID: "b-active", Disabled: false, ControlGroup: "room", Name: "b"},
		{ID: "a-active", Disabled: false, ControlGroup: "container", Name: "a"},
	}
	sortRuleRows(rows)
	is.Equal("a-active", rows[0].ID)
	is.Equal("b-active", rows[1].ID)
	is.Equal("c-inactive", rows[2].ID)
}

func TestParseRuleFormDisabled(t *testing.T) {
	is := is.New(t)
	rule, err := parseRuleForm(map[string][]string{
		"kind": {"thing"}, "event": {"things.v1.values"}, "type": {"room"}, "tenant": {"t1"},
		"disabled": {"true"},
		"e0_id":    {"urn:ngsi-ld:Room:{{id}}"}, "e0_type": {"Room"},
		"e0_p0_target": {"name"}, "e0_p0_type": {"Text"},
		"e0_p0_sourceKind": {"field"}, "e0_p0_sourceValue": {"name"},
	})
	is.NoErr(err)
	is.True(rule.Disabled)

	rule, err = parseRuleForm(map[string][]string{
		"kind": {"thing"}, "event": {"things.v1.values"}, "type": {"room"}, "tenant": {"t1"},
		"e0_id": {"urn:ngsi-ld:Room:{{id}}"}, "e0_type": {"Room"},
		"e0_p0_target": {"name"}, "e0_p0_type": {"Text"},
		"e0_p0_sourceKind": {"field"}, "e0_p0_sourceValue": {"name"},
	})
	is.NoErr(err)
	is.True(!rule.Disabled)
}

func TestRuleToggleFlipsDisabled(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()
	handler := NewRuleTogglePage(context.Background(), testLocaleBundle(), testAssets(), app)
	id := "22222222-2222-2222-2222-222222222222"

	req := httptest.NewRequest(http.MethodPost, "/rules/"+id+"/toggle", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusFound, rec.Code)
	is.Equal("/rules?notice=updated", rec.Header().Get("Location"))
}
