package things

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	appthings "github.com/diwise/diwise-web/internal/application/things"
	"github.com/diwise/diwise-web/internal/presentation/api/authz"
	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/diwise/frontend-toolkit/pkg/locale"
	"github.com/matryer/is"
)

func TestBuildThingUpdateFieldsMapsEditForm(t *testing.T) {
	is := is.New(t)

	form := url.Values{
		"name":            {"Container A"},
		"alternativeName": {"Station 17"},
		"description":     {"Updated description"},
		"organisation":    {"tenant-a"},
		"tags":            {"waste, downtown, waste"},
		"currentDevice":   {"70B3D57ED00627A1, 70B3D57ED00627A2, 70B3D57ED00627A1"},
		"latitude":        {"57.708870"},
		"longitude":       {"11.974560"},
		"maxl":            {"1.25"},
		"maxd":            {"2.50"},
		"angle":           {"3.75"},
		"offset":          {"4.50"},
	}

	fields, err := buildThingUpdateFields(context.Background(), &testThingsApp{
		thing: appthings.Thing{ID: "thing-1", ValidURNs: []string{"urn:1"}},
		validSensors: []appthings.SensorIdentifier{
			{SensorID: "70B3D57ED00627A1", DeviceID: "device-1"},
			{SensorID: "70B3D57ED00627A2", DeviceID: "device-2"},
		},
	}, "thing-1", form)

	is.NoErr(err)
	is.Equal("Container A", fields["name"])
	is.Equal("Station 17", fields["alternativeName"])
	is.Equal("Updated description", fields["description"])
	is.Equal("tenant-a", fields["tenant"])
	is.Equal([]string{"waste", "downtown"}, fields["tags"])
	is.Equal([]appthings.RefDevice{{DeviceID: "device-1"}, {DeviceID: "device-2"}}, fields["refDevices"])
	is.Equal(client.Location{Latitude: 57.70887, Longitude: 11.97456}, fields["location"])
	is.Equal(1.25, fields["maxl"])
	is.Equal(2.5, fields["maxd"])
	is.Equal(3.75, fields["angle"])
	is.Equal(4.5, fields["offset"])
}

func TestBuildThingUpdateFieldsHandlesRepeatedTagValues(t *testing.T) {
	is := is.New(t)

	form := url.Values{
		"tags": {"waste", "downtown", "waste"},
	}

	fields, err := buildThingUpdateFields(context.Background(), &testThingsApp{}, "thing-1", form)

	is.NoErr(err)
	is.Equal([]string{"waste", "downtown"}, fields["tags"])
}

func TestBuildThingUpdateFieldsAcceptsLegacyDeviceIDCurrentDevice(t *testing.T) {
	is := is.New(t)

	fields, err := buildThingUpdateFields(context.Background(), &testThingsApp{
		devices: map[string]devices.Device{
			"device-1": {DeviceID: "device-1", SensorID: "70B3D57ED00627A1"},
		},
	}, "thing-1", url.Values{
		"currentDevice": {"device-1"},
	})

	is.NoErr(err)
	is.Equal([]appthings.RefDevice{{DeviceID: "device-1"}}, fields["refDevices"])
}

func TestBuildThingUpdateFieldsRejectsUnknownConnectedSensor(t *testing.T) {
	is := is.New(t)

	_, err := buildThingUpdateFields(context.Background(), &testThingsApp{
		thing: appthings.Thing{ID: "thing-1", ValidURNs: []string{"urn:1"}},
	}, "thing-1", url.Values{
		"currentDevice": {"missing-sensor"},
	})

	is.True(err != nil)
}

func TestApplySubmittedThingDetailsFormKeepsSubmittedSensorValues(t *testing.T) {
	is := is.New(t)

	model := featuresthings.ThingDetailsPageViewModel{
		Thing: featuresthings.ThingViewModel{
			Name: "Original",
		},
		ConnectedSensors: []featuresthings.ConnectedSensorViewModel{{
			DeviceID: "device-1",
			Label:    "70B3D57ED00627A1",
		}},
	}

	applySubmittedThingDetailsForm(&model, url.Values{
		"name":            {"Updated"},
		"alternativeName": {"Alt"},
		"description":     {"Desc"},
		"organisation":    {"tenant-a"},
		"tags":            {"one, two"},
		"currentDevice":   {"missing-sensor"},
	})

	is.Equal("Updated", model.Thing.Name)
	is.Equal("Alt", model.Thing.AlternativeName)
	is.Equal("Desc", model.Thing.Description)
	is.Equal("tenant-a", model.Thing.Tenant)
	is.Equal([]string{"one", "two"}, model.Thing.Tags)
	is.Equal([]featuresthings.ConnectedSensorViewModel{{
		DeviceID: "missing-sensor",
		Label:    "missing-sensor",
	}}, model.ConnectedSensors)
}

