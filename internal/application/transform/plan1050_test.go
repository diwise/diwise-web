package transform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"github.com/matryer/is"
)

func ctxWithToken() context.Context {
	return auth.WithToken(context.Background(), "test-token")
}

// TestMatchVersionFieldsRoundtrip guards A-baseline thingTypeVersion/
// variantId/variantVersion (additive, omitempty so old backends ignore).
func TestMatchVersionFieldsRoundtrip(t *testing.T) {
	is := is.New(t)
	m := Match{Kind: "thing", Event: "things.v1.values", Type: "wastebin", ThingTypeVersion: "v2", VariantID: "160L", VariantVersion: "v1", Tenant: "t1"}
	raw, err := json.Marshal(m)
	is.NoErr(err)
	for _, k := range []string{"thingTypeVersion", "variantId", "variantVersion"} {
		is.True(strings.Contains(string(raw), `"`+k+`"`))
	}
	var back Match
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("v2", back.ThingTypeVersion)
	is.Equal("160L", back.VariantID)
	is.Equal("v1", back.VariantVersion)
	// Empty versions omit keys (old backend ignores).
	empty, err := json.Marshal(Match{Kind: "thing"})
	is.NoErr(err)
	is.True(!strings.Contains(string(empty), "thingTypeVersion"))
}

// TestSourceInlineFlat ensures Source embeds flat (no "Source" wrapper key).
func TestSourceInlineFlat(t *testing.T) {
	is := is.New(t)
	p := Property{Target: "name", Type: "Text", Field: "name"}
	raw, err := json.Marshal(p)
	is.NoErr(err)
	is.True(strings.Contains(string(raw), `"field":"name"`))
	is.True(!strings.Contains(string(raw), `"Source"`))
	is.True(!strings.Contains(string(raw), `"inline"`))
	var back Property
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("name", back.Field)
}

// TestPreviewEventRawMessageRoundtrip guards no field loss via RawMessage,
// including delta/snapshot with per-value time/unit/quality, removed[],
// previousToId (move) and bootstrap/lifecycle underlay: exact bytes preserved.
func TestPreviewEventRawMessageRoundtrip(t *testing.T) {
	is := is.New(t)
	msgs := []string{
		`{"thingId":"bin-1","values":{"fillRate":{"value":42.5,"quality":"ok","observedAt":"2026-09-18T10:00:00Z","unit":"%"}}}`,
		// B delta+snapshot: delta is this update, snapshot is producer
		// current-state (per-value time/unit/quality preserved).
		`{"thingId":"n1","tenant":"t1","thingType":"neutral","thingTypeVersion":"2","configRevision":7,"delta":[{"propertyId":"reading","value":0,"quality":"ok","observedAt":"2026-10-01T12:02:01Z"}],"snapshot":[{"propertyId":"reading","value":0,"quality":"ok","observedAt":"2026-10-01T12:02:01Z"},{"propertyId":"humidity","value":50,"quality":"uncertain","observedAt":"2026-10-01T12:00:00Z"}],"removed":["humidity.old"]}`,
		// Relations move with previousToId + bootstrap lifecycle underlay.
		`{"fromId":"room-a","toId":"building-b","previousToId":"building-a","tenant":"t1","type":"partOf","revision":9,"changedAt":"2026-10-01T12:00:00Z","fromThingType":"neutral","fromThingTypeVersion":"2"}`,
		`{"thingId":"n1","tenant":"t1","thingType":"neutral","thingTypeVersion":"2","configRevision":7,"kind":"created","bootstrap":true,"thing":{"thingId":"n1"}}`,
	}
	for _, msg := range msgs {
		ev, err := NewPreviewEvent("things.v1.values", msg, "")
		is.NoErr(err)
		is.Equal(msg, string(ev.Message))
		raw, err := json.Marshal(ev)
		is.NoErr(err)
		var back PreviewEvent
		is.NoErr(json.Unmarshal(raw, &back))
		is.Equal(msg, string(back.Message))
		// No field loss: removed/previousToId/bootstrap survive RawMessage.
		for _, k := range []string{"removed", "previousToId", "bootstrap", "snapshot", "delta"} {
			if strings.Contains(msg, `"`+k+`"`) {
				is.True(strings.Contains(string(back.Message), `"`+k+`"`))
			}
		}
	}
}

