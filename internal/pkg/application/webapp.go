package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
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

func (a *App) Export(ctx context.Context, params url.Values) ([]byte, error) {
	query, _ := url.ParseQuery(params.Encode())

	export := query.Get("export")
	if export == "" {
		return nil, fmt.Errorf("export parameter is missing")
	}

	accept := query.Get("accept")
	if accept == "" {
		return nil, fmt.Errorf("accept parameter is missing")
	}

	targetUrl := ""

	helpers.SanitizeParams(query, "limit", "offset", "mapview", "export", "accept", "redirected")

	switch export {
	case "devices":
		targetUrl = a.deviceManagementURL
	case "things":
		if query.Has("type") {
			t := query.Get("type")
			if strings.Contains(t, "-") {
				query.Set("type", strings.Split(t, "-")[0])
				query.Set("subType", strings.Split(t, "-")[1])
			}
		}
		targetUrl = a.thingManagementURL
	default:
		return nil, fmt.Errorf("export parameter is invalid")
	}

	headers := map[string][]string{
		"Authorization": {"Bearer " + authz.Token(ctx)},
		"Accept":        {accept},
	}

	b, err := helpers.GET(ctx, targetUrl, headers, query)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (a *App) Import(ctx context.Context, t string, f io.Reader) error {
	headers := map[string][]string{
		"Authorization": {"Bearer " + authz.Token(ctx)},
	}

	targetUrl := ""

	switch t {
	case "devices":
		targetUrl = a.deviceManagementURL
	case "things":
		targetUrl = a.thingManagementURL
	}

	err := helpers.FileUpload(ctx, targetUrl, headers, f)
	if err != nil {
		return err
	}

	return nil
}
