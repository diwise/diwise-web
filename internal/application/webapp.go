package application

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/application/admin"
	"github.com/diwise/diwise-web/internal/application/alarms"
	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/measurements"
	"github.com/diwise/diwise-web/internal/application/things"
	"github.com/diwise/diwise-web/internal/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app")

type App struct {
	client       *client.Client
	admin        *admin.Service
	alarms       *alarms.Service
	devices      *devices.Service
	measurements *measurements.Service
	things       *things.Service
}

func New(ctx context.Context, devmgmt, thingsURL, adminURL, alarmsURL, measurementURL string) (*App, error) {
	_ = ctx
	client := client.NewClient(devmgmt, thingsURL, adminURL, alarmsURL, measurementURL)
	return &App{
		client:       client,
		admin:        admin.NewService(client),
		alarms:       alarms.NewService(client),
		devices:      devices.NewService(client),
		measurements: measurements.NewService(client),
		things:       things.NewService(client),
	}, nil
}

func WithReverse(reverse bool) client.InputParam { return client.WithReverse(reverse) }
func WithLimit(limit int) client.InputParam      { return client.WithLimit(limit) }
func WithLastN(lastN bool) client.InputParam     { return client.WithLastN(lastN) }
func WithTimeRel(timeRel string, timeAt, endTimeAt time.Time) client.InputParam {
	return client.WithTimeRel(timeRel, timeAt, endTimeAt)
}
func WithAggrMethods(methods ...string) client.InputParam { return client.WithAggrMethods(methods...) }
func WithTimeUnit(timeUnit string) client.InputParam      { return client.WithTimeUnit(timeUnit) }
func WithAfter(timeAt time.Time) client.InputParam        { return client.WithAfter(timeAt) }
func WithBoolValue(boolValue bool) client.InputParam      { return client.WithBoolValue(boolValue) }

func (a *App) GetDevice(ctx context.Context, id string) (devices.Device, error) {
	return a.devices.GetDevice(ctx, id)
}

func (a *App) GetDevices(ctx context.Context, offset, limit int, args map[string][]string) (devices.DeviceResult, error) {
	return a.devices.GetDevices(ctx, offset, limit, args)
}

func (a *App) GetSensors(ctx context.Context, offset, limit int, args map[string][]string) (devices.SensorResult, error) {
	return a.devices.GetSensors(ctx, offset, limit, args)
}

func (a *App) UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error {
	return a.devices.UpdateDevice(ctx, deviceID, fields)
}

func (a *App) UpdateSensor(ctx context.Context, sensorID string, fields map[string]any) error {
	return a.devices.UpdateSensor(ctx, sensorID, fields)
}

func (a *App) Attach(ctx context.Context, deviceID string) error {
	return a.devices.Attach(ctx, deviceID)
}

func (a *App) Deattach(ctx context.Context, deviceID string) error {
	return a.devices.Deattach(ctx, deviceID)
}

func (a *App) GetSensorStatus(ctx context.Context, id string) ([]devices.SensorStatus, error) {
	return a.devices.GetSensorStatus(ctx, id)
}

func (a *App) GetTenants(ctx context.Context) []string {
	return a.admin.GetTenants(ctx)
}

func (a *App) GetDeviceProfiles(ctx context.Context) []devices.SensorProfile {
	return a.admin.GetDeviceProfiles(ctx)
}

func (a *App) GetStatistics(ctx context.Context) (devices.Statistics, error) {
	return a.devices.GetStatistics(ctx)
}

func (a *App) GetMeasurementInfo(ctx context.Context, id string) ([]measurements.Value, error) {
	return a.measurements.GetMeasurementInfo(ctx, id)
}

func (a *App) GetMeasurementData(ctx context.Context, id string, params ...client.InputParam) (measurements.Data, error) {
	return a.measurements.GetMeasurementData(ctx, id, params...)
}

func (a *App) GetAlarms(ctx context.Context, offset, limit int, args map[string][]string) (alarms.Result, error) {
	return a.alarms.GetAlarms(ctx, offset, limit, args)
}

func (a *App) NewThing(ctx context.Context, t things.Thing) error {
	return a.things.NewThing(ctx, t)
}

