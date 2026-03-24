package sensors

import "time"

type SensorsPageViewModel struct {
	Statistics     StatisticsViewModel
	Sensors        []SensorViewModel
	Paging         PagingViewModel
	DeviceProfiles []string
	Filters        FiltersViewModel
	MapView        bool
}

type SensorViewModel struct {
	Active       bool
	DeviceID     string
	DevEUI       string
	Name         string
	Type         string
	BatteryLevel int
	LastSeen     time.Time
	HasAlerts    bool
	Online       bool
	Latitude     float64
	Longitude    float64
}

type StatisticsViewModel struct {
	Total    int
	Active   int
	Inactive int
	Online   int
	Unknown  int
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
	Search        string
	LastSeen      string
	LastSeenDate  time.Time
	LocaleTag     string
	SelectedTypes []string
	Active        string
	Online        string
	PageSize      int
}

type SensorDetailsPageViewModel struct {
	DeviceID          string
	DevEUI            string
	Name              string
	Description       string
	DeviceProfileName string
	Tenant            string
	Environment       string
	Active            bool
	Online            bool
	Latitude          float64
	Longitude         float64
	Interval          int
	ObservedAt        time.Time
	Types             []string
	TypeOptions       []MeasurementTypeOption
	Organisations     []string
	DeviceProfiles    []DeviceProfileOption
	Metadata          []MetadataViewModel
	MeasurementTypes  []string
	Measurements      []MeasurementViewModel
	DeviceStatus      *DeviceStatusViewModel
}

type DeviceProfileOption struct {
	Name     string
	Decoder  string
	Interval int
	Types    []string
}

type MetadataViewModel struct {
	Key   string
	Value string
}

type MeasurementViewModel struct {
	ID        string
	Name      string
	Timestamp time.Time
	Unit      string
	Value     *float64
	BoolValue *bool
	String    string
}

type DeviceStatusViewModel struct {
	BatteryLevel int
	RSSI         *float64
	LoRaSNR      *float64
	Frequency    *int64
	DR           *int
	ObservedAt   time.Time
}

type MeasurementTypeOption struct {
	Value    string
	Label    string
	Selected bool
}

type MeasurementTypeOptionsProps struct {
	Options []MeasurementTypeOption
}

type AttachSensorDialogViewModel struct {
	DeviceID       string
	CurrentSensorID string
	SensorID       string
	SelectedType   string
	DeviceProfiles []DeviceProfileOption
	ErrorMessage   string
}

type DetachSensorDialogViewModel struct {
	DeviceID   string
	SensorID   string
	SensorName string
	ErrorMessage string
}
