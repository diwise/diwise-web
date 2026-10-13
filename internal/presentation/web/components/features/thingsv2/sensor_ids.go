package thingsv2

import "regexp"

var sensorInputDomIDPattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// sensorInputDomID gör om en ingångs-ID till ett DOM-id som är säkert att
// använda i CSS-selektorer (t.ex. hx-target). Ingångar som "temperature.center"
// skulle annars tolkas som id + klass av querySelector.
func sensorInputDomID(input string) string {
	return sensorInputDomIDPattern.ReplaceAllString(input, "-")
}
