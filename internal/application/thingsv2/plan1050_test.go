package thingsv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/matryer/is"
)

func ctxWithToken() context.Context {
	return auth.WithToken(context.Background(), "test-token")
}

// TestPropertyDefFullRoundtrip guards the verified gap: description/
// dataType/quantity/unit must survive without loss (backend things.PropertyDef).
func TestPropertyDefFullRoundtrip(t *testing.T) {
	is := is.New(t)
	raw := `{"displayName":"Temp","description":"Lufttemperatur","dataType":"number","quantity":"temperature","unit":"Cel","signals":[{"object":"urn:oma:lwm2m:ext:3303","resource":"5700"}]}`
	var def PropertyDef
	is.NoErr(json.Unmarshal([]byte(raw), &def))
	is.Equal("Lufttemperatur", def.Description)
	is.Equal("number", def.DataType)
	is.Equal("temperature", def.Quantity)
	is.Equal("Cel", def.Unit)
	out, err := json.Marshal(def)
	is.NoErr(err)
	var back map[string]any
	is.NoErr(json.Unmarshal(out, &back))
	for _, k := range []string{"displayName", "description", "dataType", "quantity", "unit", "signals"} {
		is.True(back[k] != nil)
	}
}

// TestTemplateSpecOperatorVersionsRoundtrip guards operatorVersions (backend
// catalog.TemplateSpec key "operatorVersions").
func TestTemplateSpecOperatorVersionsRoundtrip(t *testing.T) {
	is := is.New(t)
	spec := TemplateSpec{
		Template:         Template{ID: "wastebin", Version: "v2"},
		OperatorVersions: map[string]string{"fillRate": "v1", "mean": "v2"},
		Recipes:          []Recipe{{Name: "r", Operator: "fillRate", Version: "v1", Inputs: []string{"a"}, Outputs: []string{"b"}}},
	}
	raw, err := json.Marshal(spec)
	is.NoErr(err)
	is.True(strings.Contains(string(raw), `"operatorVersions"`))
	var back TemplateSpec
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("v1", back.OperatorVersions["fillRate"])
	is.Equal("v2", back.OperatorVersions["mean"])
}

// TestVariantSpecPresetRoundtrip guards presetProperties (backend VariantSpec).
func TestVariantSpecPresetRoundtrip(t *testing.T) {
	is := is.New(t)
	spec := VariantSpec{
		Variant:          Variant{ID: "160L", Version: "v1", TemplateID: "wastebin", TemplateVersion: "v1"},
		PresetProperties: []string{"fillRate", "temperature.outdoor"},
	}
	raw, err := json.Marshal(spec)
	is.NoErr(err)
	is.True(strings.Contains(string(raw), `"presetProperties"`))
	var back VariantSpec
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal(2, len(back.PresetProperties))
}

// TestObjectSpecFullRoundtrip guards tenant/alternativeName/subType (K1).
func TestObjectSpecFullRoundtrip(t *testing.T) {
	is := is.New(t)
	spec := ObjectSpec{
		ThingID: "bin-1", Tenant: "t1", Name: "Tunna",
		AlternativeName: "Tunna Norr", SubType: "WasteContainer",
		TemplateID: "wastebin", TemplateVersion: "v1",
		Location: &Location{Type: "Point", Coordinates: json.RawMessage(`[18.06,59.33]`)},
	}
	raw, err := json.Marshal(spec)
	is.NoErr(err)
	for _, k := range []string{"tenant", "alternativeName", "subType"} {
		is.True(strings.Contains(string(raw), `"`+k+`"`))
	}
	var back ObjectSpec
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("t1", back.Tenant)
	is.Equal("Tunna Norr", back.AlternativeName)
	is.Equal("WasteContainer", back.SubType)
}

