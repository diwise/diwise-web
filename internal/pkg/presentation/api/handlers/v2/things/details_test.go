package things

import (
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
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
		"currentDevice":   {"device-1, device-2, device-1"},
		"latitude":        {"57.708870"},
		"longitude":       {"11.974560"},
		"maxl":            {"1.25"},
		"maxd":            {"2.50"},
		"angle":           {"3.75"},
		"offset":          {"4.50"},
	}

	fields := buildThingUpdateFields(form)

	is.Equal("Container A", fields["name"])
	is.Equal("Station 17", fields["alternativeName"])
	is.Equal("Updated description", fields["description"])
	is.Equal("tenant-a", fields["tenant"])
	is.Equal([]string{"waste", "downtown"}, fields["tags"])
	is.Equal([]application.Device{{DeviceID: "device-1"}, {DeviceID: "device-2"}}, fields["refDevices"])
	is.Equal(application.Location{Latitude: 57.70887, Longitude: 11.97456}, fields["location"])
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

	fields := buildThingUpdateFields(form)

	is.Equal([]string{"waste", "downtown"}, fields["tags"])
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

func TestThingMeasurementChartConfigUsesMaxDistanceForDistanceCharts(t *testing.T) {
	is := is.New(t)

	req := httptest.NewRequest("GET", "/v2/components/things/thing-1/measurements", nil)
	maxDistance := 0.94
	config := thingMeasurementChartConfig(req, noopLocalizer{}, "3330-3", application.Thing{
		TypeValues: application.ThingTypeValues{
			MaxDistance: &maxDistance,
		},
	})

	is.True(config.Options.Scales["y"].Min != nil)
	is.True(config.Options.Scales["y"].Max != nil)
	is.Equal(0.0, *config.Options.Scales["y"].Min)
	is.Equal(1.0, *config.Options.Scales["y"].Max)
}

type noopLocalizer struct{}

func (noopLocalizer) Get(key string) string {
	return key
}

func (noopLocalizer) GetWithData(key string, _ map[string]any) string {
	return key
}
