package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

type DeviceManagement interface {
	GetSensor(ctx context.Context, id string) (Sensor, error)
	GetSensors(ctx context.Context, offset, limit int, args map[string][]string) (SensorResult, error)
	UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error
	GetTenants(ctx context.Context) []string
	GetDeviceProfiles(ctx context.Context) []DeviceProfile
	GetStatistics(ctx context.Context) Statistics
	GetMeasurementInfo(ctx context.Context, id string) (MeasurementData, error)
	GetMeasurementData(ctx context.Context, id string, params ...InputParam) (MeasurementData, error)
	GetAlarms(ctx context.Context, offset, limit int, args map[string][]string) (AlarmResult, error)
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
	Alarms        []string       `json:"alarms,omitempty"`
}

func (s Sensor) ObservedAt() time.Time {
	if s.DeviceState != nil {
		return s.DeviceState.ObservedAt
	}
	return time.Time{}
}

type Alarm struct {
	DeviceID   string    `json:"deviceID"`
	ObservedAt time.Time `json:"observedAt"`
	Types      []string  `json:"types"`
}

type SensorResult struct {
	Sensors      []Sensor
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}

type AlarmResult struct {
	Alarms       []Alarm
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}

type MeasurementData struct {
	DeviceID string             `json:"deviceID"`
	Urn      *string            `json:"urn,omitempty"`
	Name     *string            `json:"name,omitempty"`
	Values   []MeasurementValue `json:"values,omitempty"`
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
	Count       uint64    `json:"sum,omitempty"`
}

func (a *App) GetSensor(ctx context.Context, id string) (Sensor, error) {
	res, err := a.get(ctx, a.deviceManagementURL, id, url.Values{})
	if err != nil {
		return Sensor{}, err
	}

	var sensor Sensor
	err = json.Unmarshal(res.Data, &sensor)
	if err != nil {
		return Sensor{}, err
	}

	return sensor, nil
}

func (a *App) GetSensors(ctx context.Context, offset, limit int, args map[string][]string) (SensorResult, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

	for k, v := range args {
		params[k] = v
	}

	res, err := a.get(ctx, a.deviceManagementURL, "", params)
	if err != nil {
		return SensorResult{}, err
	}

	var sensors []Sensor
	err = json.Unmarshal(res.Data, &sensors)
	if err != nil {
		return SensorResult{}, err
	}

	return SensorResult{
		Sensors:      sensors,
		TotalRecords: int(res.Meta.TotalRecords),
		Offset:       int(*res.Meta.Offset),
		Limit:        int(*res.Meta.Limit),
		Count:        len(sensors),
	}, nil
}

func (a *App) UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error {
	b, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	return a.patch(ctx, a.deviceManagementURL, deviceID, b)
}

func (a *App) GetTenants(ctx context.Context) []string {
	res, err := a.get(ctx, a.adminURL, "tenants", url.Values{})
	if err != nil {
		return []string{}
	}

	var tenants []string
	err = json.Unmarshal(res.Data, &tenants)
	if err != nil {
		return []string{}
	}

	return tenants
}
