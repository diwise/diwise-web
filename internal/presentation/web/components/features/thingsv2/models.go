package thingsv2

import (
	"encoding/json"

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
