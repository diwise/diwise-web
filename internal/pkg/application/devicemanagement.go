package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

type DeviceManagement interface {
	GetSensor(ctx context.Context, id string) (Sensor, error)
	GetSensors(ctx context.Context, offset, limit int, args map[string][]string) (SensorResult, error)
	GetSensorStatus(ctx context.Context, id string) ([]DeviceStatus, error)
	UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error
	GetTenants(ctx context.Context) []string
	GetDeviceProfiles(ctx context.Context) []DeviceProfile
	GetStatistics(ctx context.Context) (Statistics, error)
	GetMeasurementInfo(ctx context.Context, id string) ([]MeasurementValue, error)
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
	BatteryLevel    int       `json:"batteryLevel,omitzero"`
	RSSI            *float64  `json:"rssi,omitempty"`
	LoRaSNR         *float64  `json:"loRaSNR,omitempty"`
	Frequency       *int64    `json:"frequency,omitempty"`
	SpreadingFactor *float64  `json:"spreadingFactor,omitempty"`
	DR              *int      `json:"dr,omitempty"`
	ObservedAt      time.Time `json:"observedAt"`
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
	Metadata      []Metadata     `json:"metadata,omitempty"`
}

type Metadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
	Types      []string  `json:"alarms"`
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
	var err error
	ctx, span := tracer.Start(ctx, "get-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var res *ApiResponse
	res, err = a.get(ctx, a.deviceManagementURL, id, url.Values{})
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
	var err error
	ctx, span := tracer.Start(ctx, "get-sensors")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

	for k, v := range args {
		params[k] = v
	}

	var res *ApiResponse
	res, err = a.get(ctx, a.deviceManagementURL, "", params)
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

func (a *App) GetSensorStatus(ctx context.Context, id string) ([]DeviceStatus, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-sensor-status")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var res *ApiResponse
	res, err = a.get(ctx, a.deviceManagementURL, fmt.Sprintf("/%s/status", id), url.Values{})
	if err != nil {
		return []DeviceStatus{}, err
	}

	var statuses []DeviceStatus
	err = json.Unmarshal(res.Data, &statuses)
	if err != nil {
		return []DeviceStatus{}, err
	}

	return statuses, nil
}

func (a *App) UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error {
	var err error
	ctx, span := tracer.Start(ctx, "update-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var b []byte
	b, err = json.Marshal(fields)
	if err != nil {
		return err
	}

	err = a.patch(ctx, a.deviceManagementURL, deviceID, b)
	return err
}

func (a *App) GetTenants(ctx context.Context) []string {
	var err error
	ctx, span := tracer.Start(ctx, "get-tenants")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var res *ApiResponse
	res, err = a.get(ctx, a.adminURL, "tenants", url.Values{})
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
