package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	featurerules "github.com/diwise/diwise-web/internal/presentation/web/components/features/rules"
	"golang.org/x/net/html"
)

// Serialize actual generated HTML successful controls, rather than a parallel
// handcrafted form shape that could conceal a missing templ field.
func renderedForm(t *testing.T, model featurerules.RuleFormViewModel) url.Values {
	t.Helper()
	var b bytes.Buffer
	if err := featurerules.RuleFormPage(testLocaleBundle().For("en"), model).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	root, err := html.Parse(&b)
	if err != nil {
		t.Fatal(err)
	}
	attr := func(n *html.Node, key string) (string, bool) {
		for _, a := range n.Attr {
			if a.Key == key {
				return a.Val, true
			}
		}
		return "", false
	}
	var text func(*html.Node) string
	text = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var s string
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			s += text(c)
		}
		return s
	}
	values := url.Values{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		name, _ := attr(n, "name")
		if name != "" {
			value, _ := attr(n, "value")
			switch n.Data {
			case "input":
				typ, _ := attr(n, "type")
				_, checked := attr(n, "checked")
				if typ != "checkbox" || checked {
					values.Add(name, value)
				}
			case "textarea":
				values.Add(name, text(n))
			case "select":
				var first, selected string
				found := false
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Data == "option" {
						v, _ := attr(c, "value")
						if !found {
							first = v
							found = true
						}
						if _, ok := attr(c, "selected"); ok {
							selected = v
							first = v
							break
						}
					}
				}
				if selected != "" {
					first = selected
				}
				values.Add(name, first)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return values
}

func TestPLAN1050RenderedFormAndJSONRoundtrip(t *testing.T) {
	var rule apptransform.Rule
	const raw = `{"priority":7,"match":{"kind":"thing","tenant":"t1","type":"neutral","thingTypeVersion":"2","variantId":"v","variantVersion":"3","event":"things.v1.relations","relation":"feeds","toType":"target","relationRemoved":true,"lifecycle":"deleted"},"entities":[{"name":"main","id":"urn:example:{{id}}","type":"Neutral","context":"https://example.org/context","create":true,"delete":true,"removeAttributes":["old"],"properties":[{"target":"reading","type":"Number","value":"snapshot.values[\"a.b\"].value","when":"present","required":true,"unit":"KEL","convert":{"from":"Cel","to":"KEL","offset":273.15},"transform":[{"op":"multiply","value":2},{"op":"round"}]},{"target":"label","type":"Text","const":"false"},{"target":"total","type":"Number","derive":{"fn":"sum","of":[{"value":"snapshot.values.a.value"},{"value":"snapshot.values.b.value"}]}},{"target":"links","type":"Relationship","array":true,"ref":"main","prefix":"urn:example:","subproperties":[{"target":"name","type":"Text","field":"name"}]}]},{"name":"inverse","id":"urn:example:inverse:{{id}}","type":"Other","properties":[]}]}`
	if err := decodeRuleJSON(raw, &rule); err != nil {
		t.Fatal(err)
	}
	rule.Name = `A free rule name <&> — not an entity role`
	form := renderedForm(t, featurerules.RuleFormViewModel{IsNew: true, TransformConfigured: true, Tenants: []string{"t1"}, Rule: rule})
	back, err := parseRuleForm(form)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(rule)
	got, _ := json.Marshal(back)
	if !bytes.Equal(want, got) {
		t.Fatalf("form lost fields\nwant %s\n got %s", want, got)
	}
	// A server validation error/conflict renders the same candidate with the
	// submitted controls attached. It must still roundtrip before JS runs.
	recovered := renderedForm(t, featurerules.RuleFormViewModel{IsNew: true, TransformConfigured: true, Tenants: []string{"t1"}, Rule: back, SubmittedForm: form})
	recoveredRule, err := parseRuleForm(recovered)
	if err != nil {
		t.Fatal(err)
	}
	recoveredJSON, _ := json.Marshal(recoveredRule)
	if !bytes.Equal(want, recoveredJSON) {
		t.Fatalf("error/conflict form lost fields: %s", recoveredJSON)
	}
	form.Set("editorMode", "json")
	back, err = parseRuleForm(form)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = json.Marshal(back)
	if !bytes.Equal(want, got) {
		t.Fatalf("JSON lost fields: %s", got)
	}
	form.Set("ruleJSON", strings.Replace(raw, `"priority":7`, `"newUnsupportedField":true`, 1))
	if _, err := parseRuleForm(form); err == nil {
		t.Fatal("unsupported field silently lost")
	}
	form.Set("ruleJSON", raw+` {}`)
	if _, err := parseRuleForm(form); err == nil {
		t.Fatal("multiple documents accepted")
	}
}

func TestPLAN1050ActualPreviewRequestAndResponse(t *testing.T) {
	rawEvent := `{"tenant":"t1","thingId":"n1","delta":[{"propertyId":"reading","value":0,"quality":"ok"}],"snapshot":[{"propertyId":"reading","value":12,"quality":"ok"}],"configRevision":9007199254740993,"extra":{"keep":false}}`
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/preview" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("bad request: %s", r.URL)
		}
		received, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"matched":true,"observations":1,"ruleId":"chosen","ruleRevision":4,"snapshotVersion":9,"matches":[{"ruleId":"other","reason":"lower priority"}],"skipped":[{"entity":"inverse","target":"links","reason":"missing target role"}],"entities":[{"id":"urn:example:n1","type":"Neutral","operation":"remove","removeAttributes":["old"],"effectiveContext":"https://example.org/context","contextSource":"deployment","properties":{"links":{"type":"Relationship","object":"urn:example:n2"}}}]}`))
	}))
	defer server.Close()
	app := &testRulesApp{transforms: apptransform.NewService(client.NewClient("", "", "", "", "", ""), server.URL)}
	rec := postFragment(t, NewPreviewFragment(context.Background(), testLocaleBundle(), testAssets(), app), previewForm(url.Values{"previewMessage": {rawEvent}, "name": {"Preview candidate name"}}))
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	var request struct {
		Rule  apptransform.Rule `json:"rule"`
		Event struct {
			Message json.RawMessage `json:"message"`
		} `json:"event"`
	}
	if err := json.Unmarshal(received, &request); err != nil {
		t.Fatal(err)
	}
	if string(request.Event.Message) != rawEvent {
		t.Fatalf("preview JSON changed: %s", request.Event.Message)
	}
	if request.Rule.Name != "Preview candidate name" || !strings.Contains(rec.Body.String(), "Inline rule: Preview candidate name") {
		t.Fatalf("preview lost candidate name: %s / %s", received, rec.Body.String())
	}
	for _, s := range []string{"chosen", "ruleRevision", "snapshotVersion", "lower priority", "missing target role", "https://example.org/context", "deployment", "old", "Relationship", "9007199254740993"} {
		if !strings.Contains(rec.Body.String(), s) {
			t.Errorf("missing preview detail %q", s)
		}
	}
}

func TestPLAN1050AdvancedRowsNeverDisappearOnParseError(t *testing.T) {
	form := previewForm(url.Values{
		"e0_p0_target": {""}, "e0_p0_convert": {`{"from":"Cel","to":"KEL","offset":273.15}`},
		"e7_name": {"later-role"}, "e7_id": {"urn:example:later"}, "e7_type": {"Later"},
		"e7_p9_target": {"total"}, "e7_p9_type": {"Number"}, "e7_p9_sourceKind": {"derive"},
		"e7_p9_derive": {`{"fn":"sum","of":[{"value":"snapshot.a.value"}]}`},
	})
	rule, err := parseRuleForm(form)
	if err == nil || !strings.Contains(err.Error(), "target is required") {
		t.Fatalf("advanced input discarded: %v", err)
	}
	model := featurerules.RuleFormViewModel{Rule: rule, SubmittedForm: form, TransformConfigured: true}
	rows := model.EntityRows()
	if len(rows) != 2 || rows[1].Index != 7 {
		t.Fatalf("lost sparse entity rows: %+v", rows)
	}
	props := model.PropertyRows(7, nil)
	if len(props) != 1 || props[0].Index != 9 {
		t.Fatalf("lost sparse property rows: %+v", props)
	}
	var b bytes.Buffer
	if err := featurerules.RuleFormPage(testLocaleBundle().For("en"), model).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`name="e0_p0_convert"`, `name="e7_name"`, `name="e7_p9_derive"`, `data-submitted=`, "later-role"} {
		if !strings.Contains(b.String(), field) {
			t.Errorf("unrecoverable field %s", field)
		}
	}
}

func TestPLAN1050BackendAuthorizationErrors(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			app := &testRulesApp{transforms: apptransform.NewService(client.NewClient("", "", "", "", "", ""), server.URL)}
			for name, handler := range map[string]http.HandlerFunc{
				"validate": NewValidateFragment(context.Background(), testLocaleBundle(), testAssets(), app),
				"preview":  NewPreviewFragment(context.Background(), testLocaleBundle(), testAssets(), app),
			} {
				rec := postFragment(t, handler, previewForm(nil))
				if rec.Code != status {
					t.Errorf("%s: got %d want %d", name, rec.Code, status)
				}
			}
			form := previewForm(url.Values{"checkCompatibility": {"true"}, "thingTypeVersion": {"2"}})
			rec := httptest.NewRecorder()
			NewValidateFragment(context.Background(), testLocaleBundle(), testAssets(), app)(rec, authorizedRequest(form))
			if rec.Code != status {
				t.Errorf("compatibility: got %d want %d", rec.Code, status)
			}
		})
	}
}

func TestPLAN1050ValuesFixtureHasIndependentCurrent(t *testing.T) {
	f, ok := featurerules.FixtureByName("values")
	if !ok {
		t.Fatal("missing values fixture")
	}
	var event struct {
		Delta    []json.RawMessage `json:"delta"`
		Values   []json.RawMessage `json:"values"`
		Snapshot []struct {
			PropertyID string `json:"propertyId"`
			ObservedAt string `json:"observedAt"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal([]byte(f.Message), &event); err != nil {
		t.Fatal(err)
	}
	if len(event.Delta) != 1 || len(event.Snapshot) != 2 || !reflect.DeepEqual(event.Delta, event.Values) {
		t.Fatal("fixture does not distinguish delta from current snapshot")
	}
	if event.Snapshot[0].ObservedAt == event.Snapshot[1].ObservedAt {
		t.Fatal("snapshot lost per-value times")
	}
}

