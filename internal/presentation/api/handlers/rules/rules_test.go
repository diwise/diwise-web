package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/matryer/is"
)

type testRulesApp struct {
	transforms *apptransform.Service
}

func (a *testRulesApp) Transforms() *apptransform.Service { return a.transforms }
func (a *testRulesApp) GetTenants(context.Context) []string {
	return []string{"t1"}
}

func testLocaleBundle() *ftkmock.LocaleBundleMock {
	return &ftkmock.LocaleBundleMock{
		ForFunc: func(string) frontendtoolkit.Localizer {
			return &ftkmock.LocalizerMock{
				GetFunc:         func(key string) string { return key },
				GetWithDataFunc: func(key string, _ map[string]any) string { return key },
			}
		},
	}
}

func testAssets() frontendtoolkit.AssetLoaderFunc {
	return func(name string) frontendtoolkit.Asset { return stubAsset{path: name} }
}

type stubAsset struct{ path string }

func (a stubAsset) Body() []byte        { return nil }
func (a stubAsset) ContentLength() int  { return 0 }
func (a stubAsset) ContentType() string { return "text/plain" }
func (a stubAsset) Path() string        { return a.path }
func (a stubAsset) SHA256() string      { return "" }

func testRule() apptransform.Rule {
	return apptransform.Rule{
		Match: apptransform.Match{Kind: "thing", Event: "things.v1.values", Type: "room", Tenant: "t1"},
		Entities: []apptransform.Entity{{
			ID: "urn:ngsi-ld:Room:{{nameOrID}}", Type: "Room",
			Properties: []apptransform.Property{{Target: "name", Type: "Text", Source: apptransform.Source{Field: "name"}}},
		}},
	}
}

// stubRules svarar som regel-API:t: lista, en regel (seed + api), delete
// med revision (204, 404 okänt, 409 fel revision).
func stubRules(t *testing.T) (*apptransform.Service, func()) {
	t.Helper()

	seed := apptransform.Model{ID: "11111111-1111-1111-1111-111111111111", Revision: 3, Source: "seed", SeedKey: "90-room#0", Kind: "thing", Rule: testRule()}
	owned := apptransform.Model{ID: "22222222-2222-2222-2222-222222222222", Revision: 1, Source: "api", Kind: "thing", Rule: testRule()}

	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var rule apptransform.Rule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil || len(rule.Entities) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rule must declare at least one entity"})
				return
			}
			created := owned
			created.Rule = rule
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(created)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []apptransform.Model{seed, owned}})
	})
	mux.HandleFunc("/models/validate", func(w http.ResponseWriter, r *http.Request) {
		var rule apptransform.Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil || len(rule.Entities) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rule must declare at least one entity"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"valid": true})
	})
	mux.HandleFunc("/models/preview", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Rule  apptransform.Rule `json:"rule"`
			Event struct {
				Kind string `json:"kind"`
			} `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Event.Kind == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "event.kind is required"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"matched": true, "observations": 1,
			"entities": []any{map[string]any{
				"id": "urn:ngsi-ld:Room:r1", "type": "Room",
				"properties": map[string]any{}, "operation": "merge",
			}},
		})
	})
	mux.HandleFunc("/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		models := map[string]apptransform.Model{seed.ID: seed, owned.ID: owned}
		m, ok := models[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m)
		case http.MethodPut:
			var rule apptransform.Rule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil || len(rule.Entities) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rule must declare at least one entity"})
				return
			}
			if r.Header.Get("If-Match") != fmt.Sprintf(`"rev-%d"`, m.Revision) {
				w.WriteHeader(http.StatusConflict)
				return
			}
			m.Rule = rule
			m.Revision++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m)
		case http.MethodDelete:
			if r.Header.Get("If-Match") != fmt.Sprintf(`"rev-%d"`, m.Revision) {
				w.WriteHeader(http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	})

	srv := httptest.NewServer(mux)
	return apptransform.NewService(&client.Client{}, srv.URL), srv.Close
}

func testApp(t *testing.T) (*testRulesApp, func()) {
	t.Helper()
	svc, done := stubRules(t)
	return &testRulesApp{transforms: svc}, done
}

func TestRulesPageRendersRowsAndFilters(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRulesPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/rules", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "11111111"))
	is.True(strings.Contains(body, "seed"))
	is.True(strings.Contains(body, "things.v1.values"))

	// Klientsides filter: kind utan träff ger tom tabell.
	req = httptest.NewRequest(http.MethodGet, "/rules?kind=measurement", nil)
	req.Header.Set("HX-Request", "true")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(!strings.Contains(rec.Body.String(), "11111111"))
}

func TestRuleDetailsPageShowsEditorAndSeedBanner(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleDetailsPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/rules/11111111-1111-1111-1111-111111111111", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "11111111-1111-1111-1111-111111111111")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Editor med ifylld regel + dold revision + seed-banner + delete.
	is.True(strings.Contains(body, `name="revision" value="3"`))
	is.True(strings.Contains(body, "urn:ngsi-ld:Room"))
	is.True(strings.Contains(body, "rules_seed_banner"))
	is.True(strings.Contains(body, "/rules/11111111-1111-1111-1111-111111111111/delete"))
}

func TestRuleDetailsPageReturns404ForUnknownID(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleDetailsPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/rules/nope", nil)
	req.SetPathValue("id", "nope")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusNotFound, rec.Code)
}

func TestRuleDeleteRequiresConfirmThenDeletes(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleDeletePage(context.Background(), testLocaleBundle(), testAssets(), app)
	id := "22222222-2222-2222-2222-222222222222"

	// Första POST utan confirm visar bekräftelsen.
	form := url.Values{"revision": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/rules/"+id+"/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "rules_confirm_delete"))

	// Andra POST med confirm raderar och redirectar.
	form.Set("confirm", "yes")
	req = httptest.NewRequest(http.MethodPost, "/rules/"+id+"/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", id)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusFound, rec.Code)
	is.Equal("/rules?notice=deleted", rec.Header().Get("Location"))
}

func TestRuleDeleteUnknownIDIsGone(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewRuleDeletePage(context.Background(), testLocaleBundle(), testAssets(), app)

	// Okänt id i confirm-steget: 404 (finns inget att bekräfta).
	form := url.Values{"revision": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/rules/nope/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", "nope")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusNotFound, rec.Code)
}
