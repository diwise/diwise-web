package things

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/admin"
	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	appthings "github.com/diwise/diwise-web/internal/application/things"
	"github.com/matryer/is"
)

func TestSelectedValuesSplitsCommaSeparatedAndDeduplicates(t *testing.T) {
	is := is.New(t)

	values, err := url.ParseQuery("type=container-wastecontainer,sewer&type=sewer")
	is.NoErr(err)

	selected := selectedValues(values, "type")

	is.Equal([]string{"container-wastecontainer", "sewer"}, selected)
}

func TestSelectedValuesReturnsNilForMissingKey(t *testing.T) {
	is := is.New(t)

	selected := selectedValues(url.Values{}, "tags")

	is.Equal(0, len(selected))
}

func TestNormalizeTypeFilterSplitsCommaSeparatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("type=Container-Sandstorage%2CSewer-CombinedSewerOverflow&tags=a")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"Container-Sandstorage", "Sewer-CombinedSewerOverflow"}, selected)
	is.Equal("tags=a&type=Container-Sandstorage&type=Sewer-CombinedSewerOverflow", params.Encode())
}

func TestNormalizeTypeFilterPreservesRepeatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("type=Container-Sandstorage&type=Sewer-CombinedSewerOverflow&type=Container-Sandstorage")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"Container-Sandstorage", "Sewer-CombinedSewerOverflow"}, selected)
	is.Equal("type=Container-Sandstorage&type=Sewer-CombinedSewerOverflow", params.Encode())
}

func TestNormalizeMultiValueFilterSplitsCommaSeparatedTags(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("tags=sandficka%2Calno&tags=sandficka")
	is.NoErr(err)

	selected := normalizeMultiValueFilter(params, "tags")

	is.Equal([]string{"sandficka", "alno"}, selected)
	is.Equal("tags=sandficka&tags=alno", params.Encode())
}

func TestNewThingFromFormSplitsSubtypeAndMapsFields(t *testing.T) {
	is := is.New(t)

	form := url.Values{
		"type":         {"Container-Sandstorage"},
		"name":         {"Sandficka A"},
		"description":  {"Beskrivning"},
		"organisation": {"tenant-a"},
	}

	thing := newThingFromForm(form)

	is.True(thing.ID != "")
	is.Equal(appthings.Thing{
		ID:          thing.ID,
		Type:        "Container",
		SubType:     "Sandstorage",
		Name:        "Sandficka A",
		Description: "Beskrivning",
		Location: client.Location{
			Latitude:  0,
			Longitude: 0,
		},
		Tenant: "tenant-a",
	}, thing)
}

func TestNewCreateThingPageRedirectsWithoutSaveFlag(t *testing.T) {
	is := is.New(t)

	app := &testThingsApp{}
	handler := NewCreateThingPage(context.Background(), nil, nil, app)

	form := url.Values{
		"type":         {"Container-Sandstorage"},
		"name":         {"Sandficka A"},
		"organisation": {"tenant-a"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusTemporaryRedirect, rec.Code)
	is.Equal("/things", rec.Header().Get("Location"))
	is.Equal(false, app.newThingCalled)
}

func TestNewCreateThingPageCreatesThingWhenSaveFlagPresent(t *testing.T) {
	is := is.New(t)

	app := &testThingsApp{}
	handler := NewCreateThingPage(context.Background(), nil, nil, app)

	form := url.Values{
		"type":         {"Container-Sandstorage"},
		"name":         {"Sandficka A"},
		"organisation": {"tenant-a"},
		"save":         {"true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/things", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusFound, rec.Code)
	is.True(app.newThingCalled)
	is.Equal("Container", app.createdThing.Type)
	is.Equal("Sandstorage", app.createdThing.SubType)
	is.Equal("Sandficka A", app.createdThing.Name)
	is.Equal("tenant-a", app.createdThing.Tenant)
}

type testThingsApp struct {
	newThingCalled bool
	createdThing   appthings.Thing
}

func (a *testThingsApp) NewThing(_ context.Context, thing appthings.Thing) error {
	a.newThingCalled = true
	a.createdThing = thing
	return nil
}

func (a *testThingsApp) GetThings(context.Context, int, int, map[string][]string) (appthings.Result, error) {
	return appthings.Result{}, nil
}

func (a *testThingsApp) GetThing(context.Context, string, map[string][]string) (appthings.Thing, error) {
	return appthings.Thing{}, nil
}

func (a *testThingsApp) GetLatestValues(context.Context, string) ([]appthings.Measurement, error) {
	return nil, nil
}

func (a *testThingsApp) GetValidSensors(context.Context, []string) ([]appthings.SensorIdentifier, error) {
	return nil, nil
}

func (a *testThingsApp) ConnectSensor(context.Context, string, []string) error {
	return nil
}

func (a *testThingsApp) UpdateThing(context.Context, string, map[string]any) error {
	return nil
}

func (a *testThingsApp) DeleteThing(context.Context, string) error {
	return nil
}

func (a *testThingsApp) GetTags(context.Context) ([]string, error) {
	return nil, nil
}

func (a *testThingsApp) GetTypes(context.Context) ([]string, error) {
	return nil, nil
}

func (a *testThingsApp) GetTenants(context.Context) []string {
	return nil
}

func (a *testThingsApp) GetDeviceProfiles(context.Context) []devices.SensorProfile {
	return nil
}

var _ thingsApp = (*testThingsApp)(nil)
var _ admin.Management = (*testThingsApp)(nil)
