package smartcity

import "context"

// Object är en NGSI-LD-entitet som visas som en markör i stadsöversikten.
type Object struct {
	ID          string
	Type        string
	Name        string
	Tenant      string
	Latitude    float64
	Longitude   float64
	HasLocation bool
}

// Management läser entiteter från context-broker för de tenants användaren har
// åtkomst till.
type Management interface {
	List(ctx context.Context, tenants []string) ([]Object, error)
}
