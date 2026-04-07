package sensors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/diwise/diwise-web/internal/application/admin"
	"github.com/diwise/diwise-web/internal/application/alarms"
	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/measurements"
	frontendtoolkit "github.com/diwise/frontend-toolkit"
	ftkmock "github.com/diwise/frontend-toolkit/mock"
	"github.com/matryer/is"
)

func TestNewAttachSensorDialogHandlerRequiresSensorID(t *testing.T) {
	is := is.New(t)

	handler := NewAttachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, newTestDeviceApp())

	form := url.Values{"sensorType": {"decoder-x"}}
	req := httptest.NewRequest(http.MethodPost, "/components/sensors/device-1/attach", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusBadRequest, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "SensorID kan inte vara tomt"))
}

func TestNewAttachSensorDialogHandlerRefreshesEditPageOnSuccess(t *testing.T) {
	is := is.New(t)

	app := newTestDeviceApp()
	callOrder := []string{}
	app.attachFunc = func(ctx context.Context, deviceID string) error {
		callOrder = append(callOrder, "attach:"+deviceID)
		sensorID, ok := devices.AttachSensorIDFromContext(ctx)
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
	req := httptest.NewRequest(http.MethodPost, "/components/sensors/device-1/attach", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("#sensor-edit-page", rec.Header().Get("HX-Retarget"))
	is.Equal("outerHTML", rec.Header().Get("HX-Reswap"))
	is.True(strings.Contains(rec.Body.String(), `id="sensor-edit-page"`))
	is.Equal([]string{"attach:device-1", "update:sensor-123"}, callOrder)
}

func TestNewAttachSensorDialogHandlerReturnsConflictForAttachConflict(t *testing.T) {
	is := is.New(t)

	app := newTestDeviceApp()
	app.attachFunc = func(context.Context, string) error {
		return client.ErrConflict
	}

	handler := NewAttachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, app)

	form := url.Values{
		"newSensorID": {"sensor-123"},
		"sensorType":  {"decoder-x"},
	}
	req := httptest.NewRequest(http.MethodPost, "/components/sensors/device-1/attach", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusConflict, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "SensorID är redan kopplad till en annan enhet"))
}

func TestNewAttachSensorDialogHandlerReturnsNotFoundForMissingSensorOnUpdate(t *testing.T) {
	is := is.New(t)

	app := newTestDeviceApp()
	app.updateSensorFunc = func(context.Context, string, map[string]any) error {
		return client.ErrNotFound
	}

	handler := NewAttachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, app)

	form := url.Values{
		"newSensorID": {"sensor-123"},
		"sensorType":  {"decoder-x"},
	}
	req := httptest.NewRequest(http.MethodPost, "/components/sensors/device-1/attach", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusNotFound, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "Sensorn hittades inte"))
}

func TestNewDetachSensorDialogHandlerReturnsDialogOnGet(t *testing.T) {
	is := is.New(t)

	handler := NewDetachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, newTestDeviceApp())

	req := httptest.NewRequest(http.MethodGet, "/components/sensors/device-1/detach", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.True(strings.Contains(rec.Body.String(), "Delete sensor"))
}

func TestNewDetachSensorDialogHandlerRefreshesEditPageOnSuccess(t *testing.T) {
	is := is.New(t)

	app := newTestDeviceApp()
	var detached string
	app.deattachFunc = func(_ context.Context, deviceID string) error {
		detached = deviceID
		return nil
	}

	handler := NewDetachSensorDialogHandler(context.Background(), testLocaleBundle(), nil, app)

	req := httptest.NewRequest(http.MethodPost, "/components/sensors/device-1/detach", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", "device-1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	is.Equal(http.StatusOK, rec.Code)
	is.Equal("device-1", detached)
	is.Equal("#sensor-edit-page", rec.Header().Get("HX-Retarget"))
	is.Equal("outerHTML", rec.Header().Get("HX-Reswap"))
	is.True(strings.Contains(rec.Body.String(), `id="sensor-edit-page"`))
}

type testDeviceApp struct {
	device           devices.Device
	deviceProfiles   []devices.SensorProfile
	measurements     []measurements.Value
	attachFunc       func(ctx context.Context, deviceID string) error
	deattachFunc     func(ctx context.Context, deviceID string) error
	updateSensorFunc func(ctx context.Context, deviceID string, fields map[string]any) error
}

func newTestDeviceApp() *testDeviceApp {
	return &testDeviceApp{
		device: devices.Device{
			DeviceID: "device-1",
			SensorID: "sensor-current",
			Name:     "Device One",
		},
		deviceProfiles: []devices.SensorProfile{
			{Name: "Weather", Decoder: "decoder-x", Types: &[]string{"urn:1"}},
		},
		measurements: []measurements.Value{},
	}
}

func (a *testDeviceApp) GetDevice(_ context.Context, id string) (devices.Device, error) {
	device := a.device
	device.DeviceID = id
	return device, nil
}

func (a *testDeviceApp) GetDevices(context.Context, int, int, map[string][]string) (devices.DeviceResult, error) {
	return devices.DeviceResult{}, nil
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

func (a *testDeviceApp) GetSensorStatus(context.Context, string) ([]devices.SensorStatus, error) {
	return nil, nil
}

func (a *testDeviceApp) UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error {
	if a.updateSensorFunc != nil {
		return a.updateSensorFunc(ctx, deviceID, fields)
	}
	return nil
}

func (a *testDeviceApp) UpdateDevice(context.Context, string, map[string]any) error {
	return nil
}

func (a *testDeviceApp) GetTenants(context.Context) []string { return []string{"tenant-a"} }

func (a *testDeviceApp) GetDeviceProfiles(context.Context) []devices.SensorProfile {
	return a.deviceProfiles
}

func (a *testDeviceApp) GetStatistics(context.Context) (devices.Statistics, error) {
	return devices.Statistics{}, nil
}

func (a *testDeviceApp) GetMeasurementInfo(context.Context, string) ([]measurements.Value, error) {
	return a.measurements, nil
}

func (a *testDeviceApp) GetMeasurementData(context.Context, string, ...client.InputParam) (measurements.Data, error) {
	return measurements.Data{}, nil
}

func (a *testDeviceApp) GetAlarms(context.Context, int, int, map[string][]string) (alarms.Result, error) {
	return alarms.Result{}, nil
}

var _ admin.Management = (*testDeviceApp)(nil)

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
