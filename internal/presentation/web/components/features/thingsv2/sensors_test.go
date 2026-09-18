package thingsv2

import (
	"context"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestSensorInputDomIDSanitizesSelectorChars(t *testing.T) {
	is := is.New(t)

	is.Equal("temperature", sensorInputDomID("temperature"))
	is.Equal("temperature-center", sensorInputDomID("temperature.center"))
	is.Equal("a-b-c", sensorInputDomID("a.b c"))
}

func TestSensorsDialogUsesSelectorSafeTargetForDottedInputs(t *testing.T) {
	is := is.New(t)

	model := ThingV2SensorsViewModel{
		ThingID:  "room-1",
		Tenant:   "default",
		Revision: 7,
		Name:     "Rum",
		Bindings: []SensorBindingViewModel{{Input: "temperature.center", Label: "Temperatur mitt i rummet"}},
	}
	var sb strings.Builder
	is.NoErr(ThingV2SensorsDialog(testLocalizer(), model).Render(context.Background(), &sb))
	html := sb.String()

	is.True(strings.Contains(html, `id="thing-v2-sensor-results-temperature-center"`))
	is.True(strings.Contains(html, `hx-target="#thing-v2-sensor-results-temperature-center"`))
	is.True(strings.Contains(html, `input=temperature.center`))
	is.Equal(false, strings.Contains(html, `hx-target="#thing-v2-sensor-results-temperature.center"`))
}
