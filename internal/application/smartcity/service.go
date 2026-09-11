package smartcity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/smartcity")

// KnownTypes är de smart-data-model-typer som representerar något "man kan ta
// på" och som vyn Smart stad hanterar. Observationer (…Observed/…Record) ingår
// inte. Listan är avsiktligt enkel att bygga ut.
var KnownTypes = []string{
	"Building",
	"Beach",
	"PointOfInterest",
	"Lifebuoy",
	"Room",
	"WasteContainer",
	"SewagePumpingStation",
	"GreenspaceRecord",
	"Device",
}

// DefaultContextURL används när en ny entitet skapas, så att context-brokern kan
// expandera typ och attribut.
const DefaultContextURL = "https://raw.githubusercontent.com/diwise/context-broker/refs/heads/main/assets/jsonldcontexts/default-context.jsonld"

type Service struct {
	client *client.Client
}

func NewService(client *client.Client) *Service {
	return &Service{client: client}
}

func (s *Service) KnownTypes() []string { return append([]string(nil), KnownTypes...) }

func (s *Service) List(ctx context.Context, types []string) ([]Object, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-smart-city-objects")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if len(types) == 0 {
		types = KnownTypes
	}

	params := url.Values{}
	params.Set("type", strings.Join(types, ","))
	params.Set("limit", "1000")

	base := strings.TrimSuffix(s.client.ContextBrokerURL(), "/")
	body, err := s.client.GetRaw(ctx, base+"/ngsi-ld/v1/entities", params, "application/ld+json")
	if err != nil {
		return nil, err
	}

	var raw []map[string]any
	if err = json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode context broker entities: %w", err)
	}

	objects := make([]Object, 0, len(raw))
	for _, entity := range raw {
		objects = append(objects, toObject(entity))
	}

	return objects, nil
}

func (s *Service) Get(ctx context.Context, id string) (Object, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-smart-city-object")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	base := strings.TrimSuffix(s.client.ContextBrokerURL(), "/")
	body, err := s.client.GetRaw(ctx, base+"/ngsi-ld/v1/entities/"+url.PathEscape(id), url.Values{}, "application/ld+json")
	if err != nil {
		return Object{}, err
	}

	var entity map[string]any
	if err = json.Unmarshal(body, &entity); err != nil {
		return Object{}, fmt.Errorf("failed to decode context broker entity: %w", err)
	}

	return toObject(entity), nil
}

func (s *Service) Create(ctx context.Context, object Object) error {
	var err error
	ctx, span := tracer.Start(ctx, "create-smart-city-object")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	entity := map[string]any{
		"id":       object.ID,
		"type":     object.Type,
		"@context": DefaultContextURL,
	}

	if object.Name != "" {
		entity["name"] = map[string]any{"type": "Property", "value": object.Name}
	}

	if object.HasLocation {
		entity["location"] = map[string]any{
			"type": "GeoProperty",
			"value": map[string]any{
				"type":        "Point",
				"coordinates": []float64{object.Longitude, object.Latitude},
			},
		}
	}

	body, err := json.Marshal(entity)
	if err != nil {
		return err
	}

	base := strings.TrimSuffix(s.client.ContextBrokerURL(), "/")
	return s.client.PostJSONLD(ctx, base+"/ngsi-ld/v1/entities", body)
}

func toObject(entity map[string]any) Object {
	object := Object{
		ID:   stringValue(entity["id"]),
		Type: shortType(stringValue(entity["type"])),
	}

	if name, ok := findAttribute(entity, "name"); ok {
		object.Name = propertyValue(name)
	}

	if location, ok := findAttribute(entity, "location"); ok {
		if lat, lon, ok := coordinates(location); ok {
			object.Latitude, object.Longitude, object.HasLocation = lat, lon, true
		}
	}

	return object
}

// findAttribute hittar ett attribut även när nyckeln är expanderad till en URI
// (t.ex. https://.../terms.jsonld#/definitions/name), genom att matcha sista
// vägsegmentet.
func findAttribute(entity map[string]any, name string) (any, bool) {
	if v, ok := entity[name]; ok {
		return v, true
	}

	for k, v := range entity {
		if lastSegment(k) == name {
			return v, true
		}
	}

	return nil, false
}

func lastSegment(key string) string {
	if i := strings.LastIndexAny(key, "#/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

func shortType(entityType string) string {
	if entityType == "" {
		return ""
	}
	if i := strings.LastIndexAny(entityType, "#/"); i >= 0 {
		return entityType[i+1:]
	}
	return entityType
}

func propertyValue(attribute any) string {
	if m, ok := attribute.(map[string]any); ok {
		return stringValue(m["value"])
	}
	return stringValue(attribute)
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%v", x)
	default:
		return ""
	}
}

func coordinates(attribute any) (float64, float64, bool) {
	m, ok := attribute.(map[string]any)
	if !ok {
		return 0, 0, false
	}

	value, ok := m["value"].(map[string]any)
	if !ok {
		return 0, 0, false
	}

	coords, ok := value["coordinates"].([]any)
	if !ok || len(coords) < 2 {
		return 0, 0, false
	}

	lon, okLon := coords[0].(float64)
	lat, okLat := coords[1].(float64)
	if !okLon || !okLat {
		return 0, 0, false
	}

	return lat, lon, true
}