// TestNewPreviewEventSingleDoc enforces exactly one JSON document.
func TestNewPreviewEventSingleDoc(t *testing.T) {
	is := is.New(t)
	_, err := NewPreviewEvent("", `{}`, "")
	is.True(err != nil)
	_, err = NewPreviewEvent("things.v1.values", ``, "")
	is.True(err != nil)
	_, err = NewPreviewEvent("things.v1.values", `{invalid}`, "")
	is.True(err != nil)
	_, err = NewPreviewEvent("things.v1.values", `{} {}`, "")
	is.True(err != nil)
	ev, err := NewPreviewEvent("things.v1.values", `{"a":1}`, "")
	is.NoErr(err)
	is.True(len(ev.Message) > 0)
}

// TestPreviewExtendedResponse decodes E wire (preview.go-facit):
// competing/skipped + ruleId/ruleRevision/snapshotVersion +
// effectiveContext/removeAttributes. G maps competing→matches/diagnostics
// (A-rekommendation Våg3) without loss; Diagnostic fields survive.
func TestPreviewExtendedResponse(t *testing.T) {
	is := is.New(t)
	// E wire: competing + skipped(Diagnostic) + identity + entity context.
	raw := `{"matched":true,"entities":[{"id":"urn:ngsi-ld:Room:r1","type":"Room","properties":{"name":{"value":"Rum"}},"context":"https://example.com/ctx","effectiveContext":"https://example.com/ctx","removeAttributes":["oldAttr"],"operation":"merge"}],"observations":1,"ruleId":"11111111-1111-1111-1111-111111111111","ruleRevision":3,"snapshotVersion":7,"competing":[{"index":0,"ruleId":"11111111-1111-1111-1111-111111111111","revision":3,"snapshotVersion":7,"priority":1,"specificity":5,"winner":true,"reason":"winner"}],"skipped":[{"code":"required-missing","ruleId":"11111111-1111-1111-1111-111111111111","ruleRevision":3,"snapshotVersion":7,"entity":"primary","attribute":"reading","expression":"values.missing.value","tenant":"t1","thingId":"n1","configRevision":7,"detail":"required source missing"}]}`
	var res PreviewResult
	is.NoErr(json.Unmarshal([]byte(raw), &res))
	// Normalize as service does (competing→matches/diagnostics).
	res = normalizePreviewResult(res)
	is.True(res.Matched)
	is.Equal(1, len(res.Entities))
	is.Equal("urn:ngsi-ld:Room:r1", res.Entities[0].ID)
	is.Equal([]string{"oldAttr"}, res.Entities[0].RemoveAttributes)
	is.Equal("https://example.com/ctx", res.Entities[0].EffectiveContext)
	is.Equal("11111111-1111-1111-1111-111111111111", res.RuleID)
	is.Equal(int64(3), res.RuleRevision)
	is.Equal(int64(7), res.SnapshotVersion)
	// E competing preserved verbatim...
	is.Equal(1, len(res.Competing))
	is.True(res.Competing[0].Winner)
	is.Equal(0, res.Competing[0].Index)
	is.Equal("winner", res.Competing[0].Reason)
	// ...and mapped to GUI views.
	is.Equal(1, len(res.Matches))
	is.True(res.Matches[0].Winner)
	is.Equal(1, len(res.Diagnostics))
	is.True(res.Diagnostics[0].Winner)
	// E skipped Diagnostic without loss (all D fields).
	is.Equal(1, len(res.Skipped))
	sk := res.Skipped[0]
	is.Equal("required-missing", string(sk.Code))
	is.Equal("primary", sk.Entity)
	is.Equal("reading", sk.Attribute)
	is.Equal("values.missing.value", sk.Expression)
	is.Equal("t1", sk.Tenant)
	is.Equal("n1", sk.ThingID)
	is.Equal(int64(7), sk.ConfigRevision)
	is.Equal(int64(3), sk.RuleRevision)
	// Legacy tolerant: old matches/skipped/diagnostics shapes still decode.
	var legacy PreviewResult
	is.NoErr(json.Unmarshal([]byte(`{"matched":true,"entities":[],"observations":1,"matches":[{"ruleId":"other","reason":"lower priority"}],"skipped":[{"entity":"inverse","target":"links","reason":"missing target role"}],"diagnostics":[{"index":0,"ruleId":"r1","revision":2,"snapshotVersion":5,"winner":true,"reason":"winner"}]}`), &legacy))
	legacy = normalizePreviewResult(legacy)
	is.Equal(1, len(legacy.Matches))
	is.Equal(1, len(legacy.Competing))
	is.Equal("other", legacy.Competing[0].RuleID)
	is.True(strings.Contains(legacy.Skipped[0].Reason+legacy.Skipped[0].Target+legacy.Skipped[0].Entity, "inverse") || legacy.Skipped[0].Entity == "inverse")
	// Old minimal shape still decodes (new fields zero).
	var old PreviewResult
	is.NoErr(json.Unmarshal([]byte(`{"matched":false,"entities":[],"observations":0}`), &old))
	old = normalizePreviewResult(old)
	is.True(!old.Matched)
	is.Equal(0, len(old.Matches))
	is.Equal(0, len(old.Competing))
}

