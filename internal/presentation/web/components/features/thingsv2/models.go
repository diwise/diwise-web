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
	Parent                 *ThingV2ViewModel
	CanChangeParent        bool
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
// Sak-ID:t genereras serversidan (UUID) och matas aldrig in i formuläret.
type ThingV2CreateViewModel struct {
	Tenants         []string
	Tenant          string
	Templates       []featuresthings.TypeOption
	Template        string
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

// ThingV2EditViewModel är redigera-sidan för /things-v2/{id}?mode=edit.
// Mall + mallversion är låsta (visas, skickas aldrig): mallbyte kräver
// ta bort + skapa ny.
type ThingV2EditViewModel struct {
	ThingID         string
	Tenant          string
	Revision        int64
	TemplateDisplay string
	TemplateRef     string
	Variants        []featuresthings.TypeOption
	Variant         string
	Params          []ParamFieldViewModel
	Name            string
	Description     string
	Latitude        string
	Longitude       string
	ErrorMessage    string
}

// ThingV2DeleteViewModel är raderingsdialogen för /things-v2/{id}.
type ThingV2DeleteViewModel struct {
	ThingID      string
	Tenant       string
	Name         string
	ErrorMessage string
}

// SensorBindingViewModel är en bindningsbar ingång med nuvarande koppling
// (tom DeviceID = okopplad). En koppling per ingång.
type SensorBindingViewModel struct {
	Input      string
	Label      string
	Object     string
	Resource   string
	DeviceID   string
	DeviceName string
}

// ThingV2SensorsViewModel är hantera-sensorer-dialogen för en sak.
type ThingV2SensorsViewModel struct {
	ThingID      string
	Tenant       string
	Revision     int64
	Name         string
	Bindings     []SensorBindingViewModel
	ErrorMessage string
}

// SensorCandidateViewModel är en valbar enhet i sensorsökningen.
type SensorCandidateViewModel struct {
	DeviceID string
	Name     string
	Decoder  string
}

// ThingV2SensorResultsViewModel är sensorsökresultatet i dialogen.
type ThingV2SensorResultsViewModel struct {
	ThingID  string
	Tenant   string
	Revision int64
	Input    string
	Results  []SensorCandidateViewModel
}
