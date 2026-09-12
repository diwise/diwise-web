package smartcity

type ObjectViewModel struct {
	ID          string
	Type        string
	Name        string
	Tenant      string
	Latitude    float64
	Longitude   float64
	HasLocation bool
}

type PageViewModel struct {
	Objects     []ObjectViewModel
	Icons       map[string]string
	DefaultIcon string
}

type mapData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Zoom      int     `json:"zoom"`
}

type mapFeature struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Name      string  `json:"name"`
	Tenant    string  `json:"tenant"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (m PageViewModel) features() []mapFeature {
	features := make([]mapFeature, 0, len(m.Objects))
	for _, object := range m.Objects {
		if !object.HasLocation {
			continue
		}
		features = append(features, mapFeature{
			ID:        object.ID,
			Type:      object.Type,
			Name:      object.Name,
			Tenant:    object.Tenant,
			Latitude:  object.Latitude,
			Longitude: object.Longitude,
		})
	}
	return features
}