// TestTargetRolesAndDeltaSnapshotSyntax guards målroller + delta/snapshot
// paths + citerade vägar/index without loss (D delta/snapshot-syntax).
func TestTargetRolesAndDeltaSnapshotSyntax(t *testing.T) {
	is := is.New(t)
	rule := Rule{
		Match: Match{Kind: "thing", Event: "things.v1.values", Type: "neutral", ThingTypeVersion: "2", VariantID: "v", VariantVersion: "3"},
		Entities: []Entity{{
			Name: "main", ID: "urn:example:{{id}}", Type: "Neutral",
			RemoveAttributes: []string{"old"},
			Properties: []Property{
				{Target: "reading", Type: "Number", Field: "snapshot.values.temperature.value", Required: true},
				{Target: "deltaReading", Type: "Number", Field: "delta.temperature.value"},
				{Target: "quoted", Type: "Number", Value: `{{field:snapshot.values."a.b".value}}`},
				{Target: "indexed", Type: "Number", Value: `{{snapshot.values.a[0].value}}`},
			},
		}, {
			Name: "inverse", ID: "urn:example:inverse:{{id}}", Type: "Other", Properties: []Property{},
		}},
	}
	raw, err := json.Marshal(rule)
	is.NoErr(err)
	for _, k := range []string{`"name":"main"`, `"removeAttributes":["old"]`, "snapshot.values", "delta.temperature", `a.b`, "thingTypeVersion", "variantId", "variantVersion"} {
		is.True(strings.Contains(string(raw), k))
	}
	var back Rule
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("main", back.Entities[0].Name)
	is.Equal("inverse", back.Entities[1].Name)
	is.Equal([]string{"old"}, back.Entities[0].RemoveAttributes)
	is.Equal("snapshot.values.temperature.value", back.Entities[0].Properties[0].Field)
}

