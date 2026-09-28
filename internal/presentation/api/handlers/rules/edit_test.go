package rules

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func postForm(t *testing.T, handler http.HandlerFunc, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestRuleNewPageRendersBlankForm(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleNewPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/rules/new", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, `action="/rules/new"`))
	is.True(strings.Contains(body, "things.v1.values"))
	is.True(strings.Contains(body, "numToOnOff"))
	is.True(strings.Contains(body, "LanguageMap"))
}

func TestRuleCreateSavesAndRedirects(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	var created map[string][]string
	_ = created
	handler := NewRuleCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)

	form := url.Values{
		"tenant": {"t1"}, "kind": {"thing"}, "event": {"things.v1.values"}, "type": {"room"},
		"e0_name": {}, "e0_id": {"urn:ngsi-ld:Room:{{nameOrID}}"}, "e0_type": {"Room"},
		"e0_p0_target": {"name"}, "e0_p0_type": {"Text"},
		"e0_p0_sourceKind": {"field"}, "e0_p0_sourceValue": {"name"},
	}
	// Rensa tomma fält (url.Values med tom slice ≈ saknat fält).
	for k, v := range form {
		if len(v) == 0 {
			delete(form, k)
		}
	}
	rec := postForm(t, handler, "/rules/new", form)

	is.Equal(http.StatusFound, rec.Code)
	is.True(strings.HasPrefix(rec.Header().Get("Location"), "/rules/"))
}

func TestRuleCreateShowsValidationError(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)

	// Tom match.kind + tom entitet: backend... här formulärfel direkt?
	// kind tom + entitet utan id/typ: parse ok (entitet hoppas över) men
	// regeln blir tom — backend-stubben kräver entities, formuläret visar
	// felet. Använd ogiltig tenant i stället: stubben kräver inget, så
	// bygg en regel utan entiteter via tomma block.
	form := url.Values{"tenant": {"t1"}, "kind": {"thing"}}
	rec := postForm(t, handler, "/rules/new", form)

	// Stub-backend: tom entities-lista ger 400 {error} → formulärfel.
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "rule must declare"))
}

func TestParseRuleFormBuildsRule(t *testing.T) {
	is := is.New(t)

	form := map[string][]string{
		"tenant": {"t1"}, "kind": {"thing"}, "event": {"things.v1.values"}, "type": {"room"},
		"relationRemoved":     {"true"},
		"e0_id":               {"urn:ngsi-ld:Room:{{nameOrID}}"},
		"e0_type":             {"Room"},
		"e0_create":           {"true"},
		"e0_removeAttributes": {"refParent\nlocation"},
		"e0_p0_target":        {"status"},
		"e0_p0_type":          {"Text"},
		"e0_p0_sourceKind":    {"field"},
		"e0_p0_sourceValue":   {"values.presence.value"},
		"e0_p0_transformOp":   {"numToOnOff"},
		"e0_p0_required":      {"true"},
		"e0_p1_target":        {"name"},
		"e0_p1_type":          {"LanguageMap"},
		"e0_p1_sourceKind":    {"const"},
		"e0_p1_sourceValue":   {`{"sv":"Hej"}`},
		"e1_id":               {},
	}
	// Rensa tomma fält.
	for k, v := range form {
		if len(v) == 1 && v[0] == "" {
			delete(form, k)
		}
		if len(v) == 0 {
			delete(form, k)
		}
	}

	rule, err := parseRuleForm(form)
	is.NoErr(err)
	is.Equal("thing", string(rule.Match.Kind))
	is.Equal("things.v1.values", rule.Match.Event)
	is.True(rule.Match.RelationRemoved)
	is.Equal(1, len(rule.Entities))
	is.Equal("urn:ngsi-ld:Room:{{nameOrID}}", rule.Entities[0].ID)
	is.True(rule.Entities[0].Create)
	is.Equal([]string{"refParent", "location"}, rule.Entities[0].RemoveAttributes)
	is.Equal(2, len(rule.Entities[0].Properties))

	status := rule.Entities[0].Properties[0]
	is.Equal("values.presence.value", status.Field)
	is.Equal("numToOnOff", status.Transform[0].Op)
	is.True(status.Required)

	name := rule.Entities[0].Properties[1]
	is.Equal("LanguageMap", string(name.Type))
	is.Equal(map[string]any{"sv": "Hej"}, name.Const)
}

func TestParseRuleFormSkipsEmptyBlocks(t *testing.T) {
	is := is.New(t)

	rule, err := parseRuleForm(map[string][]string{"kind": {"thing"}})
	is.NoErr(err)
	is.Equal(0, len(rule.Entities))

	_, err = parseRuleForm(map[string][]string{"e0_p0_type": {"Text"}})
	is.True(err != nil)
}

func TestParseRuleFormRejectsDuplicateFields(t *testing.T) {
	is := is.New(t)

	// Kolliderande blockindex (dubbelklick utan JS) ger fel, inte tyst
	// dataförlust.
	_, err := parseRuleForm(map[string][]string{
		"kind":  {"thing"},
		"e0_id": {"a", "b"},
	})
	is.True(err != nil)
}

func TestEntityBlankFragment(t *testing.T) {

	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewEntityBlankFragment(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/components/rules/entity-blank?index=2", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), `name="e2_id"`))
}

func TestPropertyBlankFragment(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewPropertyBlankFragment(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/components/rules/property-blank?entity=1&index=3", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), `name="e1_p3_target"`))
}

func ruleForm(id, revision string) url.Values {
	return url.Values{
		"tenant": {"t1"}, "kind": {"thing"}, "event": {"things.v1.values"}, "type": {"room"},
		"revision": {revision},
		"e0_id":    {"urn:ngsi-ld:Room:{{nameOrID}}"}, "e0_type": {"Room"},
		"e0_p0_target": {"name"}, "e0_p0_type": {"Text"},
		"e0_p0_sourceKind": {"field"}, "e0_p0_sourceValue": {"name"},
	}
}

func TestRuleSaveUpdatesAndRedirects(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleSavePage(context.Background(), testLocaleBundle(), testAssets(), app)
	id := "22222222-2222-2222-2222-222222222222"

	req := httptest.NewRequest(http.MethodPost, "/rules/"+id, strings.NewReader(ruleForm(id, "1").Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusFound, rec.Code)
	is.Equal("/rules/"+id+"?notice=updated", rec.Header().Get("Location"))
}

func TestRuleSaveConflictShowsFormError(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleSavePage(context.Background(), testLocaleBundle(), testAssets(), app)
	id := "22222222-2222-2222-2222-222222222222"

	// Fel revision (stubben har rev 1) -> 409 -> konfliktsida med formulär.
	req := httptest.NewRequest(http.MethodPost, "/rules/"+id, strings.NewReader(ruleForm(id, "9").Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "rules_conflict"))
}
