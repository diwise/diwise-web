package sensors

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/matryer/is"
)

func TestNormalizeTypeFilterSplitsCommaSeparatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("search=&type=elsys%2Cmilesight&active=&online=&lastseen=")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"elsys", "milesight"}, selected)
	is.Equal("active=&lastseen=&online=&search=&type=elsys&type=milesight", params.Encode())
}

func TestNormalizeTypeFilterPreservesRepeatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("type=elsys&type=milesight&type=elsys")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"elsys", "milesight"}, selected)
	is.Equal("type=elsys&type=milesight", params.Encode())
}

func TestBuildSensorUpdateFieldsMapsEditForm(t *testing.T) {
	is := is.New(t)

	form := url.Values{
		"id":                       {"device-1"},
		"name":                     {"Temperature"},
		"description":              {"Outdoor sensor"},
		"sensorType":               {"decoder-x"},
		"organisation":             {"tenant-a"},
		"environment":              {"air"},
		"interval":                 {"24"},
		"latitude":                 {"57.708870"},
		"longitude":                {"11.974560"},
		"active":                   {"on"},
		"measurementType-option[]": {"urn:1", "urn:2"},
	}
	req := &http.Request{Form: form}

	fields := buildSensorUpdateFields(req)

	is.Equal("device-1", fields["deviceID"])
	is.Equal("Temperature", fields["name"])
	is.Equal("Outdoor sensor", fields["description"])
	is.Equal("tenant-a", fields["tenant"])
	is.Equal("air", fields["environment"])
	is.Equal("24", fields["interval"])
	is.Equal(true, fields["active"])
	is.Equal(57.70887, fields["latitude"])
	is.Equal(11.97456, fields["longitude"])
	is.Equal([]string{"urn:1", "urn:2"}, fields["types"])
	_, hasSensorType := fields["sensorType"]
	is.Equal(false, hasSensorType)
	_, hasDeviceProfile := fields["deviceProfile"]
	is.Equal(false, hasDeviceProfile)
}

func TestBuildSensorUpdateFieldsSplitsCommaSeparatedMeasurementTypes(t *testing.T) {
	is := is.New(t)

	form := url.Values{
		"id":                       {"device-1"},
		"measurementType-option[]": {"urn:1,urn:2, urn:1"},
	}
	req := &http.Request{Form: form}

	fields := buildSensorUpdateFields(req)

	is.Equal([]string{"urn:1", "urn:2"}, fields["types"])
}

func TestBuildSensorUpdateFieldsDoesNotForceInactiveWhenCheckboxMissing(t *testing.T) {
	is := is.New(t)

	form := url.Values{
		"id":   {"device-1"},
		"name": {"Temperature"},
	}
	req := &http.Request{Form: form}

	fields := buildSensorUpdateFields(req)

	_, hasActive := fields["active"]
	is.Equal(false, hasActive)
}

func TestMeasurementTypeOptionsUsesMatchingProfileAndSelection(t *testing.T) {
	is := is.New(t)

	profiles := []devices.SensorProfile{
		{Name: "Weather", Decoder: "weather-decoder", Types: &[]string{"sensor:temperature", "sensor:humidity"}},
	}

	options := measurementTypeOptions(nil, profiles, "weather-decoder", []string{"sensor:humidity"}, nil)

	is.Equal(2, len(options))
	is.Equal("sensor:humidity", options[0].Value)
	is.Equal("humidity", options[0].Label)
	is.Equal(true, options[0].Selected)
	is.Equal("sensor:temperature", options[1].Value)
	is.Equal("temperature", options[1].Label)
	is.Equal(false, options[1].Selected)
}

func TestMeasurementTypeOptionsPrefersProvidedLabels(t *testing.T) {
	is := is.New(t)

	profiles := []devices.SensorProfile{
		{Name: "Weather", Decoder: "weather-decoder", Types: &[]string{"guid-1"}},
	}

	options := measurementTypeOptions(nil, profiles, "weather-decoder", []string{"guid-1"}, map[string]string{
		"guid-1": "Temperature",
	})

	is.Equal(1, len(options))
	is.Equal("guid-1", options[0].Value)
	is.Equal("Temperature", options[0].Label)
	is.Equal(true, options[0].Selected)
}
