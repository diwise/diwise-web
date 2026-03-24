package things

import "strings"

func thingDisplayName(thing ThingViewModel) string {
	if strings.TrimSpace(thing.Name) != "" {
		return thing.Name
	}
	return thing.ID
}
