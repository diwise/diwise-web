package application

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type App struct {
	deviceManagementURL string
	thingManagementURL  string
	adminURL            string
	measurementURL      string
	cache               *Cache
}

func New(ctx context.Context) (*App, error) {
	deviceManagementURL := env.GetVariableOrDefault(ctx, "DEV_MGMT_URL", "https://test.diwise.io/api/v0/devices")
	thingManagementURL := strings.Replace(deviceManagementURL, "devices", "devices", 1)
	adminURL := strings.Replace(deviceManagementURL, "devices", "admin", 1)
	measurementURL := strings.Replace(deviceManagementURL, "devices", "measurements", 1)

	c := NewCache()
	c.Cleanup(60 * time.Second)

	return &App{
		deviceManagementURL: deviceManagementURL,
		thingManagementURL:  thingManagementURL,
		adminURL:            adminURL,
		measurementURL:      measurementURL,
		cache:               c,
	}, nil
}

func (a *App) GetThing(ctx context.Context, id string) (Thing, error) {
	res, err := a.get(ctx, a.thingManagementURL, id, url.Values{})
	if err != nil {
		return Thing{}, err
	}

	var sensor Thing
	err = json.Unmarshal(res.Data, &sensor)
	if err != nil {
		return Thing{}, err
	}

	return sensor, nil
}

func (a *App) GetThings(ctx context.Context, offset, limit int) (ThingResult, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

	res, err := a.get(ctx, a.thingManagementURL, "", params)
	if err != nil {
		return ThingResult{}, err
	}

	var things []Thing
	err = json.Unmarshal(res.Data, &things)
	if err != nil {
		return ThingResult{}, err
	}

	return ThingResult{
		Things:       things,
		TotalRecords: int(res.Meta.TotalRecords),
		Offset:       int(*res.Meta.Offset),
		Limit:        int(*res.Meta.Limit),
		Count:        len(things),
	}, nil
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

func (a *App) GetSensors(ctx context.Context, offset, limit int) (SensorResult, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

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

func (a *App) GetDeviceProfiles(ctx context.Context) []DeviceProfile {
	key := "/admin/deviceprofiles"
	if d, ok := a.cache.Get(key); ok {
		return d.([]DeviceProfile)
	}

	res, err := a.get(ctx, a.adminURL, "deviceprofiles", url.Values{})
	if err != nil {
		return []DeviceProfile{}
	}

	var deviceProfiles []DeviceProfile
	err = json.Unmarshal(res.Data, &deviceProfiles)
	if err != nil {
		return []DeviceProfile{}
	}

	a.cache.Set(key, deviceProfiles, 600*time.Second)

	return deviceProfiles
}

func (a *App) GetStatistics(ctx context.Context) Statistics {
	key := "/admin/statistics"
	if s, ok := a.cache.Get(key); ok {
		return s.(Statistics)
	}

	q := url.Values{}
	q.Add("limit", "1")

	s := Statistics{}

	total, err := a.get(ctx, a.deviceManagementURL, "", q)
	if err == nil {
		s.Total = int(total.Meta.TotalRecords)
	}

	q.Add("q", "%7B%20%22deviceState%22%3A%7B%22online%22%3Atrue%7D%7D%0A")
	online, err := a.get(ctx, a.deviceManagementURL, "", q)
	if err == nil {
		s.Online = int(online.Meta.TotalRecords)
	}

	q.Set("q", "%7B%22active%22%3Atrue%7D%0A")
	active, err := a.get(ctx, a.deviceManagementURL, "", q)
	if err == nil {
		s.Active = int(active.Meta.TotalRecords)
	}

	q.Set("q", "%7B%22deviceProfile%22%3A%20%7B%22name%22%3A%20%22unknown%22%7D%7D")
	unknown, err := a.get(ctx, a.deviceManagementURL, "", q)
	if err == nil {
		s.Inactive = int(total.Meta.TotalRecords - active.Meta.TotalRecords)
		s.Unknown = int(unknown.Meta.TotalRecords)
	}

	a.cache.Set(key, s, 600*time.Second)

	return s
}

func (a *App) GetMeasurementInfo(ctx context.Context, id string) (MeasurementData, error) {

	resp, err := a.get(ctx, a.measurementURL, id, url.Values{})
	if err != nil {
		return MeasurementData{}, err
	}

	var info MeasurementData
	err = json.Unmarshal(resp.Data, &info)
	if err != nil {
		return MeasurementData{}, err
	}

	return info, nil
}
func (a *App) GetMeasurementData(ctx context.Context, id string, params ...InputParam) (MeasurementData, error) {
	q := url.Values{}
	if id != "" {
		q.Add("id", id)
	}

	for _, p := range params {
		p(&q)
	}

	resp, err := a.get(ctx, a.measurementURL, "", q)
	if err != nil {
		return MeasurementData{}, err
	}

	var data MeasurementData
	err = json.Unmarshal(resp.Data, &data)
	if err != nil {
		return MeasurementData{}, err
	}

	return data, nil
}

func (a *App) get(ctx context.Context, baseUrl, path string, params url.Values) (*ApiResponse, error) {
	if strings.ContainsAny(path, "/") {
		path = strings.TrimPrefix(path, "/")
		path = strings.TrimSuffix(path, "/")
	}

	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseUrl, path), "/"))
	if err != nil {
		log.Error("could not parse url", "err", err.Error())
		return nil, err
	}

	u.RawQuery = params.Encode()
	token := authz.Token(ctx)
	urlToGet := u.String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlToGet, nil)
	if err != nil {
		log.Error("failed to create http request", "err", err.Error())
		err = fmt.Errorf("failed to create http request: %w", err)		
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := http.Client{
		Transport: otelhttp.NewTransport(&transport),
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("http request failed", "err", err.Error())
		err = fmt.Errorf("failed to retrieve information: %w", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("unauthorized")
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed", slog.Int("status_code", resp.StatusCode))
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return nil, err
	}

	impl := ApiResponse{}

	err = json.Unmarshal(respBody, &impl)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	return &impl, nil
}

func (a *App) patch(ctx context.Context, baseUrl, sensorID string, body []byte) error {
	log := logging.GetFromContext(ctx)

	u, err := url.Parse(strings.TrimSuffix(fmt.Sprintf("%s/%s", baseUrl, sensorID), "/"))
	if err != nil {
		return err
	}

	log.Debug("PATCH", slog.String("body", string(body)), slog.String("url", u.String()))

	token := authz.Token(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u.String(), bytes.NewReader(body))
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return err
	}

	req.Header.Add("Authorization", "Bearer "+token)

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := http.Client{
		Transport: otelhttp.NewTransport(&transport),
	}

	resp, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to patch: %w", err)
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return err
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return err
	}

	return nil
}

type Meta struct {
	TotalRecords uint64  `json:"totalRecords"`
	Offset       *uint64 `json:"offset,omitempty"`
	Limit        *uint64 `json:"limit,omitempty"`
	Count        uint64  `json:"count"`
}

type Links struct {
	Self  *string `json:"self,omitempty"`
	First *string `json:"first,omitempty"`
	Prev  *string `json:"prev,omitempty"`
	Next  *string `json:"next,omitempty"`
	Last  *string `json:"last,omitempty"`
}

type ApiResponse struct {
	Meta  *Meta           `json:"meta,omitempty"`
	Data  json.RawMessage `json:"data"`
	Links *Links          `json:"links,omitempty"`
}
