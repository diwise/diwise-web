package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/authz"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app")

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
	var err error
	ctx, span := tracer.Start(ctx, "get-tags")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var res *ApiResponse
	res, err = a.get(ctx, a.thingManagementURL, "tags", url.Values{})
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
	var err error
	ctx, span := tracer.Start(ctx, "get-deviceprofiles")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var res *ApiResponse
	res, err = a.get(ctx, a.adminURL, "deviceprofiles", url.Values{})
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

func (a *App) GetStatistics(ctx context.Context) (Statistics, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-statistics")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	errs := make(chan error, 5)

	count := func(key, value string, result *int) {
		go func() {
			var err error
			defer func() { errs <- err }()

			params := url.Values{}
			params.Add("limit", "1")

			if key != "" && value != "" {
				params.Add(key, value)
			}

			var res *ApiResponse
			res, err = a.get(ctx, a.deviceManagementURL, "", params)

			if err == nil && res.Meta != nil {
				*result = int(res.Meta.TotalRecords)
			} else {
				*result = 0
			}
		}()
	}

	stats := Statistics{}

	count("", "", &stats.Total)
	count("online", "true", &stats.Online)
	count("active", "true", &stats.Active)
	count("active", "false", &stats.Inactive)
	count("profilename", "unknown", &stats.Unknown)

	for range 5 {
		err = errors.Join(err, <-errs)
	}

	return stats, err
}

func (a *App) GetMeasurementsForSensor(ctx context.Context, id string, params ...InputParam) (map[string][]MeasurementValue, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-measurement-for-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	q := url.Values{}

	for _, p := range params {
		p(&q)
	}

	var resp *ApiResponse
	resp, err = a.get(ctx, a.measurementURL, id, q)
	if err != nil {
		return map[string][]MeasurementValue{}, err
	}

	var measurements map[string][]MeasurementValue

	err = json.Unmarshal(resp.Data, &measurements)
	if err != nil {
		return map[string][]MeasurementValue{}, err
	}

	return measurements, nil
}

func (a *App) GetMeasurementInfo(ctx context.Context, id string) (MeasurementData, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-measurementinfo")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var resp *ApiResponse
	resp, err = a.get(ctx, a.measurementURL, id, url.Values{})
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
	var err error
	ctx, span := tracer.Start(ctx, "get-measurementdata")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	q := url.Values{}
	if id != "" {
		q.Add("id", id)
	}

	for _, p := range params {
		p(&q)
	}

	var resp *ApiResponse
	resp, err = a.get(ctx, a.measurementURL, "", q)
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
	var err error
	ctx, span := tracer.Start(ctx, "get-alarms")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))
	params.Add("info", "true")

	for k, v := range args {
		params[k] = v
	}

	var res *ApiResponse
	res, err = a.get(ctx, a.alarmsURL, "", params)
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

		targetUrl = a.thingManagementURL + "/values"
	default:
		return nil, fmt.Errorf("export parameter is invalid")
	}

	headers := map[string][]string{
		"Authorization": {"Bearer " + authz.Token(ctx)},
		"Accept":        {accept},
	}

	var b []byte
	b, err = helpers.GET(ctx, targetUrl, headers, query)
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

	targetUrl := ""

	switch t {
	case "devices":
		targetUrl = a.deviceManagementURL
	case "things":
		targetUrl = a.thingManagementURL
	}

	err = helpers.FileUpload(ctx, targetUrl, headers, f)
	return err
}
