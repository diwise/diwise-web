package devmode

import (
	"context"
	"net/http"
)

const smartCityEntities = `[
  {"@context":"https://uri.etsi.org/ngsi-ld/v1/ngsi-ld-core-context-v1.9.jsonld","id":"urn:ngsi-ld:Building:Gata.1","type":"Building","name":{"type":"Property","value":"Gatan 1"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.3069,62.3908]}}},
  {"@context":"https://uri.etsi.org/ngsi-ld/v1/ngsi-ld-core-context-v1.9.jsonld","id":"urn:ngsi-ld:Beach:Stranden","type":"Beach","name":{"type":"Property","value":"Stranden"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.45,62.36]}}},
  {"@context":"https://uri.etsi.org/ngsi-ld/v1/ngsi-ld-core-context-v1.9.jsonld","id":"urn:ngsi-ld:PointOfInterest:Torget","type":"PointOfInterest","name":{"type":"Property","value":"Torget"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.29,62.4]}}},
  {"@context":"https://uri.etsi.org/ngsi-ld/v1/ngsi-ld-core-context-v1.9.jsonld","id":"urn:ngsi-ld:Lifebuoy:Livboj.1","type":"Lifebuoy","name":{"type":"Property","value":"Livboj 1"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.31,62.39]}}}
]`

const smartCityTypes = `["Building","Beach","PointOfInterest","Lifebuoy"]`

func NewSmartCityTypesHandler(_ context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(smartCityTypes))
	}
}

func NewSmartCityListHandler(_ context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/ld+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(smartCityEntities))
	}
}
