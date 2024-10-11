package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/diwise/service-chassis/pkg/infrastructure/env"
)

type App struct {
	deviceManagementURL string
	thingManagementURL  string
	adminURL            string
	measurementURL      string
	alarmsURL           string
	cache               *Cache
}

func New(ctx context.Context) (*App, error) {
	deviceManagementURL := env.GetVariableOrDefault(ctx, "DEV_MGMT_URL", "https://test.diwise.io/api/v0/devices")
	adminURL := strings.Replace(deviceManagementURL, "devices", "admin", 1)
	alarmsURL := strings.Replace(deviceManagementURL, "devices", "alarms", 1)
	thingManagementURL := env.GetVariableOrDefault(ctx, "THINGS_URL", "https://test.diwise.io/api/v0/things")
	measurementURL := env.GetVariableOrDefault(ctx, "MEASUREMENTS_URL", "https://test.diwise.io/api/v0/measurements")

	c := NewCache()
	c.Cleanup(60 * time.Second)

	return &App{
		deviceManagementURL: deviceManagementURL,
		thingManagementURL:  thingManagementURL,
		adminURL:            adminURL,
		alarmsURL:           alarmsURL,
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

func (a *App) ConnectSensor(ctx context.Context, thingID, currentID, newID string) error {
	thing, err := a.GetThing(ctx, thingID)
	if err != nil {
		return err
	}

	sensorThing := Thing{
		ID:     newID,
		Type:   "device",
		Tenant: thing.Tenant,
	}

	sensor, err := a.GetSensor(ctx, newID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		sensorThing.Location = thing.Location
	} else {
		sensorThing.Location = sensor.Location
	}

	if currentID != "" {
		urlToDelete := a.thingManagementURL + "/" + thingID + "/" + "urn:diwise:device:" + currentID
		err = a.delete(ctx, urlToDelete)
		if err != nil {
			return err
		}
	}

	b, err := json.Marshal(sensorThing)
	if err != nil {
		return err
	}

	urlToPost := a.thingManagementURL + "/" + thingID
	err = a.post(ctx, urlToPost, b)
	if err != nil {
		return err
	}

	return nil
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

	m := make(map[string]any)
	err = json.Unmarshal(res.Data, &m)
	if err != nil {
		return thing, err
	}

	thing.AddProperties(m)

	return thing, nil
}

func (a *App) NewThing(ctx context.Context, id string, fields map[string]any) error {

	return nil
}

func (a *App) GetValidSensors(ctx context.Context, types []string) ([]SensorIdentifier, error) {
	params := url.Values{
		"urn": types,
	}
	res, err := a.get(ctx, a.deviceManagementURL, "", params)
	if err != nil {
		return []SensorIdentifier{}, err
	}

	var sensors []Sensor
	err = json.Unmarshal(res.Data, &sensors)
	if err != nil {
		return []SensorIdentifier{}, err
	}

	var sensorIDs []SensorIdentifier
	for _, s := range sensors {
		sensorIDs = append(sensorIDs, SensorIdentifier{
			SensorID: s.SensorID,
			DeviceID: s.DeviceID,
			Decoder:  s.DeviceProfile.Decoder,
		})
	}

	return sensorIDs, nil
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

func (a *App) UpdateThing(ctx context.Context, thingID string, fields map[string]any) error {
	b, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	return a.patch(ctx, a.thingManagementURL, thingID, b)

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
		if err != nil || res.Meta == nil {
			ch <- 0
			return
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

func (a *App) GetAlarms(ctx context.Context, offset, limit int, args map[string][]string) (AlarmResult, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))
	params.Add("info", "true")

	for k, v := range args {
		params[k] = v
	}

	res, err := a.get(ctx, a.alarmsURL, "", params)
	if err != nil {
		return AlarmResult{}, err
	}

	var alarms []Alarm
	err = json.Unmarshal(res.Data, &alarms)
	if err != nil {
		return AlarmResult{}, err
	}

	return AlarmResult{
		Alarms:       alarms,
		TotalRecords: int(res.Meta.TotalRecords),
		Offset:       int(*res.Meta.Offset),
		Limit:        int(*res.Meta.Limit),
		Count:        len(alarms),
	}, nil
}
