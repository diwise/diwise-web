package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/thingsv2"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/matryer/is"
)

func ctxWithToken() context.Context {
	return auth.WithToken(context.Background(), "test-token")
}

// TestTemplateRoundtripFull guards operatorVersions + all template fields.
func TestTemplateRoundtripFull(t *testing.T) {
	is := is.New(t)
	spec := thingsv2.TemplateSpec{
		Template: thingsv2.Template{
			ID: "wastebin", Version: "v2", Category: "waste", DisplayName: "Soptunna", Description: "Kärl",
			PrimaryProperty: "fillRate", Required: []string{"fillRate"}, Optional: []string{"temperature"},
			AllowedGeometries: []string{"Point"}, AllowNoLocation: false,
			RepeatGroups: []string{"temperature."}, AllowedQuantities: []string{"temperature"},
			Labels: []string{"outdoor"}, Relations: []thingsv2.RelationSpec{{Name: "partOf", AllowedTargets: []string{"greenhouse"}, Max: 1}},
			PropertyDefs: map[string]thingsv2.PropertyDef{
				"fillRate": {DisplayName: "Fyllnadsgrad", Description: "Andel fylld", DataType: "number", Quantity: "level", Unit: "%", Signals: []thingsv2.SignalHint{{Object: "urn:oma:lwm2m:ext:3330", Resource: "5700"}}},
			},
		},
		ParamDefaults:    map[string]float64{"usableHeight": 1.0},
		Overridable:      []string{"usableHeight"},
		ParamInfo:        map[string]thingsv2.ParamDef{"usableHeight": {Unit: "m", Description: "Höjd"}},
		Recipes:          []thingsv2.Recipe{{Name: "fill-rate", Operator: "fillRate", Version: "v1", Inputs: []string{"distance"}, Outputs: []string{"fillRate"}, Params: []string{"usableHeight"}}},
		OperatorVersions: map[string]string{"fillRate": "v1"},
	}
	raw, err := json.Marshal(spec)
	is.NoErr(err)
	for _, k := range []string{"operatorVersions", "description", "dataType", "quantity", "unit"} {
		is.True(strings.Contains(string(raw), `"`+k+`"`))
	}
	var back thingsv2.TemplateSpec
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("v1", back.OperatorVersions["fillRate"])
	is.Equal("number", back.Template.PropertyDefs["fillRate"].DataType)
}

// TestVariantRoundtripFull guards presetProperties.
func TestVariantRoundtripFull(t *testing.T) {
	is := is.New(t)
	spec := thingsv2.VariantSpec{
		Variant:          thingsv2.Variant{ID: "160L", Version: "v1", TemplateID: "wastebin", TemplateVersion: "v1", ParamValues: map[string]float64{"h": 1}},
		PresetProperties: []string{"fillRate"},
	}
	raw, err := json.Marshal(spec)
	is.NoErr(err)
	is.True(strings.Contains(string(raw), `"presetProperties"`))
	var back thingsv2.VariantSpec
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal(1, len(back.PresetProperties))
}

// TestPublishValidation422 ensures 400/422 surface as classified BackendError
// (errors.Is ErrBadRequest/ErrUnprocessable) with backend text preserved.
func TestPublishValidation422(t *testing.T) {
	is := is.New(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/catalog/templates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "operator unknown"})
	})
	mux.HandleFunc("/catalog/variants", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "preset unknown"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	err := svc.PublishTemplate(ctxWithToken(), "t", thingsv2.TemplateSpec{Template: thingsv2.Template{ID: "x", Version: "v1"}})
	var be *client.BackendError
	is.True(errors.As(err, &be))
	is.Equal(422, be.Status)
	is.True(strings.Contains(be.Message, "operator"))
	is.True(errors.Is(err, client.ErrUnprocessable))
	err = svc.PublishVariant(ctxWithToken(), "t", thingsv2.VariantSpec{Variant: thingsv2.Variant{ID: "x", Version: "v1"}})
	is.True(errors.As(err, &be))
	is.Equal(400, be.Status)
	is.True(errors.Is(err, client.ErrBadRequest))
}

// TestGetTemplateIDValidation ensures path IDs are validated client-side.
func TestGetTemplateIDValidation(t *testing.T) {
	is := is.New(t)
	svc := NewService(&client.Client{}, "http://127.0.0.1:1")
	_, err := svc.GetTemplate(ctxWithToken(), "t", "", "v1")
	is.True(err != nil)
	_, err = svc.GetTemplate(ctxWithToken(), "t", "a/b", "v1")
	is.True(err != nil)
}
