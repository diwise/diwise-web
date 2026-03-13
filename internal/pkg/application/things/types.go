package things

import (
	"context"
	"encoding/json"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application/common"
)

type Management interface {
	NewThing(ctx context.Context, t Thing) error
	GetThing(ctx context.Context, id string, params map[string][]string) (Thing, error)
	GetLatestValues(ctx context.Context, thingID string) ([]Measurement, error)
	GetThings(ctx context.Context, offset, limit int, params map[string][]string) (Result, error)
	UpdateThing(ctx context.Context, thingID string, fields map[string]any) error
	DeleteThing(ctx context.Context, thingID string) error
	GetTags(ctx context.Context) ([]string, error)
	GetTypes(ctx context.Context) ([]string, error)
	GetValidSensors(ctx context.Context, types []string) ([]SensorIdentifier, error)
	ConnectSensor(ctx context.Context, thingID string, refDevices []string) error
}

type Thing struct {
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	SubType         string          `json:"subType,omitempty"`
	Name            string          `json:"name"`
	AlternativeName string          `json:"alternativeName,omitempty"`
	Description     string          `json:"description"`
	Location        common.Location `json:"location,omitempty"`
	RefDevices      []RefDevice     `json:"refDevices,omitempty"`
	Tags            []string        `json:"tags,omitempty"`
	Tenant          string          `json:"tenant"`
	ObservedAt      time.Time       `json:"observedAt,omitempty"`
	ValidURNs       []string        `json:"validURN,omitempty"`

	Values     [][]Measurement `json:"-"`
	TypeValues TypeValues      `json:"-"`
}

type TypeValues struct {
	Energy      *float64     `json:"energy"`
	Power       *float64     `json:"power"`
	Presence    *bool        `json:"presence"`
	Temperature *Measurement `json:"temperature"`

	MaxDistance  *float64 `json:"maxd,omitempty"`
	MaxLevel     *float64 `json:"maxl,omitempty"`
	MeanLevel    *float64 `json:"meanl,omitempty"`
	Offset       *float64 `json:"offset,omitempty"`
	Angle        *float64 `json:"angle,omitempty"`
	CurrentLevel *float64 `json:"currentLevel"`
	Percent      *float64 `json:"percent"`

	CumulatedNumberOfPassages *int64 `json:"cumulatedNumberOfPassages"`
	PassagesToday             *int64 `json:"passagesToday"`
	CurrentState              *bool  `json:"currentState"`

	PumpingObserved   *bool          `json:"pumpingObserved"`
	PumpingObservedAt *time.Time     `json:"pumpingObservedAt"`
	PumpingDuration   *time.Duration `json:"pumpingDuration"`

	OverflowDuration   *time.Duration `json:"overflowDuration"`
	OverflowObserved   *bool          `json:"overflowObserved"`
	OverflowObservedAt *time.Time     `json:"overflowObservedAt"`

	CumulativeVolume *float64 `json:"cumulativeVolume"`
	Leakage          *bool    `json:"leakage"`
	Burst            *bool    `json:"burst"`
	Backflow         *bool    `json:"backflow"`
	Fraud            *bool    `json:"fraud"`
}

func (t *Thing) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}

	t2 := struct {
		ID              string          `json:"id"`
		Type            string          `json:"type"`
		SubType         string          `json:"subType,omitempty"`
		Name            string          `json:"name"`
		AlternativeName string          `json:"alternativeName,omitempty"`
		Description     string          `json:"description"`
		Location        common.Location `json:"location,omitempty"`
		RefDevices      []RefDevice     `json:"refDevices,omitempty"`
		Tags            []string        `json:"tags,omitempty"`
		Tenant          string          `json:"tenant"`
		ObservedAt      time.Time       `json:"observedAt,omitempty"`
		ValidURNs       []string        `json:"validURN,omitempty"`
	}{}

	if err := json.Unmarshal(data, &t2); err != nil {
		return err
	}

	t.ID = t2.ID
	t.Type = t2.Type
	t.SubType = t2.SubType
	t.Name = t2.Name
	t.AlternativeName = t2.AlternativeName
	t.Description = t2.Description
	t.Location = t2.Location
	t.RefDevices = t2.RefDevices
	t.Tags = t2.Tags
	t.Tenant = t2.Tenant
	t.ObservedAt = t2.ObservedAt
	t.ValidURNs = t2.ValidURNs

	tv := TypeValues{}
	if err := json.Unmarshal(data, &tv); err == nil {
		t.TypeValues = tv
	}

	values := [][]Measurement{}
	v := struct {
		Values []Measurement `json:"values,omitempty"`
	}{}
	m := struct {
		Values map[string][]Measurement `json:"values,omitempty"`
	}{}

	if err := json.Unmarshal(data, &v); err == nil {
		values = append(values, v.Values)
	} else if err := json.Unmarshal(data, &m); err == nil {
		for _, vv := range m.Values {
			values = append(values, vv)
		}
	}

	t.Values = values
	return nil
}

type RefDevice struct {
	DeviceID string `json:"deviceID"`
}

type Measurement struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Urn         string    `json:"urn"`
	BoolValue   *bool     `json:"vb,omitempty"`
	StringValue *string   `json:"vs,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Value       *float64  `json:"v,omitempty"`
	Count       *float64  `json:"count,omitempty"`
	RefDevice   string    `json:"ref,omitempty"`
	Source      *string   `json:"source,omitzero"`
}

type SensorIdentifier struct {
	SensorID string `json:"sensorID,omitempty"`
	DeviceID string `json:"deviceID"`
	Decoder  string `json:"decoder"`
}

type Result struct {
	Things       []Thing
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