// TestEffectiveConfigFullRoundtrip guards alternativeName/subType/recipes.
func TestEffectiveConfigFullRoundtrip(t *testing.T) {
	is := is.New(t)
	cfg := EffectiveConfig{
		ThingID: "bin-1", Name: "Tunna", AlternativeName: "Alt", SubType: "Sub",
		TemplateID: "wastebin", TemplateVersion: "v1",
		Recipes:      []Recipe{{Name: "fill-rate", Operator: "fillRate", Version: "v1", Inputs: []string{"a"}, Outputs: []string{"b"}}},
		ParamSources: map[string]string{"p": "thing"},
	}
	raw, err := json.Marshal(cfg)
	is.NoErr(err)
	for _, k := range []string{"alternativeName", "subType", "recipes"} {
		is.True(strings.Contains(string(raw), `"`+k+`"`))
	}
	var back EffectiveConfig
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("Alt", back.AlternativeName)
	is.Equal("Sub", back.SubType)
	is.Equal(1, len(back.Recipes))
}

// TestThingFullRoundtrip guards alternativeName/subType/paramValues/paramSources.
func TestThingFullRoundtrip(t *testing.T) {
	is := is.New(t)
	v := 42.0
	thing := Thing{
		ThingID: "bin-1", Tenant: "t1", Name: "Tunna",
		AlternativeName: "Alt", SubType: "Sub",
		TemplateID: "wastebin", TemplateVersion: "v1", Revision: 3,
		Values:       map[string]PropertyValue{"fillRate": {PropertyID: "fillRate", Value: &v, Quality: "ok"}},
		ParamValues:  map[string]float64{"h": 1.2},
		ParamSources: map[string]string{"h": "variant"},
	}
	raw, err := json.Marshal(thing)
	is.NoErr(err)
	for _, k := range []string{"alternativeName", "subType", "paramValues", "paramSources"} {
		is.True(strings.Contains(string(raw), `"`+k+`"`))
	}
	var back Thing
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("Alt", back.AlternativeName)
	is.Equal("Sub", back.SubType)
	is.Equal(1.2, back.ParamValues["h"])
	is.Equal("variant", back.ParamSources["h"])
}

// TestThingDeltaAlias tolerates A-baseline "delta" as alias for "values".
func TestThingDeltaAlias(t *testing.T) {
	is := is.New(t)
	raw := `{"thingId":"bin-1","tenant":"t1","templateId":"wastebin","templateVersion":"v1","revision":1,"delta":{"fillRate":{"propertyId":"fillRate","value":42,"quality":"ok","observedAt":"2026-09-18T10:00:00Z"}}}`
	var thing Thing
	is.NoErr(json.Unmarshal([]byte(raw), &thing))
	is.True(thing.Values != nil)
	is.Equal(42.0, *thing.Values["fillRate"].Value)
	// Canonical marshal uses "values", never duplicate "delta".
	out, err := json.Marshal(thing)
	is.NoErr(err)
	is.True(strings.Contains(string(out), `"values"`))
	is.True(!strings.Contains(string(out), `"delta"`))
	// values wins when both present.
	raw2 := `{"thingId":"b","tenant":"t","templateId":"x","templateVersion":"v1","revision":1,"values":{"a":{"propertyId":"a","value":1,"observedAt":"2026-09-18T10:00:00Z"}},"delta":{"a":{"propertyId":"a","value":2,"observedAt":"2026-09-18T10:00:00Z"}}}`
	var thing2 Thing
	is.NoErr(json.Unmarshal([]byte(raw2), &thing2))
	is.Equal(1.0, *thing2.Values["a"].Value)
}

// TestHistoryPointFullRoundtrip guards receivedAt/source/messageId/late/unit
// (B HistoryWrite-facit: unit is frozen per observation, never relabeled).
func TestHistoryPointFullRoundtrip(t *testing.T) {
	is := is.New(t)
	ts := time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC)
	v := 42.0
	p := HistoryPoint{PropertyID: "fillRate", ObservedAt: ts, ReceivedAt: ts, Value: &v, Quality: "ok", Unit: "Cel", Source: "dev/3303/5700", MessageID: "m1", Late: true}
	raw, err := json.Marshal(p)
	is.NoErr(err)
	for _, k := range []string{"receivedAt", "source", "messageId", "late", "unit"} {
		is.True(strings.Contains(string(raw), `"`+k+`"`))
	}
	var back HistoryPoint
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("dev/3303/5700", back.Source)
	is.Equal("m1", back.MessageID)
	is.Equal("Cel", back.Unit)
	is.True(back.Late)
	is.True(back.ReceivedAt.Equal(ts))
}