func TestNewSaveThingDetailsPageReturnsToastForUnknownConnectedSensorHXRequest(t *testing.T) {
	is := is.New(t)

	app := &testThingsApp{
		thing: appthings.Thing{
			ID:        "thing-1",
			Name:      "Thing One",
			ValidURNs: []string{"urn:1"},
			Type:      "Building",
		},
	}
	handler := NewSaveThingDetailsPage(context.Background(), testLocaleBundle(), func(name string) frontendtoolkit.Asset {
		return testAsset(pathValue(name))
	}, app)

	form := url.Values{
		"name":          {"Thing One"},
		"currentDevice": {"missing-sensor"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things/thing-1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	req.SetPathValue("id", "thing-1")

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(body, "data-tui-toast"))
	is.True(strings.Contains(body, "Could not find connected sensor"))
	is.True(strings.Contains(body, "missing-sensor"))
	is.True(!app.updateCalled)
}

func TestNewSaveThingDetailsPageRendersEditPageWithToastForUnknownConnectedSensor(t *testing.T) {
	is := is.New(t)

	app := &testThingsApp{
		thing: appthings.Thing{
			ID:        "thing-1",
			Name:      "Thing One",
			ValidURNs: []string{"urn:1"},
			Type:      "Building",
		},
	}
	handler := NewSaveThingDetailsPage(context.Background(), testLocaleBundle(), func(name string) frontendtoolkit.Asset {
		return testAsset(pathValue(name))
	}, app)

	form := url.Values{
		"name":          {"Thing One"},
		"currentDevice": {"missing-sensor"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things/thing-1", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), authz.LoggedIn, "yes"))
	rec := httptest.NewRecorder()
	req.SetPathValue("id", "thing-1")

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(body, "id=\"thing-edit-toast\""))
	is.True(strings.Contains(body, "data-tui-toast"))
	is.True(strings.Contains(body, "Could not find connected sensor"))
	is.True(strings.Contains(body, "missing-sensor"))
	is.True(strings.Contains(body, "name=\"currentDevice\" value=\"missing-sensor\""))
	is.True(!app.updateCalled)
}

type testAsset string

func (a testAsset) Body() []byte          { return nil }
func (a testAsset) ContentLength() int    { return 0 }
func (a testAsset) ContentType() string   { return "text/plain" }
func (a testAsset) Path() string          { return string(a) }
func (a testAsset) SHA256() string        { return "" }

func pathValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "/"
	}
	return value
}

func TestAllowsMultipleConnectedSensorsByThingType(t *testing.T) {
	is := is.New(t)

	is.True(allowsMultipleConnectedSensors(appthings.Thing{Type: "Building"}))
	is.True(allowsMultipleConnectedSensors(appthings.Thing{Type: "Room"}))
	is.True(allowsMultipleConnectedSensors(appthings.Thing{Type: "Sewer"}))
	is.True(allowsMultipleConnectedSensors(appthings.Thing{Type: "Container", SubType: "Sandstorage"}))
	is.True(!allowsMultipleConnectedSensors(appthings.Thing{Type: "Container", SubType: "WasteContainer"}))
	is.True(!allowsMultipleConnectedSensors(appthings.Thing{Type: "Lifebuoy"}))
}

func TestNormalizeCSVListTrimsAndDeduplicates(t *testing.T) {
	is := is.New(t)

	values := normalizeCSVList(" one, two ,one,, three ")

	is.Equal([]string{"one", "two", "three"}, values)
}

func TestMeasurementQueryForBooleanLikeSeries(t *testing.T) {
	is := is.New(t)

	startTime := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 3, 16, 23, 59, 0, 0, time.UTC)

	query := measurementQuery("10351-0", startTime, endTime)

	is.Equal("true", query.Get("vb"))
	is.Equal("hour", query.Get("timeunit"))
	is.Equal("", query.Get("options"))
	is.Equal("10351/0", query.Get("n"))
}

func TestMeasurementQueryForPresenceSeries(t *testing.T) {
	is := is.New(t)

	startTime := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2026, 3, 16, 23, 59, 0, 0, time.UTC)

	query := measurementQuery("3302-0", startTime, endTime)

	is.Equal("true", query.Get("vb"))
	is.Equal("hour", query.Get("timeunit"))
	is.Equal("", query.Get("options"))
	is.Equal("3302/0", query.Get("n"))
}

