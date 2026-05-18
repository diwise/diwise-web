package things

import (
	"strings"

	shared "github.com/diwise/diwise-web/internal/presentation/web/components/shared"
)

var editShapeByType = map[string]shared.EditShape{
	"pointofinterest": shared.EditShapePolygon,
}

var editShapeBySubType = map[string]shared.EditShape{
	"beach":                 shared.EditShapePolygon,
	"combinedseweroverflow": shared.EditShapePolyline,
}

func thingDisplayName(thing ThingViewModel) string {
	if strings.TrimSpace(thing.Name) != "" {
		return thing.Name
	}
	return thing.ID
}

func ThingEditMapConfig(thing ThingViewModel) (shared.EditShape, *shared.GeoJSONGeometry) {
	if thing.Geometry != nil {
		switch thing.Geometry.Type {
		case "LineString":
			return shared.EditShapePolyline, thing.Geometry
		case "Polygon":
			return shared.EditShapePolygon, thing.Geometry
		}
	}

	return thingEditShape(thing.Type, thing.SubType), nil
}

func thingEditShape(typeName, subTypeName string) shared.EditShape {
	kind := strings.ToLower(strings.TrimSpace(typeName))
	if shape, ok := editShapeByType[kind]; ok {
		return shape
	}

	kind = strings.ToLower(strings.TrimSpace(subTypeName))
	if shape, ok := editShapeBySubType[kind]; ok {
		return shape
	}

	return shared.EditShapeMarker
}
