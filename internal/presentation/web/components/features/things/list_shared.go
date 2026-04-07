package things

import "strings"
import . "github.com/diwise/frontend-toolkit"

func measurementFloat(thing ThingViewModel, key string) float64 {
	value, _ := thing.GetFloat(key)
	return value
}

func displayThingType(l10n Localizer, thing ThingViewModel) string {
	if thing.SubType != "" {
		return l10n.Get(thing.SubType)
	}
	return l10n.Get(thing.Type)
}

func fallbackValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func firstTags(tags []string, limit int) []string {
	if len(tags) <= limit {
		return tags
	}
	return tags[:limit]
}
