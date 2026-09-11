package smartcity

import "net/url"

type ObjectViewModel struct {
	ID          string
	Type        string
	Name        string
	Latitude    float64
	Longitude   float64
	HasLocation bool
}

type TypeOption struct {
	Value string
	Label string
}

type PageViewModel struct {
	Objects  []ObjectViewModel
	Types    []TypeOption
	Selected *ObjectViewModel
}

func (m PageViewModel) mapFeatures() []ObjectViewModel {
	withLocation := make([]ObjectViewModel, 0, len(m.Objects))
	for _, object := range m.Objects {
		if object.HasLocation {
			withLocation = append(withLocation, object)
		}
	}
	return withLocation
}

func objectURL(id string) string {
	return "/components/smart-city/" + url.PathEscape(id)
}
