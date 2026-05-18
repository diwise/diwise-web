package things

import (
	"testing"

	shared "github.com/diwise/diwise-web/internal/presentation/web/components/shared"
	"github.com/matryer/is"
)

func TestThingEditShapeResolvesExpectedEditMode(t *testing.T) {
	cases := []struct {
		name     string
		typeName string
		subType  string
		want     shared.EditShape
	}{
		{name: "point of interest type", typeName: "PointOfInterest", want: shared.EditShapePolygon},
		{name: "beach subtype", typeName: "Container", subType: "Beach", want: shared.EditShapePolygon},
		{name: "combined sewer overflow subtype", typeName: "Thing", subType: "CombinedSewerOverflow", want: shared.EditShapePolyline},
		{name: "building type defaults to marker", typeName: "Building", want: shared.EditShapeMarker},
		{name: "sewer type defaults to marker", typeName: "Sewer", want: shared.EditShapeMarker},
		{name: "drain type defaults to marker", typeName: "Drain", want: shared.EditShapeMarker},
		{name: "default marker", typeName: "Room", subType: "Desk", want: shared.EditShapeMarker},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			is.Equal(tc.want, thingEditShape(tc.typeName, tc.subType))
		})
	}
}

func TestThingEditMapConfigUsesPolygonForBeachSubtype(t *testing.T) {
	is := is.New(t)

	shape, geometry := ThingEditMapConfig(ThingViewModel{
		Type:    "Container",
		SubType: "Beach",
	})

	is.Equal(shared.EditShapePolygon, shape)
	is.True(geometry == nil)
}

func TestThingEditMapConfigPrefersPersistedGeometry(t *testing.T) {
	is := is.New(t)

	geometry := shared.NewPolylineGeometry([][]float64{
		{17.301991, 62.399907},
		{17.303535, 62.397818},
		{17.310011, 62.397837},
	})

	shape, resolved := ThingEditMapConfig(ThingViewModel{
		Type:     "Building",
		SubType:  "Beach",
		Geometry: geometry,
	})

	is.Equal(shared.EditShapePolyline, shape)
	is.True(resolved == geometry)
}
