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
	adminURL := strings.Replace(deviceManagementURL, "devices", "admin", 1)
	thingManagementURL := env.GetVariableOrDefault(ctx, "THINGS_URL", "https://test.diwise.io/api/v0/things")
	measurementURL := env.GetVariableOrDefault(ctx, "MEASUREMENTS_URL", "https://test.diwise.io/api/v0/measurements")

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

func (a *App) GetTags(ctx context.Context) ([]string, error) {
	res, err := a.get(ctx, a.thingManagementURL, "tags", url.Values{})
	if err != nil {
		return []string{}, err
	}

	var tags []string
	err = json.Unmarshal(res.Data, &tags)
	if err != nil {
		return []string{}, err
	}

	return tags, nil
}

func (a *App) GetTypes(ctx context.Context) ([]string, error) {
	res, err := a.get(ctx, a.thingManagementURL, "types", url.Values{})
	if err != nil {
		return []string{}, err
	}

	var tags []string
	err = json.Unmarshal(res.Data, &tags)
	if err != nil {
		return []string{}, err
	}

	return tags, nil
}

func (a *App) GetThing(ctx context.Context, id string) (Thing, error) {
	params := url.Values{
		"measurements": []string{"true"},
		"state":        []string{"true"},
	}

	res, err := a.get(ctx, a.thingManagementURL, id, params)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	err = json.Unmarshal(res.Data, &thing)
	if err != nil {
		return Thing{}, err
	}

	if len(res.Included) > 0 {
		for _, r := range res.Included {
			thing.Related = append(thing.Related, Thing{
				ID:   r.ID,
				Type: r.Type,
			})
		}
	}

	return thing, nil
}

func (a *App) GetThings(ctx context.Context, offset, limit int, args map[string][]string) (ThingResult, error) {
	params := url.Values{
		"type":         []string{"combinedsewageoverflow", "wastecontainer", "sewer", "sewagepumpingstation", "passage"},
		"measurements": []string{"true"},
	}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

	for k, v := range args {
		params[k] = v
	}

	res, err := a.get(ctx, a.thingManagementURL, "", params)
	if err != nil {
		return ThingResult{}, err
	}

	var things []Thing
	err = json.Unmarshal(res.Data, &things)
	if err != nil {
		return ThingResult{}, err
	}

	var total, off, lim int
	off = offset
	lim = limit

	if res.Meta != nil {
		total = int(res.Meta.TotalRecords)
		if res.Meta.Limit != nil {
			lim = int(*res.Meta.Limit)
		}
		if res.Meta.Offset != nil {
			off = int(*res.Meta.Offset)
		}
	}

	return ThingResult{
		Things:       things,
		TotalRecords: total,
		Offset:       off,
		Limit:        lim,
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

	s := Statistics{}

	count := func(key, value string, ch chan int) {
		params := url.Values{}
		params.Add("limit", "1")

		if key != "" && value != "" {
			params.Add(key, value)
		}

		res, err := a.get(ctx, a.deviceManagementURL, "", params)
		if err != nil {
			ch <- 0
		}
		ch <- int(res.Meta.TotalRecords)
	}

	total := make(chan int)
	online := make(chan int)
	active := make(chan int)
	inactive := make(chan int)
	unknown := make(chan int)

	go count("", "", total)
	go count("online", "true", online)
	go count("active", "true", active)
	go count("active", "false", inactive)
	go count("profilename", "unknown", unknown)

	s.Total = <-total
	s.Online = <-online
	s.Active = <-active
	s.Inactive = <-inactive
	s.Unknown = <-unknown

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
		log.Error("failed to create http request", slog.String("url", urlToGet), "err", err.Error())
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
		log.Error("http request failed", slog.String("url", urlToGet), "err", err.Error())
		err = fmt.Errorf("failed to retrieve information: %w", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		log.Error("unauthorized", slog.String("url", urlToGet))
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		log.Error("request failed", slog.String("url", urlToGet), slog.Int("status_code", resp.StatusCode))
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return nil, err
	}

	if string(respBody) == "[]" {
		var arr json.RawMessage
		json.Unmarshal(respBody, &arr)
		return &ApiResponse{
			Meta:  nil,
			Data:  arr,
			Links: nil,
		}, nil
	}

	impl := ApiResponse{}

	err = json.Unmarshal(respBody, &impl)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	log.Debug("body", slog.Any("data", impl.Data))

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

type Resource struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type ApiResponse struct {
	Meta     *Meta           `json:"meta,omitempty"`
	Data     json.RawMessage `json:"data"`
	Links    *Links          `json:"links,omitempty"`
	Included []Resource      `json:"included,omitempty"`
}
