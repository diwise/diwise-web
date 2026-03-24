package sensors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	legacydevices "github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/measurements"
	"github.com/diwise/diwise-web/internal/pkg/application"
	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/matryer/is"
)

func TestNewAttachSensorDialogHandlerRequiresSensorID(t *testing.T) {
	is := is.New(t)

	handler := NewAttachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, newTestDeviceApp())

	form := url.Values{"sensorType": {"decoder-x"}}
	req := httptest.NewRequest(http.MethodPost, "/v2/components/sensors/device-1/attach", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "SensorID kan inte vara tomt"))
}

func TestNewAttachSensorDialogHandlerRedirectsOnSuccess(t *testing.T) {
	is := is.New(t)

	app := newTestDeviceApp()
	callOrder := []string{}
	app.attachFunc = func(ctx context.Context, deviceID string) error {
		callOrder = append(callOrder, "attach:"+deviceID)
		sensorID, ok := legacydevices.AttachSensorIDFromContext(ctx)
		is.True(ok)
		is.Equal("sensor-123", sensorID)
		return nil
	}
	app.updateSensorFunc = func(_ context.Context, sensorID string, fields map[string]any) error {
		callOrder = append(callOrder, "update:"+sensorID)
		is.Equal("sensor-123", sensorID)
		is.Equal("sensor-123", fields["sensorID"])
		is.Equal("decoder-x", fields["sensorProfileID"])
		return nil
	}

	handler := NewAttachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, app)

	form := url.Values{
		"newSensorID": {"sensor-123"},
		"sensorType":  {"decoder-x"},
	}
	req := httptest.NewRequest(http.MethodPost, "/v2/components/sensors/device-1/attach", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusNoContent, rec.Code)
	is.Equal("/v2/sensors/device-1?mode=edit", rec.Header().Get("HX-Redirect"))
	is.Equal([]string{"attach:device-1", "update:sensor-123"}, callOrder)
}

func TestNewDetachSensorDialogHandlerReturnsDialogOnGet(t *testing.T) {
	is := is.New(t)

	handler := NewDetachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, newTestDeviceApp())

	req := httptest.NewRequest(http.MethodGet, "/v2/components/sensors/device-1/detach", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "Delete sensor"))
}

func TestNewDetachSensorDialogHandlerRedirectsOnSuccess(t *testing.T) {
	is := is.New(t)

	app := newTestDeviceApp()
	var detached string
	app.deattachFunc = func(_ context.Context, deviceID string) error {
		detached = deviceID
		return nil
	}

	handler := NewDetachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, app)

	req := httptest.NewRequest(http.MethodPost, "/v2/components/sensors/device-1/detach", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusNoContent, rec.Code)
	is.Equal("device-1", detached)
	is.Equal("/v2/sensors/device-1?mode=edit", rec.Header().Get("HX-Redirect"))
}

type testDeviceApp struct {
	device           application.Device
	deviceProfiles   []application.DeviceProfile
	measurements     []application.MeasurementValue
	attachFunc       func(ctx context.Context, deviceID string) error
	deattachFunc     func(ctx context.Context, deviceID string) error
	updateSensorFunc func(ctx context.Context, deviceID string, fields map[string]any) error
}

func newTestDeviceApp() *testDeviceApp {
	return &testDeviceApp{
		device: application.Device{
			DeviceID: "device-1",
			SensorID: "sensor-current",
			Name:     "Device One",
		},
		deviceProfiles: []application.DeviceProfile{
			{Name: "Weather", Decoder: "decoder-x", Types: &[]string{"urn:1"}},
		},
		measurements: []application.MeasurementValue{},
	}
}

func (a *testDeviceApp) GetDevice(_ context.Context, id string) (application.Device, error) {
	device := a.device
	device.DeviceID = id
	return device, nil
}

func (a *testDeviceApp) GetDevices(context.Context, int, int, map[string][]string) (legacydevices.DeviceResult, error) {
	return legacydevices.DeviceResult{}, nil
}

func (a *testDeviceApp) GetSensor(ctx context.Context, id string) (application.Sensor, error) {
	device, err := a.GetDevice(ctx, id)
	if err != nil {
		return application.Sensor{}, err
	}
	return application.Sensor{
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
		Metadata:      device.Metadata,
	}, nil
}

func (a *testDeviceApp) GetSensors(context.Context, int, int, map[string][]string) (application.SensorResult, error) {
	return application.SensorResult{}, nil
}

func (a *testDeviceApp) Attach(ctx context.Context, deviceID string) error {
	if a.attachFunc != nil {
		return a.attachFunc(ctx, deviceID)
	}
	return nil
}

func (a *testDeviceApp) Deattach(ctx context.Context, deviceID string) error {
	if a.deattachFunc != nil {
		return a.deattachFunc(ctx, deviceID)
	}
	return nil
}

func (a *testDeviceApp) GetSensorStatus(context.Context, string) ([]application.DeviceStatus, error) {
	return nil, nil
}

func (a *testDeviceApp) UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error {
	if a.updateSensorFunc != nil {
		return a.updateSensorFunc(ctx, deviceID, fields)
	}
	return nil
}

func (a *testDeviceApp) GetTenants(context.Context) []string { return []string{"tenant-a"} }

func (a *testDeviceApp) GetDeviceProfiles(context.Context) []application.DeviceProfile {
	return a.deviceProfiles
}

func (a *testDeviceApp) GetStatistics(context.Context) (application.Statistics, error) {
	return application.Statistics{}, nil
}

func (a *testDeviceApp) GetMeasurementInfo(context.Context, string) ([]application.MeasurementValue, error) {
	return a.measurements, nil
}

func (a *testDeviceApp) GetMeasurementData(context.Context, string, ...application.InputParam) (application.MeasurementData, error) {
	return measurements.Data{}, nil
}

func (a *testDeviceApp) GetAlarms(context.Context, int, int, map[string][]string) (application.AlarmResult, error) {
	return application.AlarmResult{}, nil
}

func testLocaleBundle() *ftkmock.LocaleBundleMock {
	return &ftkmock.LocaleBundleMock{
		ForFunc: func(string) frontendtoolkit.Localizer {
			return &ftkmock.LocalizerMock{
				GetFunc:         func(key string) string { return key },
				GetWithDataFunc: func(key string, _ map[string]any) string { return key },
			}
		},
	}
}
