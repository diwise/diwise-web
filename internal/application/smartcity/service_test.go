package smartcity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/matryer/is"
)

func TestListFetchesTypesPerTenantAndFiltersObservations(t *testing.T) {
	is := is.New(t)

	var entityTypeQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.Equal(r.Header.Get("NGSILD-Tenant"), "default")

		switch r.URL.Path {
		case "/ngsi-ld/types":
			_, _ = w.Write([]byte(`["Building","Beach","WeatherObserved"]`))
		case "/ngsi-ld/v1/entities":
			entityTypeQuery = r.URL.Query().Get("type")
			_, _ = w.Write([]byte(`[
				{"@context":"https://uri.etsi.org/ngsi-ld/v1/ngsi-ld-core-context-v1.9.jsonld","id":"urn:ngsi-ld:Building:Gata.1","type":"Building","name":{"type":"Property","value":"Gatan 1"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.3,62.39]}}},
				{"@context":"https://uri.etsi.org/ngsi-ld/v1/ngsi-ld-core-context-v1.9.jsonld","id":"urn:ngsi-ld:Beach:Stranden","type":"Beach","name":{"type":"Property","value":"Stranden"}}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := NewService(client.NewClient("", "", "", "", "", srv.URL))
	objects, err := svc.List(context.Background(), []string{"default"})
	is.NoErr(err)
	is.Equal(len(objects), 2)

	// Observationer ska filtreras bort innan entitetsanropet.
	is.True(!strings.Contains(entityTypeQuery, "WeatherObserved"))
	is.True(strings.Contains(entityTypeQuery, "Building"))

	is.Equal(objects[0].ID, "urn:ngsi-ld:Building:Gata.1")
	is.Equal(objects[0].Type, "Building")
	is.Equal(objects[0].Name, "Gatan 1")
	is.Equal(objects[0].Tenant, "default")
	is.True(objects[0].HasLocation)
	is.Equal(objects[0].Latitude, 62.39)
	is.Equal(objects[0].Longitude, 17.3)
}

func TestToObjectParsesExpandedEntity(t *testing.T) {
	is := is.New(t)

	object := toObject(map[string]any{
		"id":   "urn:ngsi-ld:PointOfInterest:Torg",
		"type": "https://uri.fiware.org/ns/data-models#PointOfInterest",
		"https://smart-data-models.github.io/data-models/terms.jsonld#/definitions/name": map[string]any{
			"type":  "Property",
			"value": "Torget",
		},
	})

	is.Equal(object.Type, "PointOfInterest")
	is.Equal(object.Name, "Torget")
}
