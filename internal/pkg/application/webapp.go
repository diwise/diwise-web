package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type App struct {
	deviceManagementURL string
	thingManagementURL  string
	adminURL            string
	measurementURL      string
	alarmsURL           string
}

func New(ctx context.Context, devmgmt, things, admin, alarms, measurement string) (*App, error) {
	return &App{
		deviceManagementURL: devmgmt,
		thingManagementURL:  things,
		adminURL:            admin,
		alarmsURL:           alarms,
		measurementURL:      measurement,
	}, nil
}

type InputParam func(v *url.Values)

func WithReverse(reverse bool) InputParam {
	return func(v *url.Values) {
		v.Set("reverse", fmt.Sprintf("%t", reverse))
	}
}
func WithLimit(limit int) InputParam {
	return func(v *url.Values) {
		v.Set("limit", fmt.Sprintf("%d", limit))
	}
}
func WithLastN(lastN bool) InputParam {
	return func(v *url.Values) {
		v.Set("lastN", fmt.Sprintf("%t", lastN))
	}
}

func WithTimeRel(timeRel string, timeAt, endTimeAt time.Time) InputParam {
	return func(v *url.Values) {
		v.Set("timeRel", timeRel)
		v.Set("timeAt", timeAt.UTC().Format(time.RFC3339))
		v.Set("endTimeAt", endTimeAt.UTC().Format(time.RFC3339))
	}
}

func WithAggrMethods(methods ...string) InputParam {
	return func(v *url.Values) {
		v.Set("aggrMethods", strings.Join(methods, ","))
	}
}

func WithTimeUnit(timeUnit string) InputParam {
	return func(v *url.Values) {
		v.Set("timeUnit", timeUnit)
	}
}

func WithAfter(timeAt time.Time) InputParam {
	return func(v *url.Values) {
		v.Set("timeRel", "after")
		v.Set("timeAt", timeAt.UTC().Format(time.RFC3339))
	}
}

func WithBoolValue(boolValue bool) InputParam {
	return func(v *url.Values) {
		v.Set("vb", fmt.Sprintf("%t", boolValue))
	}
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

func (a *App) GetDeviceProfiles(ctx context.Context) []DeviceProfile {
	res, err := a.get(ctx, a.adminURL, "deviceprofiles", url.Values{})
	if err != nil {
		return []DeviceProfile{}
	}

	var deviceProfiles []DeviceProfile
	err = json.Unmarshal(res.Data, &deviceProfiles)
	if err != nil {
		return []DeviceProfile{}
	}

	return deviceProfiles
}

func (a *App) GetStatistics(ctx context.Context) Statistics {

	count := func(key, value string) <-chan int {
		ch := make(chan int)

		go func() {
			defer close(ch)

			params := url.Values{}
			params.Add("limit", "1")

			if key != "" && value != "" {
				params.Add(key, value)
			}

			res, err := a.get(ctx, a.deviceManagementURL, "", params)
			if err != nil || res.Meta == nil {
				return
			}

			ch <- int(res.Meta.TotalRecords)
		}()

		return ch
	}

	return Statistics{
		Total:    <-count("", ""),
		Online:   <-count("online", "true"),
		Active:   <-count("active", "true"),
		Inactive: <-count("active", "false"),
		Unknown:  <-count("profilename", "unknown"),
	}
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
