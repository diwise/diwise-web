package catalog

import (
	"context"
	"encoding/json"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/catalog"
	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/matryer/is"
)

type testCatalogApp struct {
	catalog *catalog.Service
	v2      *appthingsv2.Service
}

func (a *testCatalogApp) Catalog() *catalog.Service      { return a.catalog }
func (a *testCatalogApp) ThingsV2() *appthingsv2.Service { return a.v2 }

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

// stubCatalog svarar som iot-things-v2 catalog: lista med category-filter,
// en version, 404 för okänt, POST skapar (201 tom), dubblett 409, ogiltig
// body 400. Samma för varianter.
func stubCatalog(t *testing.T) (*catalog.Service, *appthingsv2.Service, func()) {
	t.Helper()

	specs := []appthingsv2.TemplateSpec{
		{Template: appthingsv2.Template{
			ID: "wastebin", Version: "v1", Category: "waste",
			DisplayName: "Soptunna", Description: "Mäter fyllnadsgrad",
			Required: []string{"fillRate"}, Optional: []string{"temperature"},
			Relations: []appthingsv2.RelationSpec{{Name: "partOf"}},
		}},
		{Template: appthingsv2.Template{
			ID: "room", Version: "v2", Category: "indoor",
			DisplayName: "Rum", Required: []string{"temperature"},
		}},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/catalog/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var spec appthingsv2.TemplateSpec
			if err := json.NewDecoder(r.Body).Decode(&spec); err != nil || spec.Template.ID == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			for _, s := range specs {
				if s.Template.ID == spec.Template.ID && s.Template.Version == spec.Template.Version {
					w.WriteHeader(http.StatusConflict)
					return
				}
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		out := specs
		if cat := r.URL.Query().Get("category"); cat != "" {
			out = nil
			for _, s := range specs {
				if s.Template.Category == cat {
					out = append(out, s)
				}
			}
			if out == nil {
				out = []appthingsv2.TemplateSpec{}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/catalog/templates/{id}/{version}", func(w http.ResponseWriter, r *http.Request) {
		for _, s := range specs {
			if s.Template.ID == r.PathValue("id") && s.Template.Version == r.PathValue("version") {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(s)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	variant := appthingsv2.VariantSpec{Variant: appthingsv2.Variant{
		ID: "std", Version: "v1", TemplateID: "wastebin", TemplateVersion: "v1",
		ParamValues: map[string]float64{"level": 1},
	}}
	mux.HandleFunc("/catalog/variants", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var s appthingsv2.VariantSpec
			if err := json.NewDecoder(r.Body).Decode(&s); err != nil || s.Variant.ID == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if s.Variant.ID == variant.Variant.ID && s.Variant.Version == variant.Variant.Version {
				w.WriteHeader(http.StatusConflict)
				return
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]appthingsv2.VariantSpec{variant})
	})
	mux.HandleFunc("/catalog/variants/{id}/{version}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == variant.Variant.ID && r.PathValue("version") == variant.Variant.Version {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(variant)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	c := &client.Client{}
	return catalog.NewService(c, srv.URL), appthingsv2.NewService(c, srv.URL), srv.Close
}

func testApp(t *testing.T) (*testCatalogApp, func()) {
	t.Helper()
	cat, v2, done := stubCatalog(t)
	return &testCatalogApp{catalog: cat, v2: v2}, done
}

func TestTemplatesPageRendersRows(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplatesPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/catalog/templates?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "wastebin"))
	is.True(strings.Contains(body, "Soptunna"))
	is.True(strings.Contains(body, "room"))
	is.True(strings.Contains(body, "/catalog/templates/wastebin/v1?tenant=t1"))
}

func TestTemplatesPageFiltersByCategory(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplatesPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/catalog/templates?tenant=t1&category=waste", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "wastebin"))
	is.True(!strings.Contains(body, "room"))
}

func TestTemplatesPageWithoutTenantShowsPicker(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplatesPage(context.Background(), testLocaleBundle(), testAssets(), app)

	// Utan token-tenants och utan ?tenant=: väljare, ingen datahämtning.
	req := httptest.NewRequest(http.MethodGet, "/catalog/templates", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(!strings.Contains(rec.Body.String(), "wastebin"))
}

func TestTemplateDetailsPageRendersSpec(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateDetailsPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/catalog/templates/wastebin/v1?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "wastebin")
	req.SetPathValue("version", "v1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "wastebin"))
	is.True(strings.Contains(body, "Soptunna"))
	is.True(strings.Contains(body, "fillRate"))
	is.True(strings.Contains(body, "partOf"))
}

func TestTemplateDetailsPageReturns404ForUnknownVersion(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateDetailsPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/catalog/templates/nope/v9?tenant=t1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.SetPathValue("id", "nope")
	req.SetPathValue("version", "v9")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusNotFound, rec.Code)
}

func TestTemplateDetailsPageRequiresTenantWhenAmbiguous(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewTemplateDetailsPage(context.Background(), testLocaleBundle(), testAssets(), app)

	req := httptest.NewRequest(http.MethodGet, "/catalog/templates/wastebin/v1", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.SetPathValue("id", "wastebin")
	req.SetPathValue("version", "v1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}
