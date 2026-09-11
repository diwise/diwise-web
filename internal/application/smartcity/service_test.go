package smartcity

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/matryer/is"
)

func TestListParsesCompactEntities(t *testing.T) {
	is := is.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.Equal(r.URL.Path, "/ngsi-ld/v1/entities")
		is.True(r.URL.Query().Get("type") != "")
		_, _ = w.Write([]byte(`[
			{"id":"urn:ngsi-ld:Building:Gata.1","type":"Building","name":{"type":"Property","value":"Gatan 1"},"location":{"type":"GeoProperty","value":{"type":"Point","coordinates":[17.3,62.39]}}},
			{"id":"urn:ngsi-ld:Beach:Stranden","type":"Beach","name":{"type":"Property","value":"Stranden"}}
		]`))
	}))
	defer srv.Close()

	svc := NewService(client.NewClient("", "", "", "", "", srv.URL))
	objects, err := svc.List(context.Background(), nil)
	is.NoErr(err)
	is.Equal(len(objects), 2)

	is.Equal(objects[0].ID, "urn:ngsi-ld:Building:Gata.1")
	is.Equal(objects[0].Type, "Building")
	is.Equal(objects[0].Name, "Gatan 1")
	is.True(objects[0].HasLocation)
	is.Equal(objects[0].Latitude, 62.39)
	is.Equal(objects[0].Longitude, 17.3)

	is.True(!objects[1].HasLocation)
}

func TestListParsesExpandedEntities(t *testing.T) {
	is := is.New(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{
				"id":"urn:ngsi-ld:PointOfInterest:Torg",
				"type":"https://uri.fiware.org/ns/data-models#PointOfInterest",
				"https://smart-data-models.github.io/data-models/terms.jsonld#/definitions/name":{"type":"Property","value":"Torget"}
			}
		]`))
	}))
	defer srv.Close()

	svc := NewService(client.NewClient("", "", "", "", "", srv.URL))
	objects, err := svc.List(context.Background(), []string{"PointOfInterest"})
	is.NoErr(err)
	is.Equal(len(objects), 1)
	is.Equal(objects[0].Type, "PointOfInterest")
	is.Equal(objects[0].Name, "Torget")
}

func TestCreatePostsJSONLD(t *testing.T) {
	is := is.New(t)

	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.Equal(r.Method, http.MethodPost)
		is.Equal(r.URL.Path, "/ngsi-ld/v1/entities")
		is.Equal(r.Header.Get("Content-Type"), "application/ld+json")

		body, _ := io.ReadAll(r.Body)
		is.NoErr(json.Unmarshal(body, &captured))
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	svc := NewService(client.NewClient("", "", "", "", "", srv.URL))
	err := svc.Create(context.Background(), Object{
		ID: "urn:ngsi-ld:Building:Ny.1", Type: "Building", Name: "Nya huset",
		Latitude: 62.4, Longitude: 17.3, HasLocation: true,
	})
	is.NoErr(err)
	is.Equal(captured["id"], "urn:ngsi-ld:Building:Ny.1")
	is.Equal(captured["type"], "Building")
	is.True(captured["@context"] != nil)
	is.True(captured["location"] != nil)
}