func TestThingMeasurementChartConfigUsesLegacyPercentageScaleForFillingLevel(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/components/things/thing-1/measurements", nil)
	config := thingMeasurementChartConfig(req, noopLocalizer{}, "3435-2", appthings.Thing{})

	is.True(config.Options.Scales["y"].Min != nil)
	is.True(config.Options.Scales["y"].Max != nil)
	is.True(config.Options.Scales["y"].Ticks != nil)
	is.True(config.Options.Scales["y"].Ticks.StepSize != nil)
	is.Equal(0.0, *config.Options.Scales["y"].Min)
	is.Equal(100.0, *config.Options.Scales["y"].Max)
	is.Equal(10.0, *config.Options.Scales["y"].Ticks.StepSize)
}

func TestThingMeasurementChartConfigUsesLegacyPresenceScale(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/components/things/thing-1/measurements", nil)
	config := thingMeasurementChartConfig(req, noopLocalizer{}, "3302-0", appthings.Thing{})

	is.True(config.Options.Scales["y"].Ticks != nil)
	is.True(config.Options.Scales["y"].Ticks.StepSize != nil)
	is.Equal(1.0, *config.Options.Scales["y"].Ticks.StepSize)
}

func TestThingMeasurementChartConfigUsesLegacyStopwatchOnOffScale(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/components/things/thing-1/measurements", nil)
	config := thingMeasurementChartConfig(req, noopLocalizer{}, "3350-5850", appthings.Thing{})

	is.True(config.Options.Scales["y"].Min != nil)
	is.True(config.Options.Scales["y"].Ticks != nil)
	is.True(config.Options.Scales["y"].Ticks.StepSize != nil)
	is.Equal(0.0, *config.Options.Scales["y"].Min)
	is.Equal(1.0, *config.Options.Scales["y"].Ticks.StepSize)
}

func TestThingMeasurementChartConfigUsesMaxDistanceForDistanceCharts(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/components/things/thing-1/measurements", nil)
	maxDistance := 0.94
	config := thingMeasurementChartConfig(req, noopLocalizer{}, "3330-3", appthings.Thing{
		TypeValues: appthings.TypeValues{
			MaxDistance: &maxDistance,
		},
	})

	is.True(config.Options.Scales["y"].Min != nil)
	is.True(config.Options.Scales["y"].Max != nil)
	is.True(config.Options.Scales["y"].Ticks != nil)
	is.True(config.Options.Scales["y"].Ticks.StepSize != nil)
	is.Equal(0.0, *config.Options.Scales["y"].Min)
	is.Equal(1.0, *config.Options.Scales["y"].Max)
	is.Equal(1.0, *config.Options.Scales["y"].Ticks.StepSize)
}

func TestLatestMeasurementViewModelUsesSelectedMeasurement(t *testing.T) {
	is := is.New(t)

	valuePercent := 72.5
	valueMeters := 1.4
	summary := latestMeasurementViewModel("thing-1", []appthings.Measurement{
		{
			ID:        "thing-1/3435/0",
			Timestamp: time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
			Unit:      "%",
			Value:     &valuePercent,
		},
		{
			ID:        "thing-1/3435/1",
			Timestamp: time.Date(2026, 4, 22, 9, 5, 0, 0, time.UTC),
			Unit:      "m",
			Value:     &valueMeters,
		},
	}, "3435-1")

	is.Equal("3435-1", summary.Label)
	is.Equal("m", summary.Unit)
	is.True(summary.Value != nil)
	is.Equal(valueMeters, *summary.Value)
}

