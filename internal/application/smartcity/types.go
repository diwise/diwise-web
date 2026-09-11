package smartcity

import "context"

// Object är en "påtaglig" NGSI-LD-entitet (byggnad, strand, POI, livboj …) i
// context-broker, till skillnad från observationer.
type Object struct {
	ID          string
	Type        string
	Name        string
	Latitude    float64
	Longitude   float64
	HasLocation bool
}

// Management läser och skapar påtagliga objekt via context-broker.
type Management interface {
	List(ctx context.Context, types []string) ([]Object, error)
	Get(ctx context.Context, id string) (Object, error)
	Create(ctx context.Context, object Object) error
	KnownTypes() []string
}