// TestCompatibilityRoundtrip guards D wire (register.go + compat_d_test.go
// facit): CompatRequest{template,sources,rules,tenant,thingId,configRevision}
// and CompatResult{compatible,diagnostics} without loss; legacy flat shape
// stays readable and Issues derives from Diagnostics for H/I templ.
func TestCompatibilityRoundtrip(t *testing.T) {
	is := is.New(t)
	// D request wire (new shape).
	req := CompatibilityRequest{
		Template: TemplateVersion{ThingType: "wastebin", ThingTypeVersion: "v2", VariantID: "160L", VariantVersion: "v1"},
		Sources: SourceContract{
			Properties: []PropertyContract{{ID: "fillRate", Unit: "%"}},
			Relations:  []RelationSlot{{Name: "partOf", AllowedTargets: []string{"greenhouse"}, Max: 1}},
		},
		Rules: []Rule{{
			Match:    Match{Kind: "thing", Event: "things.v1.values", Type: "wastebin", ThingTypeVersion: "v2"},
			Entities: []Entity{{Name: "main", ID: "urn:ngsi-ld:Waste:{{id}}", Type: "WasteContainer", Properties: []Property{{Target: "fillRate", Type: "Number", Field: "snapshot.fillRate.value"}}}},
		}},
		Tenant: "t1", ThingID: "bin-1", ConfigRevision: 7,
	}
	raw, err := json.Marshal(req)
	is.NoErr(err)
	// D keys on wire, no old flat keys (strict backend would reject them).
	for _, k := range []string{`"template"`, `"sources"`, `"rules"`, `"thingType"`, `"properties"`, `"relations"`, `"tenant"`, `"thingId"`, `"configRevision"`} {
		is.True(strings.Contains(string(raw), k))
	}
	is.True(!strings.Contains(string(raw), `"templateId"`))
	is.True(!strings.Contains(string(raw), `"templateVersion"`))
	var back CompatibilityRequest
	is.NoErr(json.Unmarshal(raw, &back))
	is.Equal("wastebin", back.Template.ThingType)
	is.Equal("v2", back.Template.ThingTypeVersion)
	is.Equal("160L", back.Template.VariantID)
	is.Equal(1, len(back.Rules))
	is.Equal("main", back.Rules[0].Entities[0].Name)
	is.Equal("t1", back.Tenant)
	is.Equal("bin-1", back.ThingID)
	is.Equal(int64(7), back.ConfigRevision)
	// Legacy flat construction still compiles and maps to D wire (H/I).
	legacyReq := CompatibilityRequest{TemplateID: "wastebin", TemplateVersion: "v2", VariantID: "160L", VariantVersion: "v1", Rules: back.Rules}
	legacyRaw, err := json.Marshal(legacyReq)
	is.NoErr(err)
	is.True(strings.Contains(string(legacyRaw), `"thingType":"wastebin"`))
	var legacyBack CompatibilityRequest
	is.NoErr(json.Unmarshal(legacyRaw, &legacyBack))
	is.Equal("wastebin", legacyBack.TemplateID)
	is.Equal("wastebin", legacyBack.Template.ThingType)
	// Old flat JSON still decodes (H/I stub compat).
	var flatBack CompatibilityRequest
	is.NoErr(json.Unmarshal([]byte(`{"templateId":"wastebin","templateVersion":"v2","variantId":"160L","variantVersion":"v1","rules":[{"match":{"kind":"thing"},"entities":[]}]}`), &flatBack))
	is.Equal("wastebin", flatBack.Template.ThingType)
	is.Equal("wastebin", flatBack.TemplateID)
	// D response wire.
	resRaw := `{"compatible":false,"diagnostics":[{"code":"required-missing","ruleId":"r1","ruleRevision":2,"snapshotVersion":5,"entity":"main","attribute":"reading","expression":"values.missing.value","tenant":"t1","thingId":"bin-1","configRevision":7,"detail":"required source missing"}]}`
	var res CompatibilityResult
	is.NoErr(json.Unmarshal([]byte(resRaw), &res))
	res = normalizeCompatibilityResult(res)
	is.True(!res.Compatible)
	is.Equal(1, len(res.Diagnostics))
	d := res.Diagnostics[0]
	is.Equal("required-missing", string(d.Code))
	is.Equal("main", d.Entity)
	is.Equal("reading", d.Attribute)
	is.Equal("values.missing.value", d.Expression)
	is.Equal("t1", d.Tenant)
	is.Equal("bin-1", d.ThingID)
	is.Equal(int64(7), d.ConfigRevision)
	// Issues derived for H/I templ (no loss of displayed values).
	is.Equal(1, len(res.Issues))
	is.True(strings.Contains(res.Issues[0].Message, "required source missing"))
	is.Equal("main", res.Issues[0].TargetRole)
	// Old issues/warnings shape still decodes and derives diagnostics.
	var oldRes CompatibilityResult
	is.NoErr(json.Unmarshal([]byte(`{"compatible":false,"issues":[{"ruleId":"r1","targetRole":"main","attribute":"reading","expression":"value","message":"unknown property","severity":"error"}],"warnings":[{"message":"dynamic metadata check","severity":"warning"}]}`), &oldRes))
	oldRes = normalizeCompatibilityResult(oldRes)
	is.Equal(1, len(oldRes.Issues))
	is.Equal(1, len(oldRes.Warnings))
	is.True(len(oldRes.Diagnostics) >= 2)
}

