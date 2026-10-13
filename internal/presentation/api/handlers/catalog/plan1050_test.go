package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	appcatalog "github.com/diwise/diwise-web/internal/application/catalog"
	"github.com/diwise/diwise-web/internal/application/client"
	appthings "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"golang.org/x/net/html"
)

// Successful native controls, not a handwritten substitute for the form.
func serializedForm(t *testing.T, body string) url.Values {
	t.Helper()
	root, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	values := url.Values{}
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
		s := ""
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			s += text(c)
		}
		return s
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		name, _ := attr(n, "name")
		_, disabled := attr(n, "disabled")
		if name != "" && !disabled {
			switch n.Data {
			case "textarea":
				values.Add(name, text(n))
			case "input":
				kind, _ := attr(n, "type")
				_, checked := attr(n, "checked")
				if kind != "submit" && kind != "button" && (kind != "checkbox" && kind != "radio" || checked) {
					v, _ := attr(n, "value")
					values.Add(name, v)
				}
			case "select":
				_, multi := attr(n, "multiple")
				first := ""
				count := 0
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Data != "option" {
						continue
					}
					v, _ := attr(c, "value")
					if first == "" {
						first = v
					}
					if _, selected := attr(c, "selected"); selected {
						values.Add(name, v)
						count++
					}
				}
				if !multi && count == 0 {
					values.Add(name, first)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return values
}

// Wire field names verified against B's catalog.TemplateSpec/things.Template.
const completeTemplate = `{"template":{"id":"neutral","version":"v1","displayName":"N <&>","description":"Beskrivning","category":"test","primaryProperty":"a.b","required":["a.b"],"optional":["temperature"],"repeatGroups":["zones"],"allowedQuantities":["temperature"],"labels":["custom"],"allowedGeometries":["Point","Polygon"],"allowNoLocation":true,"propertyDefs":{"a.b":{"displayName":"Noll","description":"doc","dataType":"number","quantity":"temperature","unit":"Cel","signals":[{"object":"urn:oma:lwm2m:ext:3303","resource":"5700"}]}},"relations":[{"name":"serves","allowedTargets":["building"],"min":0,"max":3}]},"paramDefaults":{"offset":0},"overridable":["offset"],"paramInfo":{"offset":{"unit":"Cel","description":"doc","min":0,"max":10}},"recipes":[{"name":"r","operator":"identity","version":"v1","inputs":["input"],"outputs":["a.b"],"params":["offset"]}],"operatorVersions":{"identity":"v1"}}`

func catalogHarness(t *testing.T, status int) (*testCatalogApp, *json.RawMessage) {
	t.Helper()
	var captured json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tenant") != "t1" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing tenant/token: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Error(err)
			}
			w.WriteHeader(status)
			if status != 201 {
				_, _ = w.Write([]byte(`{"error":"propertyDefs.a.b: rejected"}`))
			}
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/variants/"):
			_, _ = w.Write([]byte(`{"variant":{"id":"preset","version":"v1","templateId":"neutral","templateVersion":"v1","paramValues":{"offset":0}},"presetProperties":["a.b"]}`))
		case strings.HasSuffix(r.URL.Path, "/templates"):
			_, _ = w.Write([]byte("[" + completeTemplate + "]"))
		default:
			_, _ = w.Write([]byte(completeTemplate))
		}
	}))
	t.Cleanup(srv.Close)
	c := &client.Client{}
	return &testCatalogApp{catalog: appcatalog.NewService(c, srv.URL), v2: appthings.NewService(c, srv.URL)}, &captured
}

func authorized(r *http.Request) *http.Request {
	ctx := auth.WithToken(r.Context(), "test-token")
	ctx = auth.WithAccess(ctx, map[string]map[auth.Scope]struct{}{"t1": {auth.AnyScope: {}}})
	return r.WithContext(ctx)
}

func TestPublishedTemplateHTMLRoundTripAllFields(t *testing.T) {
	app, captured := catalogHarness(t, 201)
	get := authorized(httptest.NewRequest("GET", "/catalog/templates/new?tenant=t1&from=neutral/v1", nil))
	get.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	NewTemplateNewPage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, get)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	form := serializedForm(t, rec.Body.String())
	form.Set("version", "v2")
	post := authorized(httptest.NewRequest("POST", "/catalog/templates/new", strings.NewReader(form.Encode())))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, post)
	if rec.Code != 302 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var want, got map[string]any
	var source appthings.TemplateSpec
	if err := json.Unmarshal([]byte(completeTemplate), &source); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(canonical, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(*captured, &got); err != nil {
		t.Fatal(err)
	}
	want["template"].(map[string]any)["version"] = "v2"
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("field loss:\nwant %v\ngot %v", want, got)
	}
}

func TestCatalogErrorKeepsAdvancedFieldsAndStatus(t *testing.T) {
	for _, status := range []int{400, 409, 422, 401, 403} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			app, _ := catalogHarness(t, status)
			form := url.Values{"tenant": {"t1"}, "id": {"neutral"}, "version": {"v2"}, "propertyDefs": {`{"a.b":{"unit":"Cel"}}`}, "relations": {`[{"name":"serves","min":0,"max":2}]`}, "recipes": {`[]`}, "paramInfo": {`{"offset":{"min":0}}`}, "operatorVersions": {`{"identity":"v1"}`}, "repeatGroups": {"zones"}, "primaryProperty": {"a.b"}}
			r := authorized(httptest.NewRequest("POST", "/catalog/templates/new", strings.NewReader(form.Encode())))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, r)
			wantStatus := 200
			if status == 401 || status == 403 {
				wantStatus = status
			}
			if rec.Code != wantStatus {
				t.Fatal(rec.Code, rec.Body.String())
			}
			got := serializedForm(t, rec.Body.String())
			for key, values := range form {
				if got.Get(key) != values[0] {
					t.Errorf("lost %s: %q", key, got.Get(key))
				}
			}
			if status == 422 && !strings.Contains(rec.Body.String(), "propertyDefs.a.b") {
				t.Error("backend field error hidden")
			}
		})
	}
}

