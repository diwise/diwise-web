package devmode

import (
	"context"
	"encoding/json"
	"net/http"
)

const smartCityEntities = `[
  {"id":"urn:ngsi-ld:Building:Gata.1","type":"Building","name":{"type":"Property","value":"Gatan 1"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.3069,62.3908]}}},
  {"id":"urn:ngsi-ld:Beach:Stranden","type":"Beach","name":{"type":"Property","value":"Stranden"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.45,62.36]}}},
  {"id":"urn:ngsi-ld:PointOfInterest:Torget","type":"PointOfInterest","name":{"type":"Property","value":"Torget"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.29,62.4]}}},
  {"id":"urn:ngsi-ld:Lifebuoy:Livboj.1","type":"Lifebuoy","name":{"type":"Property","value":"Livboj 1"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.31,62.39]}}}
]`

func NewSmartCityListHandler(_ context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/ld+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(smartCityEntities))
	}
}

func NewSmartCityEntityHandler(_ context.Context) http.HandlerFunc {
	var entities []map[string]any
	_ = json.Unmarshal([]byte(smartCityEntities), &entities)

	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		for _, entity := range entities {
			if entity["id"] == id {
				w.Header().Set("Content-Type", "application/ld+json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(entity)
				return
			}
		}
		http.NotFound(w, r)
	}
}

func NewSmartCityCreateHandler(_ context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}
}
