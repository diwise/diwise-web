package sensors

import (
	"fmt"

	shared "github.com/diwise/diwise-web/internal/presentation/web/components/shared"
)

func formatCoordinate(value float64) string {
	if value == 0 {
		return "-"
	}
	return fmt.Sprintf("%f", value)
}

func sensorDetailsMapFeature(sensor SensorDetailsPageViewModel) shared.FeatureCollection {
	feature := shared.NewFeature(shared.NewPoint(sensor.Latitude, sensor.Longitude))
	feature.AddProperty("desc", sensor.Description)
	feature.AddProperty("type", "sensor")
	return shared.NewFeatureCollection([]shared.Feature{feature})
}
