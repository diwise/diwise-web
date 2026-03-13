package measurements

import (
	"context"
	"time"

	"github.com/diwise/diwise-web/internal/application/common"
)

type Management interface {
	GetMeasurementInfo(ctx context.Context, id string) ([]Value, error)
	GetMeasurementData(ctx context.Context, id string, params ...common.InputParam) (Data, error)
}

type Data struct {
	DeviceID string  `json:"deviceID"`
	Urn      *string `json:"urn,omitempty"`
	Name     *string `json:"name,omitempty"`
	Values   []Value `json:"values,omitempty"`
}

type Value struct {
	ID          *string   `json:"id,omitempty"`
	Name        *string   `json:"n,omitempty"`
	BoolValue   *bool     `json:"vb,omitempty"`
	StringValue string    `json:"vs,omitempty"`
	Value       *float64  `json:"v,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Link        *string   `json:"link,omitempty"`
	Count       uint64    `json:"sum,omitempty"`
}
