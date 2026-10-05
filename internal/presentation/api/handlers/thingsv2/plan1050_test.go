package thingsv2

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	appthings "github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	view "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"
	"golang.org/x/net/html"
)

func relationApp(t *testing.T) (*testThingsV2App, context.Context) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tenant") != "t1" {
			t.Errorf("unscoped read: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/export"):
			// Export shape matches B catalog.ObjectSpec, not list ThingSummary.
			_, _ = w.Write([]byte(`[{"thingId":"a","tenant":"t1","name":"A","templateId":"building","templateVersion":"v1","relations":[{"name":"serves","targetThingId":"source"}]},{"thingId":"b","tenant":"t1","name":"B","templateId":"building","templateVersion":"v2"},{"thingId":"wrong","tenant":"t1","name":"Wrong type","templateId":"sensor","templateVersion":"v1"},{"thingId":"foreign","tenant":"t2","name":"SECRET","templateId":"building","templateVersion":"v1"}]`))
		case strings.HasSuffix(r.URL.Path, "/relations"):
			_, _ = w.Write([]byte(`{"relations":[{"name":"serves","targetThingId":"a","target":{"thingId":"a","tenant":"t1","name":"A","templateId":"building","templateVersion":"v1"}},{"name":"serves","targetThingId":"deleted","target":null}]}`))
		case strings.HasSuffix(r.URL.Path, "/a"), strings.HasSuffix(r.URL.Path, "/b"):
			_, _ = w.Write([]byte(`{"thingId":"` + r.PathValue("id") + `","tenant":"t1","templateId":"building","templateVersion":"v1"}`))
		case strings.HasSuffix(r.URL.Path, "/wrong"):
			_, _ = w.Write([]byte(`{"thingId":"wrong","tenant":"t1","templateId":"sensor","templateVersion":"v1"}`))
		case strings.HasSuffix(r.URL.Path, "/foreign"):
			w.WriteHeader(403)
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(srv.Close)
	return &testThingsV2App{svc: appthings.NewService(&client.Client{}, srv.URL)}, auth.WithToken(context.Background(), "test-token")
}

func selectedRelationControls(t *testing.T, body string) url.Values {
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
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		name, _ := attr(n, "name")
		if n.Data == "select" && strings.HasPrefix(name, "relation.") {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Data == "option" {
					if _, selected := attr(c, "selected"); selected {
						value, _ := attr(c, "value")
						values.Add(name, value)
					}
				}
			}
		}
		if n.Data == "input" && name == "relationsPresent" {
			v, _ := attr(n, "value")
			values.Set(name, v)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return values
}

func TestGenericRelationsHTMLSerializationAndScope(t *testing.T) {
	app, ctx := relationApp(t)
	slots := []appthings.RelationSpec{{Name: "serves", AllowedTargets: []string{"building"}, Max: 2}}
	stored := []appthings.RelationRef{{Name: "serves", TargetThingID: "a"}, {Name: "serves", TargetThingID: "b"}}
	rows, message := relationSlots(ctx, app, "t1", "source", slots, stored, nil)
	if message != "" {
		t.Fatal(message)
	}
	var buf bytes.Buffer
	if err := view.ThingV2RelationFields(rows, message).Render(ctx, &buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "SECRET") || strings.Contains(buf.String(), "Wrong type") {
		t.Fatal("ineligible choices exposed", buf.String())
	}
	form := selectedRelationControls(t, buf.String())
	refs, fail := parseRelations(ctx, app, "t1", "source", slots, stored, form)
	if fail != "" || len(refs) != 2 || refs[0].TargetTemplateID != "building" {
		t.Fatal(refs, fail)
	}
	// Moving changes only the selected target set; clearing all removes the slot.
	form.Set("relation.serves", "b")
	refs, fail = parseRelations(ctx, app, "t1", "source", slots, stored, form)
	if fail != "" || len(refs) != 1 || refs[0].TargetThingID != "b" {
		t.Fatal(refs, fail)
	}
	form.Del("relation.serves")
	refs, fail = parseRelations(ctx, app, "t1", "source", slots, stored, form)
	if fail != "" || len(refs) != 0 {
		t.Fatal(refs, fail)
	}
	for _, selected := range [][]string{{"wrong"}, {"foreign"}, {"a", "a"}, {"a", "b", "c"}} {
		form["relation.serves"] = selected
		if _, fail := parseRelations(ctx, app, "t1", "source", slots, stored, form); fail == "" {
			t.Error("accepted", selected)
		}
	}
	form["relation.serves"] = []string{"deleted"}
	rows, _ = relationSlots(ctx, app, "t1", "source", slots, stored, form)
	buf.Reset()
	_ = view.ThingV2RelationFields(rows, "").Render(ctx, &buf)
	if !reflect.DeepEqual(selectedRelationControls(t, buf.String())["relation.serves"], []string{"deleted"}) {
		t.Fatal("invalid input lost", buf.String())
	}
}

