package application

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

type DeviceManagement interface {
	GetSensor(ctx context.Context, id string) (Sensor, error)
	GetSensors(ctx context.Context, offset, limit int) (SensorResult, error)
	UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error
	GetTenants(ctx context.Context) []string
	GetDeviceProfiles(ctx context.Context) []DeviceProfile
	GetStatistics(ctx context.Context) Statistics
	GetMeasurementInfo(ctx context.Context, id string) (MeasurementData, error)
	GetMeasurementData(ctx context.Context, id string, params ...InputParam) (MeasurementData, error)
}

type Statistics struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Inactive int `json:"inactive"`
	Online   int `json:"online"`
	Unknown  int `json:"unknown"`
}

type Location struct {
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type Type struct {
	URN  string `json:"urn,omitempty"`
	Name string `json:"name"`
}

type DeviceProfile struct {
	Name     string    `json:"name"`
	Decoder  string    `json:"decoder,omitempty"`
	Interval int       `json:"interval,omitempty"`
	Types    *[]string `json:"types,omitempty"`
}

type DeviceStatus struct {
	BatteryLevel int       `json:"batteryLevel,omitempty"`
	ObservedAt   time.Time `json:"observedAt,omitempty"`
}

type DeviceState struct {
	Online     bool      `json:"online,omitempty"`
	State      int       `json:"state,omitempty"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
}

type Sensor struct {
	Active        bool           `json:"active"`
	SensorID      string         `json:"sensorID,omitempty"`
	DeviceID      string         `json:"deviceID"`
	Tenant        string         `json:"tenant,omitempty"`
	Name          string         `json:"name,omitempty"`
	Description   string         `json:"description,omitempty"`
	Location      Location       `json:"location,omitempty"`
	Environment   *string        `json:"environment,omitempty"`
	Types         []Type         `json:"types,omitempty"`
	DeviceProfile *DeviceProfile `json:"deviceProfile,omitempty"`
	DeviceStatus  *DeviceStatus  `json:"deviceStatus,omitempty"`
	DeviceState   *DeviceState   `json:"deviceState,omitempty"`
}

type SensorResult struct {
	Sensors      []Sensor
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}

type InputParam func(v *url.Values)

func WithReverse(reverse bool) InputParam {
	return func(v *url.Values) {
		v.Del("reverse")
		v.Add("reverse", fmt.Sprintf("%t", reverse))
	}
}
func WithLimit(limit int) InputParam {
	return func(v *url.Values) {
		v.Del("limit")
		v.Add("limit", fmt.Sprintf("%d", limit))
	}
}
func WithLastN(lastN bool) InputParam {
	return func(v *url.Values) {
		v.Del("lastN")
		v.Add("lastN", fmt.Sprintf("%t", lastN))
	}
}

type MeasurementData struct {
	DeviceID     string             `json:"deviceID"`
	Urn          *string            `json:"urn,omitempty"`
	Name         *string            `json:"name,omitempty"`
	Measurements []MeasurementInfo  `json:"measurements,omitempty"`
	Values       []MeasurementValue `json:"values,omitempty"`
}

type MeasurementInfo struct {
	ID string `json:"id"`
}

type MeasurementValue struct {
	ID          *string   `json:"id,omitempty"`
	Name        *string   `json:"n,omitempty"`
	BoolValue   *bool     `json:"vb,omitempty"`
	StringValue string    `json:"vs,omitempty"`
	Value       *float64  `json:"v,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Link        *string   `json:"link,omitempty"`
}
