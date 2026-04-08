package things

import (
	"time"

	"github.com/a-h/templ"
)

type ThingsPageViewModel struct {
	Things        []ThingViewModel
	Paging        PagingViewModel
	Filters       FiltersViewModel
	TypeOptions   []TypeOption
	TagOptions    []TagOption
	Organisations []string
	MapView       bool
}

type PagingViewModel struct {
	PageIndex  int
	PageLast   int
	PageSize   int
	TotalCount int
	Query      string
	TargetURL  string
	TargetID   string
}

type FiltersViewModel struct {
	SelectedTypes []string
	SelectedTags  []string
	PageSize      int
}

type TypeOption struct {
	Value string
	Label string
}

type TagOption struct {
	Value string
}

type NewThingViewModel struct {
	TypeOptions   []TypeOption
	Organisations []string
}

type ThingViewModel struct {
	ID              string
	Type            string
	SubType         string
	Name            string
	AlternativeName string
	Description     string
	Latitude        float64
	Longitude       float64
	RefDevice       []string
	Tenant          string
	Tags            []string
	Measurements    []MeasurementViewModel
	Latest          map[string]MeasurementViewModel
	ObservedAt      time.Time
	Properties      map[string]any
}

func (t ThingViewModel) HasWarning() bool {
	return t.ObservedAt.IsZero()
}

func (t ThingViewModel) GetMeasurementValue(key string) float64 {
	if t.Properties[key] == nil {
		return 0.0
	}

	value, ok := t.Properties[key].(float64)
	if ok {
		return value
	}

	mapped, ok := t.Properties[key].(map[string]any)
	if !ok {
		return 0.0
	}

	value, ok = mapped["v"].(float64)
	if !ok {
		return 0.0
	}

	return value
}

func (t ThingViewModel) GetFloat(key string) (float64, bool) {
	value, ok := t.Properties[key].(float64)
	return value, ok
}

func (t ThingViewModel) GetBool(key string) (bool, bool) {
	value, ok := t.Properties[key].(bool)
	return value, ok
}

type MeasurementViewModel struct {
	ID          string
	Timestamp   time.Time
	Urn         string
	BoolValue   *bool
	StringValue string
	Unit        string
	Value       *float64
}

type ThingDetailsPageViewModel struct {
	Thing               ThingViewModel
	LatestValues        []LatestMeasurementViewModel
	ConnectedNames      []string
	ValidSensors        []SensorOption
	Organisations       []string
	TagOptions          []string
	MeasurementOptions  []MeasurementOption
	SelectedMeasurement string
}

type SensorOption struct {
	Value string
	Label string
}

type LatestMeasurementViewModel struct {
	ID          string
	Label       string
	Timestamp   time.Time
	Unit        string
	Value       *float64
	BoolValue   *bool
	StringValue string
}

type MeasurementOption struct {
	Value string
	Label string
}

type ThingMeasurementPanelProps struct {
	Chart templ.Component
	Rows  []MeasurementTableRow
	Empty bool
}

type MeasurementTableRow struct {
	Timestamp string
	Value     string
}

type ThingMeasurementField struct {
	Label string
	Name  string
	Value string
	Unit  string
	Step  string
	Min   string
	Max   string
}
