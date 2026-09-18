package thingsv2

import (
	"encoding/json"
	"time"

	featuresthings "github.com/diwise/diwise-web/internal/presentation/web/components/features/things"
)

// ThingV2ViewModel är en sakrad (iot-things-v2): generisk status via
// primary i stället för typswitch. Geometry bär råa GeoJSON-koordinater
// för icke-punktgeometrier (polygoner ritas som overlay).
type ThingV2ViewModel struct {
	ID              string
	Tenant          string
	Name            string
	Category        string
	TemplateID      string
	HasLocation     bool
	Latitude        float64
	Longitude       float64
	HasGeometry     bool
	GeometryType    string
	Geometry        json.RawMessage
	PrimaryLabel    string
	PrimaryValue    float64
	HasPrimaryValue bool
	PrimaryUnit     string
	PrimaryQuality  string
}

// ThingsV2PageViewModel är list/kart-modellen för /things-v2.
type ThingsV2PageViewModel struct {
	Things          []ThingV2ViewModel
	Paging          featuresthings.PagingViewModel
	Filters         FiltersViewModel
	CategoryOptions []featuresthings.TypeOption
	TemplateOptions []featuresthings.TypeOption
	MapView         bool
}

// FiltersViewModel är valda filter (enkelval per dropdown + fritext namn).
type FiltersViewModel struct {
	SelectedCategories []string
	SelectedTemplates  []string
	Name               string
	PageSize           int
}

// ThingV2ValueViewModel är ett aktuellt egenskapsvärde, sorterat på
// PropertyID i vyn. Generiskt: inga typspecifika rader.
type ThingV2ValueViewModel struct {
	PropertyID string
	Label      string
	HasValue   bool
	Value      float64
	Unit       string
	Quality    string
	ObservedAt time.Time
}

// MetadataItem är en metadata-nyckel sorterad på Key.
type MetadataItem struct {
	Key   string
	Value string
}

// ConnectedSensorViewModel är en sensorkoppling: enhetssignal till
// beräkningsingång, sorterad på enhet sedan ingång.
type ConnectedSensorViewModel struct {
	DeviceID string
	Input    string
	Object   string
	Resource string
}

// ThingV2DetailsViewModel är detaljsidan för /things-v2/{id}.
type ThingV2DetailsViewModel struct {
	Thing                  ThingV2ViewModel
	Values                 []ThingV2ValueViewModel
	Metadata               []MetadataItem
	ConnectedSensors       []ConnectedSensorViewModel
	RelatedThings          []ThingV2ViewModel
	TemplateVersion        string
	VariantID              string
	VariantVersion         string
	Revision               int64
	Tenant                 string
	HistoryProperties      []featuresthings.TypeOption
	DefaultHistoryProperty string
}

// ParamFieldViewModel är ett överstyrbart parameterfält i skapa/redigera.
type ParamFieldViewModel struct {
	Name       string
	Label      string
	Unit       string
	Min        *float64
	Max        *float64
	Default    float64
	HasDefault bool
	Value      string
}

// ThingV2CreateViewModel är skapa-sidan för /things-v2/new.
type ThingV2CreateViewModel struct {
	Tenants         []string
	Tenant          string
	Templates       []featuresthings.TypeOption
	Template        string
	TemplateDisplay string
	Variants        []featuresthings.TypeOption
	Variant         string
	Params          []ParamFieldViewModel
	ThingID         string
	Name            string
	Description     string
	Latitude        string
	Longitude       string
	ErrorMessage    string
}

// ThingV2EditViewModel är redigera-sidan för /things-v2/{id}?mode=edit.
// Mall + mallversion är låsta (visas, skickas aldrig): mallbyte kräver
// ta bort + skapa ny.
type ThingV2EditViewModel struct {
	ThingID         string
	Tenant          string
	Revision        int64
	TemplateDisplay string
	Variants        []featuresthings.TypeOption
	Variant         string
	Params          []ParamFieldViewModel
	Name            string
	Description     string
	Latitude        string
	Longitude       string
	ErrorMessage    string
}