// TestFilterBBoxNearQuery verifies bbox/near reach the backend as documented
// ?bbox=minLon,minLat,maxLon,maxLat and ?near=lon,lat,radiusMeters.
func TestFilterBBoxNearQuery(t *testing.T) {
	is := is.New(t)
	var gotBBox, gotNear, gotMeta string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBBox = r.URL.Query().Get("bbox")
		gotNear = r.URL.Query().Get("near")
		gotMeta = r.URL.Query().Get("meta.owner")
		w.Header().Set(TotalCountHeader, "0")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Thing{})
	}))
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	_, err := svc.ListThings(ctxWithToken(), "t1", Filter{
		BBox:     &BBox{MinLon: 17, MinLat: 62, MaxLon: 18, MaxLat: 63},
		Near:     &NearFilter{Lon: 17.3, Lat: 62.39, RadiusMeters: 500},
		Metadata: map[string]string{"owner": " park "[:4]},
		Limit:    10,
	})
	is.NoErr(err)
	is.Equal("17,62,18,63", gotBBox)
	is.Equal("17.3,62.39,500", gotNear)
	is.Equal(" par", gotMeta)
}

// stubPlan1050 mimics new B endpoints: export, relations, preview, upgrade,
// tree with single-doc JSON and tenant scoping.
func stubPlan1050(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/things/export", func(w http.ResponseWriter, r *http.Request) {
		specs := []ObjectSpec{{ThingID: "bin-1", Tenant: "t1", Name: "Tunna", AlternativeName: "Alt", SubType: "Sub", TemplateID: "wastebin", TemplateVersion: "v1", Location: &Location{Type: "Point", Coordinates: json.RawMessage(`[18.06,59.33]`)}}}
		if r.URL.Query().Get("tenant") == "empty" {
			specs = []ObjectSpec{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(specs)
	})
	mux.HandleFunc("/things/{id}/relations", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		thing := Thing{ThingID: "gh-1", Tenant: "t1", Name: "Växthus", TemplateID: "greenhouse", TemplateVersion: "v1", Revision: 1}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]RelationResolve{"relations": {
			{Name: "partOf", TargetThingID: "gh-1", Target: &thing},
			{Name: "monitors", TargetThingID: "gone"},
		}})
	})
	mux.HandleFunc("/things/{id}/preview", func(w http.ResponseWriter, r *http.Request) {
		var spec ObjectSpec
		if err := json.NewDecoder(r.Body).Decode(&spec); err != nil || spec.TemplateID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "templateId is required"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(EffectiveConfig{ThingID: r.PathValue("id"), Name: spec.Name, AlternativeName: spec.AlternativeName, SubType: spec.SubType, TemplateID: spec.TemplateID, TemplateVersion: spec.TemplateVersion, Recipes: []Recipe{{Name: "r", Operator: "sum", Inputs: []string{"a"}, Outputs: []string{"b"}}}})
	})
	mux.HandleFunc("/things/{id}/upgrade/preview", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UpgradeResult{Diff: Diff{AddedProps: []string{"newProp"}}, Conflicts: []string{"override \"h\" dropped"}})
	})
	mux.HandleFunc("/things/{id}/upgrade", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Match") != `"rev-2"` {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(UpgradeResult{Diff: Diff{AddedProps: []string{"newProp"}}, Conflicts: []string{"conflict"}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UpgradeResult{Diff: Diff{}, Conflicts: []string{}})
	})
	mux.HandleFunc("/things/{id}/tree", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TreeNode{Thing: Thing{ThingID: r.PathValue("id"), Tenant: "t1"}, Children: []TreeNode{{Thing: Thing{ThingID: "child-1", Tenant: "t1"}}}})
	})
	mux.HandleFunc("/things/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		points := []HistoryPoint{{PropertyID: "fillRate", ObservedAt: time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC), ReceivedAt: time.Date(2026, 9, 18, 10, 0, 1, 0, time.UTC), Source: "dev/3303/5700", MessageID: "m1", Quality: "ok", Unit: "Cel"}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(points)
	})
	return httptest.NewServer(mux)
}