func TestNewConnectedSensorSearchOptionsHandlerRequiresThingID(t *testing.T) {
	is := is.New(t)

	handler := NewCompatibleSensorSearchOptionsHandler(context.Background(), testLocaleBundle(), nil, &testThingsApp{})

	req := httptest.NewRequest(http.MethodGet, "/components/things/search-compatible-sensor-options?q=sensor", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
}

func TestComposeDetailsModelUsesConnectedSensorDevEUIForEditDisplay(t *testing.T) {
	is := is.New(t)

	model, err := composeDetailsModel(context.Background(), "thing-1", &testThingsApp{
		thing: appthings.Thing{
			ID:         "thing-1",
			RefDevices: []appthings.RefDevice{{DeviceID: "device-1"}},
			ValidURNs:  []string{"urn:1"},
		},
		devices: map[string]devices.Device{
			"device-1": {DeviceID: "device-1", SensorID: "70B3D57ED00627A1"},
		},
	}, true)

	is.NoErr(err)
	is.True(!model.AllowsMultipleConnectedSensors)
	is.Equal([]featuresthings.ConnectedSensorViewModel{{
		DeviceID: "device-1",
		Label:    "70B3D57ED00627A1",
	}}, model.ConnectedSensors)
}

func TestComposeDetailsModelMarksSandstorageAsMultiSensor(t *testing.T) {
	is := is.New(t)

	model, err := composeDetailsModel(context.Background(), "thing-1", &testThingsApp{
		thing: appthings.Thing{
			ID:        "thing-1",
			Type:      "Container",
			SubType:   "Sandstorage",
			ValidURNs: []string{"urn:1"},
		},
	}, true)

	is.NoErr(err)
	is.True(model.AllowsMultipleConnectedSensors)
}

func TestComposeDetailsModelFallsBackToDeviceIDWhenSensorLookupFails(t *testing.T) {
	is := is.New(t)

	model, err := composeDetailsModel(context.Background(), "thing-1", &testThingsApp{
		thing: appthings.Thing{
			ID:         "thing-1",
			RefDevices: []appthings.RefDevice{{DeviceID: "device-1"}},
		},
	}, false)

	is.NoErr(err)
	is.Equal([]featuresthings.ConnectedSensorViewModel{{
		DeviceID: "device-1",
		Label:    "device-1",
	}}, model.ConnectedSensors)
}

func TestNewCompatibleSensorSearchOptionsHandlerFiltersValidSensors(t *testing.T) {
	is := is.New(t)

	app := &testThingsApp{
		thing: appthings.Thing{
			ID:        "thing-1",
			ValidURNs: []string{"urn:1"},
		},
		validSensors: []appthings.SensorIdentifier{
			{SensorID: "alpha-1", DeviceID: "device-1", Decoder: "weather"},
			{SensorID: "beta-2", DeviceID: "device-2", Decoder: "traffic"},
		},
	}
	handler := NewCompatibleSensorSearchOptionsHandler(context.Background(), testLocaleBundle(), nil, app)

	req := httptest.NewRequest(http.MethodGet, "/components/things/search-compatible-sensor-options?q=weath&thingID=thing-1", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(body, "alpha-1"))
	is.True(strings.Contains(body, "beta-2"))
	is.True(!strings.Contains(body, "device-1"))
}

func TestNewCompatibleSensorSearchOptionsHandlerSupportsMultiUsage(t *testing.T) {
	is := is.New(t)

	app := &testThingsApp{
		thing: appthings.Thing{
			ID:        "thing-1",
			ValidURNs: []string{"urn:1"},
		},
		validSensors: []appthings.SensorIdentifier{
			{SensorID: "alpha-1", DeviceID: "device-1", Decoder: "weather"},
		},
	}
	handler := NewCompatibleSensorSearchOptionsHandler(context.Background(), testLocaleBundle(), nil, app)

	req := httptest.NewRequest(http.MethodGet, "/components/things/search-compatible-sensor-options?q=alpha&thingID=thing-1&usage=multi", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(body, "diwiseMultiSensorSearch.add"))
	is.True(strings.Contains(body, "currentDevice"))
}

func TestLocalizeThingValidationMessageInterpolatesSensorName(t *testing.T) {
	is := is.New(t)

	l10n := locale.NewLocalizer(filepath.Join("..", "..", "..", "..", "..", "assets"), "sv", "en")
	localizer := l10n.For("sv")

	message := localizeThingValidationMessage(localizer, unresolvedConnectedSensorError{value: "sensor-123"})

	is.Equal("Kunde inte hitta kopplad sensor \"sensor-123\"", message)
}

type noopLocalizer struct{}

func (noopLocalizer) Get(key string) string {
	return key
}

func (noopLocalizer) GetWithData(key string, _ map[string]any) string {
	return key
}

func testLocaleBundle() *ftkmock.LocaleBundleMock {
	return &ftkmock.LocaleBundleMock{
		ForFunc: func(string) frontendtoolkit.Localizer {
			return &ftkmock.LocalizerMock{
				GetFunc:         func(key string) string { return key },
				GetWithDataFunc: func(key string, data map[string]any) string {
					if key == "invalidconnectedsensor" {
						return "Could not find connected sensor \"" + fmt.Sprint(data["sensor"]) + "\""
					}
					return key
				},
			}
		},
	}
}
