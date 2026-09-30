package rules

import (
	"context"
	"encoding/json"
	"github.com/diwise/diwise-web/internal/presentation/api/auth"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	featurerules "github.com/diwise/diwise-web/internal/presentation/web/components/features/rules"
	"github.com/matryer/is"
)

// Fixtures ska vara giltig JSON per kind (en per kind/event-variant).
func TestFixturesAreValidJSON(t *testing.T) {
	is := is.New(t)

	seen := map[string]bool{}
	for _, f := range featurerules.Fixtures() {
		var v any
		if err := json.Unmarshal([]byte(f.Message), &v); err != nil {
			t.Fatalf("fixture %q: invalid JSON: %v", f.Name, err)
		}
		if f.Kind == "" || f.Label == "" {
			t.Fatalf("fixture %q: kind/label required", f.Name)
		}
		seen[f.Kind+"/"+f.Name] = true
	}

	for _, want := range []string{
		"measurement/measurement",
		"things.v1.values/values",
		"things.v1.relations/relations-set",
		"things.v1.relations/relations-removed",
		"things.v1.lifecycle/lifecycle-created",
		"things.v1.lifecycle/lifecycle-deleted",
	} {
		is.True(seen[want])
	}
}

func previewForm(extra url.Values) url.Values {
	form := url.Values{
		"tenant": {"t1"}, "kind": {"thing"}, "event": {"things.v1.values"}, "type": {"room"},
		"e0_id": {"urn:ngsi-ld:Room:{{id}}"}, "e0_type": {"Room"},
		"e0_p0_target": {"name"}, "e0_p0_type": {"Text"},
		"e0_p0_sourceKind": {"field"}, "e0_p0_sourceValue": {"name"},
		"previewKind":       {"things.v1.values"},
		"previewMessage":    {`{"thingId":"r1"}`},
		"previewSensorType": {""},
	}
	for k, v := range extra {
		form[k] = v
	}
	return form
}

func postFragment(t *testing.T, handler http.HandlerFunc, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(form.Encode()))
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestValidateFragmentOKAndError(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewValidateFragment(context.Background(), testLocaleBundle(), testAssets(), app)

	rec := postFragment(t, handler, previewForm(nil))
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "rules_valid"))

	bad := previewForm(nil)
	for k := range bad {
		if strings.HasPrefix(k, "e0_") {
			delete(bad, k)
		}
	}
	rec = postFragment(t, handler, bad)
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "rule must declare"))
}

func TestPreviewFragmentMatchedAndBadMessage(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewPreviewFragment(context.Background(), testLocaleBundle(), testAssets(), app)

	rec := postFragment(t, handler, previewForm(nil))
	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "rules_matched"))
	is.True(strings.Contains(body, "urn:ngsi-ld:Room:r1"))
	is.True(strings.Contains(body, "merge"))

	// Trasig JSON nekas med formulärfel (aldrig backend-anrop).
	rec = postFragment(t, handler, previewForm(url.Values{"previewMessage": {"{ojsan"}}))
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "exactly one JSON document"))

	// Två dokument nekas likaså.
	rec = postFragment(t, handler, previewForm(url.Values{"previewMessage": {`{} {}`}}))
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "exactly one JSON document"))
}

func TestFixtureFragment(t *testing.T) {
	is := is.New(t)
	app, done := testApp(t)
	defer done()

	handler := NewFixtureFragment(context.Background(), testLocaleBundle(), testAssets(), app)

	// Formulärets fältnamn (det htmx skickar vid fixture-val).
	req := httptest.NewRequest(http.MethodGet, "/components/rules/fixture?previewFixture=lifecycle-deleted", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	var v any
	is.NoErr(json.Unmarshal(rec.Body.Bytes(), &v))

	// Fallback för direktlänkar.
	req = httptest.NewRequest(http.MethodGet, "/components/rules/fixture?name=values", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/components/rules/fixture?name=nope", nil)
	req = req.WithContext(auth.WithToken(req.Context(), "test-token"))
	req.Header.Set("HX-Request", "true")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}