func TestResolvedAndReverseRelationsKeepBrokenReferences(t *testing.T) {
	app, ctx := relationApp(t)
	rows, message := detailRelations(ctx, app, "t1", "source")
	if message != "" || len(rows) != 3 {
		t.Fatal(rows, message)
	}
	if !rows[1].Broken || rows[1].ID != "deleted" || !rows[2].Incoming || rows[2].Name != "serves" {
		t.Fatal(rows)
	}
	var buf bytes.Buffer
	_ = view.ThingV2RelationsSection(view.ThingV2DetailsViewModel{Tenant: "t1", Relations: rows}).Render(ctx, &buf)
	if !strings.Contains(buf.String(), "Bruten referens: deleted") || !strings.Contains(buf.String(), "tenant=t1") {
		t.Fatal(buf.String())
	}
}

func TestThingSaveSerializesPreservedFieldsAndRevision(t *testing.T) {
	for _, status := range []int{200, 409, 422, 401, 403} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var captured appthings.ObjectSpec
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("tenant") != "t1" {
					t.Errorf("missing tenant %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "PUT" {
					if r.Header.Get("If-Match") != `"rev-7"` {
						t.Error("revision lost", r.Header.Get("If-Match"))
					}
					if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
						t.Error(err)
					}
					w.WriteHeader(status)
					if status == 200 {
						_, _ = w.Write([]byte(`{"thingId":"` + testBinID + `","tenant":"t1"}`))
					} else {
						_, _ = w.Write([]byte(`{"error":"relations.serves: rejected"}`))
					}
					return
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					_, _ = w.Write([]byte(`{"name":"Stored","alternativeName":"Alt","subType":"Special","category":"custom","templateId":"neutral","templateVersion":"v1","properties":{"a.b":{"unit":"Cel"}},"location":{"type":"Point","coordinates":[17,62,3]},"bindings":[{"deviceID":"sensor","object":"obj","resource":"5700","input":"input"}],"metadata":{"keep":"yes"},"relations":[{"name":"serves","targetThingId":"a","targetTemplateId":"building"}]}`))
				case strings.HasSuffix(r.URL.Path, "/templates"):
					_, _ = w.Write([]byte(`[{"template":{"id":"neutral","version":"v1","allowNoLocation":true,"relations":[{"name":"serves","allowedTargets":["building"],"max":2}]},"overridable":["offset"]}]`))
				case strings.HasSuffix(r.URL.Path, "/variants"), strings.HasSuffix(r.URL.Path, "/export"):
					_, _ = w.Write([]byte(`[]`))
				case strings.HasSuffix(r.URL.Path, "/a"):
					_, _ = w.Write([]byte(`{"thingId":"a","tenant":"t1","templateId":"building","templateVersion":"v1"}`))
				default:
					_, _ = w.Write([]byte(`{"thingId":"` + testBinID + `","tenant":"t1","revision":8}`))
				}
			}))
			defer srv.Close()
			app := &testThingsV2App{svc: appthings.NewService(&client.Client{}, srv.URL)}
			form := url.Values{"tenant": {"t1"}, "revision": {"7"}, "name": {"Edited <&>"}, "description": {"new"}, "param.offset": {"0"}, "latitude": {"62"}, "longitude": {"17"}, "relationsPresent": {"true"}, "relation.serves": {"a"}}
			r := httptest.NewRequest("POST", "/things-v2/"+testBinID, strings.NewReader(form.Encode()))
			r = r.WithContext(auth.WithToken(r.Context(), "test-token"))
			r.SetPathValue("id", testBinID)
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			NewThingsV2SavePage(context.Background(), testLocaleBundle(), testAssets(), app)(rec, r)
			if captured.AlternativeName != "Alt" || captured.SubType != "Special" || captured.Category != "custom" || captured.Metadata["keep"] != "yes" || string(captured.Location.Coordinates) != "[17,62,3]" || len(captured.Bindings) != 1 || len(captured.Relations) != 1 || captured.ParamOverrides["offset"] != 0 {
				t.Fatal("field loss", captured)
			}
			wantStatus := 200
			if status == 401 || status == 403 {
				wantStatus = status
			}
			if rec.Code != wantStatus {
				t.Fatal(rec.Code, rec.Body.String())
			}
			if status != 200 {
				body := rec.Body.String()
				if !strings.Contains(body, `value="7"`) || !strings.Contains(body, `Edited &lt;&amp;&gt;`) || !strings.Contains(body, `value="0"`) || !reflect.DeepEqual(selectedRelationControls(t, body)["relation.serves"], []string{"a"}) {
					t.Fatal("input lost", body)
				}
				if status == 422 && !strings.Contains(body, "relations.serves") {
					t.Error("field error hidden")
				}
			}
		})
	}
}

func TestTenantChoiceCannotExpandAccess(t *testing.T) {
	r := httptest.NewRequest("GET", "/things-v2/new?tenant=t2", nil)
	r = r.WithContext(auth.WithAccess(r.Context(), map[string]map[auth.Scope]struct{}{"t1": {auth.AnyScope: {}}}))
	if _, ok := resolveCreateTenant(r, tokenTenants(r), "t2"); ok {
		t.Fatal("cross tenant choice accepted")
	}
	if _, err := composeCreateModel(r.Context(), r, nil, "t2", "", nil, ""); err == nil {
		t.Fatal("cross tenant composition called backend")
	}
}
