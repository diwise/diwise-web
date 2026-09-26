package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/matryer/is"
)

func testSpec() thingsv2.TemplateSpec {
	return thingsv2.TemplateSpec{
		Template: thingsv2.Template{
			ID: "wastebin", Version: "v2", Category: "waste",
			DisplayName: "Soptunna", Required: []string{"fillRate"},
			Relations: []thingsv2.RelationSpec{{Name: "partOf"}},
		},
		ParamDefaults: map[string]float64{"level": 1},
		Recipes:       []thingsv2.Recipe{{Name: "r", Operator: "sum", Inputs: []string{"a"}, Outputs: []string{"b"}}},
	}
}

// stubCatalog svarar som catalog-API:t: GET en version, POST skapar (201
// tom), dubblett ger 409, ogiltig body ger 400.
func stubCatalog(t *testing.T, seen *map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	spec := testSpec()

	mux.HandleFunc("/catalog/templates", func(w http.ResponseWriter, r *http.Request) {
		var s thingsv2.TemplateSpec
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil || s.Template.ID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if s.Template.Version == "v1" {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/catalog/templates/{id}/{version}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != "wastebin" || r.PathValue("version") != "v1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		(*seen)["tenant"] = r.URL.Query().Get("tenant")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	})
	mux.HandleFunc("/catalog/variants", func(w http.ResponseWriter, r *http.Request) {
		var s thingsv2.VariantSpec
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil || s.Variant.ID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	return httptest.NewServer(mux)
}

func testService(t *testing.T) (*Service, *map[string]string, func()) {
	t.Helper()
	seen := &map[string]string{}
	srv := stubCatalog(t, seen)
	return NewService(&client.Client{}, srv.URL), seen, srv.Close
}

func TestGetTemplateRoundTripsRecipes(t *testing.T) {
	is := is.New(t)
	svc, seen, done := testService(t)
	defer done()

	spec, err := svc.GetTemplate(context.Background(), "t", "wastebin", "v1")
	is.NoErr(err)
	is.Equal("wastebin", spec.Template.ID)
	is.Equal("t", (*seen)["tenant"])
	// Recipes följer med (read-only i GUI, kopieras vid ny version).
	is.Equal(1, len(spec.Recipes))
	is.Equal("sum", spec.Recipes[0].Operator)
	is.Equal(1, len(spec.Template.Relations))

	_, err = svc.GetTemplate(context.Background(), "t", "nope", "v9")
	is.True(errors.Is(err, client.ErrNotFound))
}

func TestPublishTemplateConflictAndValidation(t *testing.T) {
	is := is.New(t)
	svc, _, done := testService(t)
	defer done()

	is.NoErr(svc.PublishTemplate(context.Background(), "t", testSpec()))

	dup := testSpec()
	dup.Template.Version = "v1"
	err := svc.PublishTemplate(context.Background(), "t", dup)
	is.True(errors.Is(err, client.ErrConflict))

	bad := testSpec()
	bad.Template.ID = ""
	err = svc.PublishTemplate(context.Background(), "t", bad)
	is.True(err != nil && !errors.Is(err, client.ErrConflict))
}

func TestPublishVariant(t *testing.T) {
	is := is.New(t)
	svc, _, done := testService(t)
	defer done()

	spec := thingsv2.VariantSpec{Variant: thingsv2.Variant{
		ID: "v1", Version: "v2", TemplateID: "wastebin", TemplateVersion: "v1",
		ParamValues: map[string]float64{"level": 2},
	}}
	is.NoErr(svc.PublishVariant(context.Background(), "t", spec))

	bad := thingsv2.VariantSpec{}
	is.True(svc.PublishVariant(context.Background(), "t", bad) != nil)
}
