package smartcity

import (
	"context"
	"fmt"
	"strings"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/smartcity")

type Service struct {
	client *client.Client
}

func NewService(client *client.Client) *Service {
	return &Service{client: client}
}

// List hämtar entiteter för samtliga tenants. Eftersom en användare kan ha
// åtkomst till flera tenants görs ett anrop per tenant. Prestanda är inte
// prioriterat här.
func (s *Service) List(ctx context.Context, tenants []string) ([]Object, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-smart-city-objects")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx)

	objects := make([]Object, 0)

	for _, tenant := range tenants {
		types, err := s.client.ContextBrokerTypes(ctx, tenant)
		if err != nil {
			log.Warn("could not fetch entity types for tenant", "tenant", tenant, "err", err.Error())
			continue
		}

		types = tangibleTypes(types)
		if len(types) == 0 {
			continue
		}

		entities, err := s.client.ContextBrokerEntities(ctx, tenant, types)
		if err != nil {
			log.Warn("could not fetch entities for tenant", "tenant", tenant, "err", err.Error())
			continue
		}

		for _, entity := range entities {
			object := toObject(entity)
			if object.ID == "" {
				continue
			}
			object.Tenant = tenant
			objects = append(objects, object)
		}
	}

	return objects, nil
}

// tangibleTypes filtrerar bort observationer (…Observed) så att översikten
// visar sådant "man kan ta på".
func tangibleTypes(types []string) []string {
	result := make([]string, 0, len(types))
	for _, t := range types {
		if strings.Contains(strings.ToLower(t), "observed") {
			continue
		}
		result = append(result, t)
	}
	return result
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