// stubCompat mimics real D/E endpoints (register.go + preview.go facit):
// POST /models/compat takes {template,sources,rules} and returns
// {compatible,diagnostics} (strict: unknown fields rejected like backend
// decodeJSONBody would, empty rules rejected); POST /models/preview returns
// E wire {competing,skipped,ruleId/ruleRevision/snapshotVersion,
// effectiveContext/removeAttributes}.
func stubCompat(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/models/compat", func(w http.ResponseWriter, r *http.Request) {
		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
			return
		}
		// D strict: template object + rules required, unknown top keys rejected.
		tmpl, _ := raw["template"].(map[string]any)
		rules, _ := raw["rules"].([]any)
		if tmpl == nil || tmpl["thingType"] == nil || len(rules) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "template.thingType and rules are required"})
			return
		}
		for k := range raw {
			switch k {
			case "template", "sources", "rules", "tenant", "thingId", "configRevision":
			default:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "unknown field " + k})
				return
			}
		}
		if tmpl["thingType"] == "missing" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CompatibilityResult{Compatible: true, Diagnostics: []Diagnostic{}})
	})
	mux.HandleFunc("/models/preview", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Rule  Rule         `json:"rule"`
			Event PreviewEvent `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Event.Kind) == 0 || len(req.Event.Message) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "event is required"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"matched": true,
			"entities": []any{map[string]any{
				"id": "urn:ngsi-ld:Room:r1", "type": "Room",
				"properties": map[string]any{"name": map[string]any{"value": "Rum"}},
				"context":    "https://example.com/ctx", "effectiveContext": "https://example.com/ctx",
				"operation": "merge",
			}},
			"observations": 1, "ruleId": "r1", "ruleRevision": 2, "snapshotVersion": 5,
			"competing": []any{map[string]any{
				"index": 0, "ruleId": "r1", "revision": 2, "snapshotVersion": 5,
				"priority": 1, "specificity": 3, "winner": true, "reason": "winner",
			}},
			"skipped": []any{},
		})
	})
	mux.HandleFunc("/models/validate", func(w http.ResponseWriter, r *http.Request) {
		var rule Rule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil || len(rule.Entities) == 0 {
			// 422 path for semantic errors (distinct from 400 syntax).
			if len(rule.Entities) == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rule must declare at least one entity"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"valid": true})
	})
	return httptest.NewServer(mux)
}

func TestCheckCompatibilityStub(t *testing.T) {
	is := is.New(t)
	srv := stubCompat(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	// D request wire (new shape).
	res, err := svc.CheckCompatibility(ctxWithToken(), "t1", CompatibilityRequest{
		Template: TemplateVersion{ThingType: "wastebin", ThingTypeVersion: "v2"},
		Sources:  SourceContract{Properties: []PropertyContract{{ID: "fillRate"}}},
		Rules:    []Rule{testRule()},
	})
	is.NoErr(err)
	is.True(res.Compatible)
	is.Equal(0, len(res.Diagnostics))
	// Legacy flat construction still sends D wire (H/I compat).
	res, err = svc.CheckCompatibility(ctxWithToken(), "t1", CompatibilityRequest{TemplateID: "wastebin", TemplateVersion: "v2", Rules: []Rule{testRule()}})
	is.NoErr(err)
	is.True(res.Compatible)
	// Wildcard version allowed per D (only thingType required).
	res, err = svc.CheckCompatibility(ctxWithToken(), "t1", CompatibilityRequest{Template: TemplateVersion{ThingType: "wastebin"}, Rules: []Rule{testRule()}})
	is.NoErr(err)
	is.True(res.Compatible)
	_, err = svc.CheckCompatibility(ctxWithToken(), "t1", CompatibilityRequest{})
	is.True(err != nil)
	_, err = svc.CheckCompatibility(ctxWithToken(), "t1", CompatibilityRequest{Template: TemplateVersion{ThingType: "missing"}, Rules: []Rule{testRule()}})
	is.True(errors.Is(err, client.ErrNotFound))
	_, err = svc.CheckCompatibility(ctxWithToken(), "t1", CompatibilityRequest{Template: TemplateVersion{}, Rules: []Rule{testRule()}})
	is.True(err != nil)
	// D wire check: flat old keys never hit backend (strict would 400).
	raw, err := json.Marshal(CompatibilityRequest{TemplateID: "wastebin", TemplateVersion: "v2", Rules: []Rule{testRule()}})
	is.NoErr(err)
	is.True(strings.Contains(string(raw), `"thingType":"wastebin"`))
	is.True(!strings.Contains(string(raw), `"templateId"`))
}

func TestPreviewExtendedStub(t *testing.T) {
	is := is.New(t)
	srv := stubCompat(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	ev, err := NewPreviewEvent("things.v1.values", `{"thingId":"r1","tenant":"t1","thingType":"room","thingTypeVersion":"1","configRevision":2,"delta":[{"propertyId":"name","value":1,"quality":"ok","observedAt":"2026-10-01T12:00:00Z"}],"snapshot":[{"propertyId":"name","value":1,"quality":"ok","observedAt":"2026-10-01T12:00:00Z"}]}`, "")
	is.NoErr(err)
	res, err := svc.PreviewRule(ctxWithToken(), "t1", testRule(), ev)
	is.NoErr(err)
	is.True(res.Matched)
	is.Equal("r1", res.RuleID)
	is.Equal(int64(2), res.RuleRevision)
	is.Equal(int64(5), res.SnapshotVersion)
	// E competing preserved + mapped (A-rekommendation).
	is.Equal(1, len(res.Competing))
	is.True(res.Competing[0].Winner)
	is.Equal(1, len(res.Matches))
	is.True(res.Matches[0].Winner)
	is.Equal(1, len(res.Diagnostics))
	is.Equal("https://example.com/ctx", res.Entities[0].EffectiveContext)
	is.Equal("merge", res.Entities[0].Operation)
}

func TestValidate422(t *testing.T) {
	is := is.New(t)
	srv := stubCompat(t)
	defer srv.Close()
	svc := NewService(&client.Client{}, srv.URL)
	err := svc.ValidateRule(ctxWithToken(), "t1", Rule{Match: Match{Kind: "thing"}})
	var verr *ValidationError
	is.True(errors.As(err, &verr))
	is.Equal(422, verr.Status)
	is.True(errors.Is(err, client.ErrUnprocessable))
}

// TestCompatDParity mirrors D compat_d_test.go cases against D wire:
// positive generic, no-match version filter, required-missing with
// role/attribute/thingId/configRevision, and version/variant wildcards.
// Proves G sends {template,sources,rules} and reads {diagnostics} without loss.
func TestCompatDParity(t *testing.T) {
	is := is.New(t)
	// Positive: versioned template + sources, rule otherwise valid.
	positive := `{"compatible":true,"diagnostics":[]}`
	var pos CompatibilityResult
	is.NoErr(json.Unmarshal([]byte(positive), &pos))
	pos = normalizeCompatibilityResult(pos)
	is.True(pos.Compatible)
	is.Equal(0, len(pos.Diagnostics))
	// No-match: version filter mismatch carries code + tenant.
	noMatch := `{"compatible":false,"diagnostics":[{"code":"no-match","tenant":"tenant-a","detail":"no candidate rule matches template"}]}`
	var nm CompatibilityResult
	is.NoErr(json.Unmarshal([]byte(noMatch), &nm))
	nm = normalizeCompatibilityResult(nm)
	is.True(!nm.Compatible)
	is.Equal("no-match", string(nm.Diagnostics[0].Code))
	is.Equal("tenant-a", nm.Diagnostics[0].Tenant)
	is.Equal(1, len(nm.Issues))
	// Required-missing with entity/attribute/thingId/configRevision.
	reqMissing := `{"compatible":false,"diagnostics":[{"code":"required-missing","entity":"primary","attribute":"reading","tenant":"tenant-a","thingId":"thing-1","configRevision":7,"detail":"required source missing"}]}`
	var rm CompatibilityResult
	is.NoErr(json.Unmarshal([]byte(reqMissing), &rm))
	rm = normalizeCompatibilityResult(rm)
	is.True(!rm.Compatible)
	d := rm.Diagnostics[0]
	is.Equal("primary", d.Entity)
	is.Equal("reading", d.Attribute)
	is.Equal("thing-1", d.ThingID)
	is.Equal(int64(7), d.ConfigRevision)
	is.Equal("primary", rm.Issues[0].TargetRole)
	// Request wire: version/variant exact + wildcard roundtrip.
	for _, req := range []CompatibilityRequest{
		{Template: TemplateVersion{ThingType: "generic-template", ThingTypeVersion: "5"}, Rules: []Rule{testRule()}},
		{Template: TemplateVersion{ThingType: "generic-template", VariantID: "variant-a"}, Rules: []Rule{testRule()}},
	} {
		raw, err := json.Marshal(req)
		is.NoErr(err)
		var back CompatibilityRequest
		is.NoErr(json.Unmarshal(raw, &back))
		is.Equal("generic-template", back.Template.ThingType)
	}
	// Old G stub without competing still maps (H/I smoke): covered in preview test.
}

// TestPreviewEParity mirrors E preview_e_test.go wire: delta/snapshot
// parity fields, competing winner, precise skipped (required-missing with
// entity/attribute), removeAttributes/effectiveContext, version matching.
// Proves G maps competing→matches/diagnostics and preserves Diagnostic.
func TestPreviewEParity(t *testing.T) {
	is := is.New(t)
	// E competing winner + skipped precise (matchad men undertryckt).
	raw := `{"matched":true,"entities":[],"observations":1,"ruleId":"","ruleRevision":0,"snapshotVersion":0,"competing":[{"index":0,"ruleId":"","revision":0,"snapshotVersion":0,"priority":0,"specificity":5,"winner":true,"reason":"winner"}],"skipped":[{"code":"required-missing","entity":"primary","attribute":"missing","detail":"required source missing"}],"skippedReason":"matched rule produced no entities (required source missing or suppressed)"}`
	var res PreviewResult
	is.NoErr(json.Unmarshal([]byte(raw), &res))
	res = normalizePreviewResult(res)
	is.True(res.Matched)
	is.Equal(0, len(res.Entities))
	is.Equal(1, len(res.Competing))
	is.True(res.Competing[0].Winner)
	is.Equal(1, len(res.Matches))
	is.True(res.Matches[0].Winner)
	is.Equal(1, len(res.Skipped))
	is.Equal("required-missing", string(res.Skipped[0].Code))
	is.Equal("primary", res.Skipped[0].Entity)
	is.Equal("missing", res.Skipped[0].Attribute)
	is.True(res.SkippedReason != "")
	// Remove + effective context per operation (E relations remove).
	raw2 := `{"matched":true,"entities":[{"id":"urn:ngsi-ld:Generic:thing-2","type":"Generic","properties":{},"context":"https://example.invalid/ctx.jsonld","effectiveContext":"https://example.invalid/ctx.jsonld","operation":"remove","removeAttributes":["stale"]}],"observations":1,"competing":[],"skipped":[]}`
	var res2 PreviewResult
	is.NoErr(json.Unmarshal([]byte(raw2), &res2))
	res2 = normalizePreviewResult(res2)
	is.Equal("remove", res2.Entities[0].Operation)
	is.Equal([]string{"stale"}, res2.Entities[0].RemoveAttributes)
	is.Equal("https://example.invalid/ctx.jsonld", res2.Entities[0].EffectiveContext)
}