func authorizedRequest(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/components/rules/validate", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r = r.WithContext(auth.WithToken(r.Context(), "test-token"))
	return r.WithContext(auth.WithAccess(r.Context(), map[string]map[auth.Scope]struct{}{"t1": {auth.AnyScope: {}}}))
}

func TestPLAN1050CompatibilityAndTenantBoundary(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/models/compat" || r.URL.Query().Get("tenant") != "t1" {
			t.Errorf("bad compat route: %s", r.URL)
		}
		var req apptransform.CompatibilityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.TemplateID != "room" || req.TemplateVersion != "2" || req.VariantID != "v" || len(req.Rules) != 1 || req.Rules[0].Name != "Compatibility candidate" {
			t.Errorf("bad compat request: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"compatible":false,"issues":[{"targetRole":"main","attribute":"reading","expression":"value","message":"unknown property","severity":"error"}],"warnings":[{"message":"dynamic metadata check","severity":"warning"}]}`))
	}))
	defer server.Close()
	app := &testRulesApp{transforms: apptransform.NewService(client.NewClient("", "", "", "", "", ""), server.URL)}
	handler := NewValidateFragment(context.Background(), testLocaleBundle(), testAssets(), app)
	form := previewForm(url.Values{"checkCompatibility": {"true"}, "thingTypeVersion": {"2"}, "variantId": {"v"}, "name": {"Compatibility candidate"}})
	rec := httptest.NewRecorder()
	handler(rec, authorizedRequest(form))
	for _, s := range []string{"Candidate rule: Compatibility candidate", "Errors / compatibility issues", "Warnings / dynamic limitations", "unknown property", "dynamic metadata check", "main", "reading"} {
		if !strings.Contains(rec.Body.String(), s) {
			t.Errorf("missing %q", s)
		}
	}
	form.Set("tenant", "foreign")
	rec = httptest.NewRecorder()
	handler(rec, authorizedRequest(form))
	if rec.Code != http.StatusForbidden || calls != 1 {
		t.Fatalf("tenant boundary: status=%d calls=%d", rec.Code, calls)
	}
	request := httptest.NewRequest(http.MethodGet, "/components/rules/type-options?tenant=foreign", nil)
	request = request.WithContext(authorizedRequest(form).Context())
	rec = httptest.NewRecorder()
	NewTypeOptionsFragment(context.Background(), testLocaleBundle(), testAssets(), app)(rec, request)
	if rec.Code != http.StatusForbidden || calls != 1 {
		t.Fatal("foreign catalog lookup allowed")
	}
}

