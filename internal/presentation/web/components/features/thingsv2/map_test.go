package thingsv2

import (
	"encoding/json"
	"strings"
	"testing"

	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/matryer/is"
)

func testLocalizer() frontendtoolkit.Localizer {
	return &ftkmock.LocalizerMock{
		GetFunc:         func(key string) string { return key },
		GetWithDataFunc: func(key string, _ map[string]any) string { return key },
	}
}

func TestThingsV2ToMapFeatureKeepsPolygonsAndSkipsLocationless(t *testing.T) {
	is := is.New(t)

	primary := 42.0
	collection := thingsV2ToMapFeature(testLocalizer(), []ThingV2ViewModel{
		{ID: "bin-1", Name: "Tunna", Category: "container", HasLocation: true, Latitude: 62.39, Longitude: 17.3, PrimaryLabel: "Fyllnadsgrad", PrimaryValue: primary, HasPrimaryValue: true, PrimaryUnit: "%"},
		{ID: "area-1", Name: "Område", Category: "area", HasGeometry: true, GeometryType: "Polygon", Geometry: json.RawMessage(`[[[17.3,62.39],[17.4,62.39],[17.4,62.4],[17.3,62.39]]]`)},
		{ID: "noloc-1", Name: "Utan plats", Category: "room"},
	})

	is.Equal(2, len(collection.Features))

	pointGeometry, err := json.Marshal(collection.Features[0].Geometry)
	is.NoErr(err)
	is.True(strings.Contains(string(pointGeometry), `"Point"`))
	is.Equal("Tunna", collection.Features[0].Properties["name"])
	is.Equal(true, collection.Features[0].Properties["hasprimary"])
	is.Equal("42 %", collection.Features[0].Properties["primaryvalue"])

	polygonGeometry, err := json.Marshal(collection.Features[1].Geometry)
	is.NoErr(err)
	is.True(strings.Contains(string(polygonGeometry), `"Polygon"`))
	is.True(strings.Contains(string(polygonGeometry), "17.4"))
	is.Equal(false, collection.Features[1].Properties["hasprimary"])
}

func TestThingsV2MapReturnsNopWhenTableView(t *testing.T) {
	is := is.New(t)

	component := ThingsV2Map(testLocalizer(), ThingsV2PageViewModel{MapView: false})

	is.True(component != nil)
}
