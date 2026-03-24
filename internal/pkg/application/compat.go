package application

import (
	"context"
	"io"
	"net/url"
	"time"

	base "github.com/diwise/diwise-web/internal/application"
	"github.com/diwise/diwise-web/internal/application/alarms"
	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/measurements"
	"github.com/diwise/diwise-web/internal/application/things"
)

type App struct {
	*base.App
}

func New(ctx context.Context, devmgmt, thingsURL, adminURL, alarmsURL, measurementURL string) (*App, error) {
	app, err := base.New(ctx, devmgmt, thingsURL, adminURL, alarmsURL, measurementURL)
	if err != nil {
		return nil, err
	}

	return Wrap(app), nil
}

func Wrap(app *base.App) *App {
	if app == nil {
		return nil
	}

	return &App{App: app}
}

type InputParam = client.InputParam

type Location = client.Location
type Metadata = client.Metadata

type Meta = client.Meta
type Links = client.Links
type Resource = client.Resource
type ApiResponse = client.ApiResponse

type Statistics = devices.Statistics
type Type = devices.Type
type DeviceProfile = devices.SensorProfile
type DeviceStatus = devices.SensorStatus
type DeviceState = devices.DeviceState
type Device = devices.Device

type Sensor struct {
	Active        bool
	SensorID      string
	DeviceID      string
	Tenant        string
	Name          string
	Description   string
	Location      Location
	Environment   *string
	Types         []Type
	DeviceProfile *DeviceProfile
	DeviceStatus  *DeviceStatus
	DeviceState   *DeviceState
	Alarms        []string
	Metadata      []Metadata
}

type MeasurementData = measurements.Data
type MeasurementValue = measurements.Value

type Alarm = alarms.Alarm
type AlarmResult = alarms.Result

type RefDevice = things.RefDevice
type Measurement = things.Measurement
type Thing = things.Thing
type ThingTypeValues = things.TypeValues
type SensorIdentifier = things.SensorIdentifier
type ThingResult = things.Result

type SensorResult struct {
	Sensors      []Sensor
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}

type DeviceManagement interface {
	GetDevice(ctx context.Context, id string) (Device, error)
	GetDevices(ctx context.Context, offset, limit int, args map[string][]string) (devices.DeviceResult, error)
	GetSensor(ctx context.Context, id string) (Sensor, error)
	GetSensors(ctx context.Context, offset, limit int, args map[string][]string) (SensorResult, error)
	Attach(ctx context.Context, deviceID string) error
	Deattach(ctx context.Context, deviceID string) error
	GetSensorStatus(ctx context.Context, id string) ([]DeviceStatus, error)
	UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error
	GetTenants(ctx context.Context) []string
	GetDeviceProfiles(ctx context.Context) []DeviceProfile
	GetStatistics(ctx context.Context) (Statistics, error)
	GetMeasurementInfo(ctx context.Context, id string) ([]MeasurementValue, error)
	GetMeasurementData(ctx context.Context, id string, params ...InputParam) (MeasurementData, error)
	GetAlarms(ctx context.Context, offset, limit int, args map[string][]string) (AlarmResult, error)
}

type ThingManagement interface {
	NewThing(ctx context.Context, t Thing) error
	GetThing(ctx context.Context, id string, params map[string][]string) (Thing, error)
	GetLatestValues(ctx context.Context, thingID string) ([]Measurement, error)
	GetThings(ctx context.Context, offset, limit int, params map[string][]string) (ThingResult, error)
	UpdateThing(ctx context.Context, thingID string, fields map[string]any) error
	DeleteThing(ctx context.Context, thingID string) error
	GetTenants(ctx context.Context) []string
	GetTags(ctx context.Context) ([]string, error)
	GetTypes(ctx context.Context) ([]string, error)
	GetValidSensors(ctx context.Context, urns []string) ([]SensorIdentifier, error)
	ConnectSensor(ctx context.Context, thingID string, refDevices []string) error
	Export(ctx context.Context, params url.Values) ([]byte, error)
	Import(ctx context.Context, t string, f io.Reader) error
}

var ErrNotFound = client.ErrNotFound
var ErrUnauthorized = client.ErrUnauthorized
var ErrConflict = client.ErrConflict

func WithReverse(reverse bool) InputParam { return client.WithReverse(reverse) }
func WithLimit(limit int) InputParam      { return client.WithLimit(limit) }
func WithLastN(lastN bool) InputParam     { return client.WithLastN(lastN) }

func WithTimeRel(timeRel string, timeAt, endTimeAt time.Time) InputParam {
	return client.WithTimeRel(timeRel, timeAt, endTimeAt)
}

func WithAggrMethods(methods ...string) InputParam { return client.WithAggrMethods(methods...) }
func WithTimeUnit(timeUnit string) InputParam      { return client.WithTimeUnit(timeUnit) }
func WithAfter(timeAt time.Time) InputParam        { return client.WithAfter(timeAt) }
func WithBoolValue(boolValue bool) InputParam      { return client.WithBoolValue(boolValue) }

func (a *App) GetSensor(ctx context.Context, id string) (Sensor, error) {
	device, err := a.App.GetDevice(ctx, id)
	if err != nil {
		return Sensor{}, err
	}

	return sensorFromDevice(device), nil
}

func (a *App) GetSensors(ctx context.Context, offset, limit int, args map[string][]string) (SensorResult, error) {
	result, err := a.App.GetDevices(ctx, offset, limit, args)
	if err != nil {
		return SensorResult{}, err
	}

	sensors := make([]Sensor, 0, len(result.Devices))
	for _, device := range result.Devices {
		sensors = append(sensors, sensorFromDevice(device))
	}

	return SensorResult{
		Sensors:      sensors,
		TotalRecords: result.TotalRecords,
		Count:        result.Count,
		Offset:       result.Offset,
		Limit:        result.Limit,
	}, nil
}

func (a *App) GetDevice(ctx context.Context, id string) (Device, error) {
	return a.App.GetDevice(ctx, id)
}

func (a *App) GetDevices(ctx context.Context, offset, limit int, args map[string][]string) (devices.DeviceResult, error) {
	return a.App.GetDevices(ctx, offset, limit, args)
}

func (a *App) Attach(ctx context.Context, deviceID string) error {
	return a.App.Attach(ctx, deviceID)
}

func (a *App) Deattach(ctx context.Context, deviceID string) error {
	return a.App.Deattach(ctx, deviceID)
}

func (s Sensor) ObservedAt() time.Time {
	if s.DeviceState != nil {
		return s.DeviceState.ObservedAt
	}
	if s.DeviceStatus != nil {
		return s.DeviceStatus.ObservedAt
	}
	return time.Time{}
}

func sensorFromDevice(device devices.Device) Sensor {
	return Sensor{
		Active:        device.Active,
		SensorID:      device.SensorID,
		DeviceID:      device.DeviceID,
		Tenant:        device.Tenant,
		Name:          device.Name,
		Description:   device.Description,
		Location:      device.Location,
		Environment:   device.Environment,
		Types:         device.Types,
		DeviceProfile: device.SensorProfile,
		DeviceStatus:  device.SensorStatus,
		DeviceState:   device.DeviceState,
		Alarms:        device.Alarms,
		Metadata:      device.Metadata,
	}
}

var _ DeviceManagement = (*App)(nil)
var _ ThingManagement = (*App)(nil)