func TestJSONFieldsReplaceAndRejectLossyInput(t *testing.T) {
	for _, raw := range []string{`{} {}`, `{"x":{"unknown":true}}`, `{"x":`} {
		var defs map[string]appthings.PropertyDef
		if err := decodeField(map[string][]string{"propertyDefs": {raw}}, "propertyDefs", &defs); err == nil {
			t.Error("accepted", raw)
		}
	}
	defs := map[string]appthings.PropertyDef{"old": {Unit: "Cel"}}
	if err := decodeField(map[string][]string{"propertyDefs": {`{"new":{"unit":"m"}}`}}, "propertyDefs", &defs); err != nil {
		t.Fatal(err)
	}
	if _, exists := defs["old"]; exists {
		t.Error("removed definition retained")
	}
	if err := decodeField(map[string][]string{"propertyDefs": {""}}, "propertyDefs", &defs); err != nil || defs != nil {
		t.Fatal("explicit clearing failed", err, defs)
	}
	for _, raw := range []string{"x=NaN", "x=+Inf", "x=0\nx=1"} {
		if _, err := parseKeyValues(raw); err == nil {
			t.Error("accepted", raw)
		}
	}
}

func TestVariantHTMLCopyPreservesExactVersionAndPreset(t *testing.T) {
	app, captured := catalogHarness(t, 201)
	req := authorized(httptest.NewRequest("GET", "/catalog/variants/new?tenant=t1&from=preset/v1", nil))
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	NewVariantNewPage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, req)
	form := serializedForm(t, rec.Body.String())
	form.Set("version", "v2")
	r := authorized(httptest.NewRequest("POST", "/catalog/variants/new", strings.NewReader(form.Encode())))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	NewVariantCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, r)
	if rec.Code != 302 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var got appthings.VariantSpec
	if err := json.Unmarshal(*captured, &got); err != nil {
		t.Fatal(err)
	}
	if got.Variant.TemplateVersion != "v1" || got.Variant.Version != "v2" || !reflect.DeepEqual(got.PresetProperties, []string{"a.b"}) {
		t.Fatal(got)
	}
}

func TestRelationRowsAddEditRemoveWithoutPublication(t *testing.T) {
	app, captured := catalogHarness(t, 201)
	form := url.Values{
		"tenant": {"t1"}, "id": {"neutral"}, "version": {"v2"}, "displayName": {"keep <&>"},
		"propertyDefs": {`{"a.b":{"unit":"Cel"}}`}, "relationRowsPresent": {"true"},
		"relationName": {"serves", "removed"}, "relationTargets": {"building\nroom", "old"},
		"relationMin": {"1", "0"}, "relationMax": {"3", "1"}, "addRelation": {"true"},
	}
	r := authorized(httptest.NewRequest("POST", "/catalog/templates/new", strings.NewReader(form.Encode())))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler := NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)
	handler(rec, r)
	if rec.Code != 200 || len(*captured) != 0 {
		t.Fatal("adding row published", rec.Code, string(*captured))
	}
	posted := serializedForm(t, rec.Body.String())
	if len(posted["relationName"]) != 3 || posted.Get("displayName") != form.Get("displayName") || posted.Get("propertyDefs") != form.Get("propertyDefs") {
		t.Fatal("adding row lost input", posted)
	}
	posted["relationName"][1] = "" // remove second slot
	posted["relationName"][2] = "backsUp"
	posted["relationTargets"][2] = "building"
	posted["relationMax"][2] = "2"
	r = authorized(httptest.NewRequest("POST", "/catalog/templates/new", strings.NewReader(posted.Encode())))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler(rec, r)
	if rec.Code != 302 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var spec appthings.TemplateSpec
	if err := json.Unmarshal(*captured, &spec); err != nil {
		t.Fatal(err)
	}
	want := []appthings.RelationSpec{{Name: "serves", AllowedTargets: []string{"building", "room"}, Min: 1, Max: 3}, {Name: "backsUp", AllowedTargets: []string{"building"}, Max: 2}}
	if !reflect.DeepEqual(spec.Template.Relations, want) {
		t.Fatal(spec.Template.Relations)
	}
}

func TestMalformedJSONIsEchoedVerbatim(t *testing.T) {
	app, captured := catalogHarness(t, 201)
	raw := "  {\"a.b\": <&>\n  "
	form := url.Values{"tenant": {"t1"}, "id": {"neutral"}, "version": {"v2"}, "propertyDefs": {raw}, "recipes": {"["}}
	r := authorized(httptest.NewRequest("POST", "/catalog/templates/new", strings.NewReader(form.Encode())))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	NewTemplateCreatePage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, r)
	got := serializedForm(t, rec.Body.String())
	if got.Get("propertyDefs") != raw || got.Get("recipes") != "[" || len(*captured) != 0 {
		t.Fatal("malformed input lost or published", got)
	}
}