func TestPLAN1050ConflictPreservesJSONAndPreviewInput(t *testing.T) {
	app, done := testApp(t)
	defer done()
	rule := testRule()
	rule.Name = "User name preserved through conflict"
	rule.Match.ThingTypeVersion = "2"
	rule.Entities[0].Name = "edited-role"
	raw, _ := json.Marshal(rule)
	form := previewForm(url.Values{"editorMode": {"json"}, "ruleJSON": {string(raw)}, "revision": {"0"}, "previewMessage": {`{"keep":"edited-event"}`}})
	req := authorizedRequest(form)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "22222222-2222-2222-2222-222222222222")
	rec := httptest.NewRecorder()
	NewRuleSavePage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, req)
	if !strings.Contains(rec.Body.String(), "rules_conflict") {
		t.Fatal("missing conflict")
	}
	back := renderedForm(t, featurerules.RuleFormViewModel{Rule: rule, TransformConfigured: true, SubmittedForm: form})
	if back.Get("ruleJSON") != string(raw) || back.Get("previewMessage") != form.Get("previewMessage") {
		t.Fatal("submitted input lost")
	}
	parsed, err := parseRuleForm(back)
	if err != nil || !reflect.DeepEqual(parsed, rule) {
		t.Fatalf("conflict rule changed: %+v %v", parsed, err)
	}
	if !strings.Contains(rec.Body.String(), "edited-role") || !strings.Contains(rec.Body.String(), "edited-event") || !strings.Contains(rec.Body.String(), rule.Name) {
		t.Fatal("conflict response dropped edits")
	}
}