func (a *App) GetThing(ctx context.Context, id string, params map[string][]string) (things.Thing, error) {
	return a.things.GetThing(ctx, id, params)
}

func (a *App) GetLatestValues(ctx context.Context, thingID string) ([]things.Measurement, error) {
	return a.things.GetLatestValues(ctx, thingID)
}

func (a *App) GetThings(ctx context.Context, offset, limit int, params map[string][]string) (things.Result, error) {
	return a.things.GetThings(ctx, offset, limit, params)
}

func (a *App) UpdateThing(ctx context.Context, thingID string, fields map[string]any) error {
	return a.things.UpdateThing(ctx, thingID, fields)
}

func (a *App) DeleteThing(ctx context.Context, thingID string) error {
	return a.things.DeleteThing(ctx, thingID)
}

func (a *App) GetTags(ctx context.Context) ([]string, error) {
	return a.things.GetTags(ctx)
}

func (a *App) GetTypes(ctx context.Context) ([]string, error) {
	return a.things.GetTypes(ctx)
}

func (a *App) GetValidSensors(ctx context.Context, urns []string) ([]things.SensorIdentifier, error) {
	return a.things.GetValidSensors(ctx, urns)
}

func (a *App) ConnectSensor(ctx context.Context, thingID string, refDevices []string) error {
	return a.things.ConnectSensor(ctx, thingID, refDevices)
}

func (a *App) Export(ctx context.Context, params url.Values) ([]byte, error) {
	var err error
	ctx, span := tracer.Start(ctx, "export")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	query, _ := url.ParseQuery(params.Encode())
	export := query.Get("export")
	if export == "" {
		return nil, fmt.Errorf("export parameter is missing")
	}

	accept := query.Get("accept")
	if accept == "" {
		return nil, fmt.Errorf("accept parameter is missing")
	}

	targetURL := ""
	helpers.SanitizeParams(query, "limit", "offset", "mapview", "export", "accept", "redirected")

	switch export {
	case "devices":
		targetURL = a.client.DeviceManagementURL()
	case "things":
		if query.Has("type") {
			t := query.Get("type")
			if strings.Contains(t, "-") {
				parts := strings.Split(t, "-")
				query.Set("type", parts[0])
				query.Set("subType", parts[1])
			}
		}
		targetURL = a.client.ThingManagementURL()
	case "thing":
		if query.Has("tab") {
			query.Set("n", strings.ReplaceAll(query.Get("tab"), "-", "/"))
			query.Del("tab")
		}
		if query.Has("timeAt") {
			timeAt := query.Get("timeAt")
			if len(timeAt) == len("0000-00-00T00:00") {
				timeAt += ":00Z"
				query.Set("timeAt", timeAt)
			}
			query.Set("timerel", "after")
		}
		if query.Has("endTimeAt") {
			endTimeAt := query.Get("endTimeAt")
			if len(endTimeAt) == len("0000-00-00T00:00") {
				endTimeAt += ":59Z"
				query.Set("endTimeAt", endTimeAt)
			}
			query.Set("timerel", query.Get("before"))
		}
		if query.Has("timeAt") && query.Has("endTimeAt") {
			query.Set("timerel", "between")
		}
		if !query.Has("limit") {
			query.Set("limit", strconv.Itoa(math.MaxInt32))
		}
		targetURL = a.client.ThingManagementURL() + "/values"
	default:
		return nil, fmt.Errorf("export parameter is invalid")
	}

	headers := map[string][]string{
		"Authorization": {"Bearer " + authz.Token(ctx)},
		"Accept":        {accept},
	}
	query.Add("export", "true")

	b, err := helpers.GET(ctx, targetURL, headers, query)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (a *App) Import(ctx context.Context, t string, f io.Reader) error {
	var err error
	ctx, span := tracer.Start(ctx, "import")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	headers := map[string][]string{
		"Authorization": {"Bearer " + authz.Token(ctx)},
	}

	targetURL := ""
	switch t {
	case "devices":
		targetURL = a.client.DeviceManagementURL()
	case "things":
		targetURL = a.client.ThingManagementURL()
	}

	err = helpers.FileUpload(ctx, targetURL, headers, f)
	return err
}
