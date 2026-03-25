package home

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/application"
	"github.com/diwise/diwise-web/internal/application/alarms"
	appclient "github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/diwise-web/internal/application/devices"
	"github.com/diwise/diwise-web/internal/application/measurements"
	"github.com/diwise/diwise-web/internal/presentation/api/helpers"
	featurehome "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/home"
	v2layout "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/layout"
	shared "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/shared"

	. "github.com/diwise/frontend-toolkit"
)

type homeApp interface {
	alarms.Management
	devices.Management
	measurements.Management
}

func NewHomePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app homeApp) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx = helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "home",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := getOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		model := featurehome.HomeViewModel{Alarms: make([]featurehome.AlarmViewModel, 0)}

		result, _ := app.GetAlarms(ctx, offset, limit, args)
		for _, a := range result.Alarms {
			model.Alarms = append(model.Alarms, featurehome.AlarmViewModel{
				DeviceID:   a.DeviceID,
				ObservedAt: a.ObservedAt,
				Types:      a.Types,
			})
		}

		pageIndexInt, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))
		model.Paging = getPaging(pageIndexInt, pageLast, limit, args)

		home := featurehome.Home(localizer, model)
		component := templ.Component(v2layout.StartPage(version, localizer, assets, home))
		if helpers.IsHxRequest(r) {
			component = v2layout.AppShell(localizer, assets, home)
		}
		helpers.WriteComponentResponse(ctx, w, r, component, 60*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewAlarmsTable(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app homeApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := helpers.Decorate(
			r.Context(),
			v2layout.CurrentComponent, "home",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := getOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		model := featurehome.HomeViewModel{
			Alarms: make([]featurehome.AlarmViewModel, 0),
		}

		result, _ := app.GetAlarms(ctx, offset, limit, args)
		for _, a := range result.Alarms {
			model.Alarms = append(model.Alarms, featurehome.AlarmViewModel{
				DeviceID:   a.DeviceID,
				ObservedAt: a.ObservedAt,
				Types:      a.Types,
			})
		}

		pageIndexInt, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))
		model.Paging = getPaging(pageIndexInt, pageLast, limit, args)

		component := featurehome.AlarmsTableSection(localizer, model)
		helpers.WriteComponentResponse(ctx, w, r, component, 12*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewOverviewCardsHandler(_ context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app homeApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := r.Context()

		stats, err := app.GetStatistics(ctx)
		if err != nil {
			if errors.Is(err, appclient.ErrUnauthorized) {
				http.Error(w, "not authorized", http.StatusUnauthorized)
			} else {
				http.Error(w, "could not compose view model", http.StatusInternalServerError)
			}
			return
		}

		component := featurehome.OverviewCards(localizer, featurehome.OverviewStats{
			Total:    stats.Total,
			Active:   stats.Active,
			Inactive: stats.Inactive,
			Online:   stats.Online,
			Unknown:  stats.Unknown,
		})

		helpers.WriteComponentResponse(ctx, w, r, component, 12*1024, 30*time.Second)
	}

	return http.HandlerFunc(fn)
}

func NewUsageHandler(_ context.Context, _ LocaleBundle, _ AssetLoaderFunc, app homeApp) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		isDark := helpers.IsDarkMode(r)

		data, err := getUsageData(isDark, ctx, app)
		if err != nil {
			if errors.Is(err, appclient.ErrUnauthorized) {
				http.Error(w, "not authorized", http.StatusUnauthorized)
			} else {
				http.Error(w, "could not compose view model", http.StatusInternalServerError)
			}
			return
		}

		component := featurehome.UsageChart(isDark, data)
		helpers.WriteComponentResponse(ctx, w, r, component, 10*1024, 10*time.Minute)
	}

	return http.HandlerFunc(fn)
}

func getUsageData(isDark bool, ctx context.Context, app homeApp) (shared.AdvancedChartData, error) {
	daysInMonth := func(ts time.Time) int {
		return time.Date(ts.Year(), ts.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1).Day()
	}

	now := time.Now().UTC()
	timeAt := time.Date(now.Year(), now.Month(), now.Day()-3, 0, 0, 0, 0, time.UTC)
	endTimeAt := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC).Add(-1 * time.Second)

	max := daysInMonth(timeAt)
	if daysInMonth(endTimeAt) > max {
		max = daysInMonth(endTimeAt)
	}

	data, err := app.GetMeasurementData(ctx, "", application.WithAggrMethods("rate"), application.WithTimeUnit("day"), application.WithTimeRel("between", timeAt, endTimeAt))
	if err != nil {
		return shared.AdvancedChartData{}, err
	}

	labels := make([]string, 0, max)
	for day := 1; day <= max; day++ {
		labels = append(labels, strconv.Itoa(day))
	}

	sets := make(map[string]shared.AdvancedChartDataset, 0)
	order := make([]string, 0)

	for _, v := range data.Values {
		m := fmt.Sprintf("%d-%02d", v.Timestamp.Year(), v.Timestamp.Month())
		ds, ok := sets[m]
		if !ok {
			color := usageChartColor(isDark, len(order))
			ds = shared.AdvancedChartDataset{
				Label:           m,
				Data:            make([]any, len(labels)),
				BorderWidth:     1,
				BorderColor:     color,
				BackgroundColor: color,
			}
			order = append(order, m)
		}
		day := v.Timestamp.Day()
		if day >= 1 && day <= len(labels) {
			ds.Data[day-1] = float64(v.Count)
		}
		sets[m] = ds
	}

	datasets := make([]shared.AdvancedChartDataset, 0, len(order))
	for _, key := range order {
		datasets = append(datasets, sets[key])
	}

	return shared.AdvancedChartData{Labels: labels, Datasets: datasets}, nil
}

func usageChartColor(isDark bool, index int) string {
	if isDark {
		colors := []string{"#FFFFFF", "#C24E18"}
		return colors[index%len(colors)]
	}

	colors := []string{"#1F1F25", "#C24E18"}
	return colors[index%len(colors)]
}

func getPaging(pageIndex, pageLast, pageSize int, args url.Values) featurehome.PagingViewModel {
	return featurehome.PagingViewModel{
		PageIndex: max(pageIndex, 1),
		PageLast:  max(pageLast, 1),
		PageSize:  pageSize,
		Query:     args.Encode(),
		TargetURL: "/v2/components/tables/alarms",
		TargetID:  "#tableview",
	}
}

func getOffsetAndLimit(r *http.Request) (int, int) {
	offset, limit := helpers.GetOffsetAndLimit(r)
	if r.URL.Query().Get("limit") == "" {
		limit = 5
	}

	return offset, limit
}