func TestExportThings(t *testing.T) {
	is := is.New(t)
	srv := stubPlan1050(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	specs, err := svc.ExportThings(ctxWithToken(), "t1")
	is.NoErr(err)
	is.Equal(1, len(specs))
	is.Equal("Alt", specs[0].AlternativeName)
	is.Equal("Sub", specs[0].SubType)
	specs, err = svc.ExportThings(ctxWithToken(), "empty")
	is.NoErr(err)
	is.Equal(0, len(specs))
}

func TestGetRelationsGeneric(t *testing.T) {
	is := is.New(t)
	srv := stubPlan1050(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	rels, err := svc.GetRelations(ctxWithToken(), "t1", "bin-1")
	is.NoErr(err)
	is.Equal(2, len(rels))
	is.Equal("partOf", rels[0].Name)
	is.True(rels[0].Target != nil)
	is.Equal("gh-1", rels[0].Target.ThingID)
	// Broken reference stays visible with nil target.
	is.Equal("monitors", rels[1].Name)
	is.True(rels[1].Target == nil)
	_, err = svc.GetRelations(ctxWithToken(), "t1", "missing")
	is.True(errors.Is(err, client.ErrNotFound))
	_, err = svc.GetRelations(ctxWithToken(), "t1", "bad/id")
	is.True(err != nil)
}

func TestPreviewThing(t *testing.T) {
	is := is.New(t)
	srv := stubPlan1050(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	cfg, err := svc.PreviewThing(ctxWithToken(), "t1", "bin-1", ObjectSpec{ThingID: "bin-1", Name: "Ny", AlternativeName: "Alt", SubType: "Sub", TemplateID: "wastebin", TemplateVersion: "v1", Location: &Location{Type: "Point", Coordinates: json.RawMessage(`[18,59]`)}})
	is.NoErr(err)
	is.Equal("Alt", cfg.AlternativeName)
	is.Equal(1, len(cfg.Recipes))
	_, err = svc.PreviewThing(ctxWithToken(), "t1", "bin-1", ObjectSpec{ThingID: "bin-1"})
	is.True(err != nil)
	var be *client.BackendError
	is.True(errors.As(err, &be))
	is.True(errors.Is(err, client.ErrBadRequest))
}

func TestPreviewUpgradeAndUpgrade(t *testing.T) {
	is := is.New(t)
	srv := stubPlan1050(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	res, err := svc.PreviewUpgrade(ctxWithToken(), "t1", "bin-1", "wastebin", "v2", "", "")
	is.NoErr(err)
	is.Equal(1, len(res.Diff.AddedProps))
	is.Equal(1, len(res.Conflicts))
	res, err = svc.UpgradeThing(ctxWithToken(), "t1", "bin-1", "wastebin", "v2", "", "", false, 2)
	is.NoErr(err)
	is.Equal(0, len(res.Conflicts))
	_, err = svc.UpgradeThing(ctxWithToken(), "t1", "bin-1", "wastebin", "v2", "", "", false, 9)
	is.True(errors.Is(err, client.ErrConflict))
}

func TestGetTree(t *testing.T) {
	is := is.New(t)
	srv := stubPlan1050(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	node, err := svc.GetTree(ctxWithToken(), "t1", "gh-1", 1)
	is.NoErr(err)
	is.Equal("gh-1", node.Thing.ThingID)
	is.Equal(1, len(node.Children))
	_, err = svc.GetTree(ctxWithToken(), "t1", "gh-1", -1)
	is.True(err != nil)
}

func TestHistoryProvenance(t *testing.T) {
	is := is.New(t)
	srv := stubPlan1050(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	points, err := svc.GetHistory(ctxWithToken(), "t1", "bin-1", "", time.Time{}, time.Time{}, 0)
	is.NoErr(err)
	is.Equal(1, len(points))
	is.Equal("dev/3303/5700", points[0].Source)
	is.Equal("m1", points[0].MessageID)
	is.Equal("Cel", points[0].Unit)
}

func TestThingIDValidation(t *testing.T) {
	is := is.New(t)
	srv := stubPlan1050(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	_, err := svc.GetThing(ctxWithToken(), "t1", "")
	is.True(err != nil)
	_, err = svc.GetThing(ctxWithToken(), "t1", "a/b")
	is.True(err != nil)
	_, err = svc.GetRelations(ctxWithToken(), "t1", "")
	is.True(err != nil)
}
