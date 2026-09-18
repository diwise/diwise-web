package thingsv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestThingsV2SensorsDialogListsBindableInputs(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SensorsDialog(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/sensors?tenant=t1", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "thing-v2-sensors-dialog"))
	// distance har signaler (bindningsbar), level saknar (visas ej).
	is.True(strings.Contains(body, "Avstånd"))
	is.True(strings.Contains(body, "milesight:79"))
	is.True(strings.Contains(body, "Namn milesight:79"))
}

func TestThingsV2SensorSearchListsCandidates(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SensorSearch(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/sensors/search?tenant=t1&revision=2&input=distance&query=tunn", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	is.True(strings.Contains(body, "milesight:80"))
	is.True(strings.Contains(body, `name="deviceId"`))
	is.True(strings.Contains(body, `type="submit"`))
}

func TestThingsV2SensorSearchEmptyQueryRendersNothing(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SensorSearch(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodGet, "/components/things-v2/bin-1/sensors/search?tenant=t1&revision=2&input=distance", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(!strings.Contains(rec.Body.String(), "milesight:80"))
}

func TestThingsV2SetSensorRedirectsToDetails(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SetSensor(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/bin-1/sensors?tenant=t1", strings.NewReader("input=distance&deviceId=milesight%3A80&revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}

func TestThingsV2SetSensorRejectsMissingInput(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2SetSensor(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/bin-1/sensors?tenant=t1", strings.NewReader("deviceId=milesight%3A80&revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}

func TestThingsV2UnbindSensorRedirectsToDetails(t *testing.T) {
	is := is.New(t)

	svc, done := stubThingsV2(t)
	defer done()

	handler := NewThingsV2UnbindSensor(context.Background(), testLocaleBundle(), nil, &testThingsV2App{svc: svc})

	req := httptest.NewRequest(http.MethodPost, "/components/things-v2/bin-1/sensors/unbind?tenant=t1", strings.NewReader("input=distance&revision=2"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "bin-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("/things-v2/bin-1?tenant=t1", rec.Header().Get("HX-Redirect"))
}