func TestPLAN1050JSONImportExportAndInvalidInput(t *testing.T) {
	app, done := testApp(t)
	defer done()
	handler := NewValidateFragment(context.Background(), testLocaleBundle(), testAssets(), app)
	form := previewForm(url.Values{"editorAction": {"export"}, "thingTypeVersion": {"2"}, "e0_name": {"my-role"}, "name": {"Exported rule name"}})
	rec := postFragment(t, handler, form)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "my-role") || !strings.Contains(rec.Body.String(), "thingTypeVersion") || !strings.Contains(rec.Body.String(), "Exported rule name") {
		t.Fatal("export lost fields")
	}
	rule := testRule()
	rule.Name = "Imported rule name"
	rule.Entities[0].Name = "imported-role"
	raw, _ := json.Marshal(rule)
	form.Set("editorAction", "import")
	form.Set("editorMode", "json")
	form.Set("ruleJSON", string(raw))
	form.Set("ruleID", "existing-id")
	form.Set("revision", "3")
	rec = postFragment(t, handler, form)
	for _, s := range []string{`id="rule-editor"`, `action="/rules/existing-id"`, `name="revision" value="3"`, `name="e0_name" value="imported-role"`, `name="name" value="Imported rule name"`} {
		if !strings.Contains(rec.Body.String(), s) {
			t.Errorf("import missing %s", s)
		}
	}
	form.Set("ruleJSON", `{"match":`)
	rec = postFragment(t, handler, form)
	if rec.Header().Get("HX-Retarget") != "#validate-result" {
		t.Fatal("invalid JSON would replace and lose editor")
	}
}
