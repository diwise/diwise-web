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

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featurehome "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/features/home"
	featuresensors "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/features/sensors"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components/layout"
	shared "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/shared"

	. "github.com/diwise/frontend-toolkit"
)

func NewHomePage(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx = helpers.Decorate(
			r.Context(),
			layout.CurrentComponent, "home",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, _ := helpers.GetOffsetAndLimit(r)

		limit := 5

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		datasets := []shared.ChartDataset{}
		max := 31

		model := featurehome.HomeViewModel{
			UsageDatasets: datasets,
			XScaleMax:     uint(max),
			Alarms:        make([]featurehome.AlarmViewModel, 0),
		}

		result, _ := app.GetAlarms(ctx, offset, limit, args)
		for _, a := range result.Alarms {
			model.Alarms = append(model.Alarms, featurehome.AlarmViewModel{
				DeviceID:   a.DeviceID,
				ObservedAt: a.ObservedAt,
				Types:      a.Types,
			})
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))
		model.Pageing = getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args)

		home := featurehome.Home(localizer, assets, model)
		component := layout.StartPage(version, localizer, assets, home)

		helpers.WriteComponentResponse(ctx, w, r, component, 50*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewAlarmsTable(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := helpers.Decorate(
			r.Context(),
			layout.CurrentComponent, "home",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

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

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))
		model.Pageing = getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args)

		component := featurehome.AlarmsTable(localizer, model)

		helpers.WriteComponentResponse(ctx, w, r, component, 5*1024, 0)
	}

	return http.HandlerFunc(fn)
}

func NewOverviewCardsHandler(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := r.Context()
		stats, err := app.GetStatistics(ctx)

		if err != nil {
			if errors.Is(err, application.ErrUnauthorized) {
				http.Error(w, "not authorized", http.StatusUnauthorized)
			} else {
				http.Error(w, "could not compose view model", http.StatusInternalServerError)
			}
			return
		}

		component := featurehome.OverviewCards(localizer, assets, featuresensors.StatisticsViewModel{
			Total:    stats.Total,
			Active:   stats.Active,
			Inactive: stats.Inactive,
			Online:   stats.Online,
			Unknown:  stats.Unknown,
		})

		helpers.WriteComponentResponse(ctx, w, r, component, 10*1024, 30*time.Second)
	}

	return http.HandlerFunc(fn)
}

func NewUsageHandler(_ context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		//localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := r.Context()

		isDark := helpers.IsDarkMode(r)

		datasets, max, err := getUsageData(isDark, ctx, app)
		if err != nil {
			if errors.Is(err, application.ErrUnauthorized) {
				http.Error(w, "not authorized", http.StatusUnauthorized)
			} else {
				http.Error(w, "could not compose view model", http.StatusInternalServerError)
			}
			return
		}

		component := featurehome.UsageChart(isDark, max, datasets)
		helpers.WriteComponentResponse(ctx, w, r, component, 10*1024, 10*time.Minute)
	}

	return http.HandlerFunc(fn)
}

func getUsageData(isDark bool, ctx context.Context, app application.DeviceManagement) ([]shared.ChartDataset, uint, error) {
	daysInMonth := func(ts time.Time) int {
		return time.Date(ts.Year(), ts.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1).Day()
	}

	now := time.Now().UTC()
	//	timeAt := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	//	endTimeAt := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).Add(-1 * time.Second)

	timeAt := time.Date(now.Year(), now.Month(), now.Day()-3, 0, 0, 0, 0, time.UTC)
	endTimeAt := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC).Add(-1 * time.Second)

	max := daysInMonth(timeAt)
	if daysInMonth(endTimeAt) > max {
		max = daysInMonth(endTimeAt)
	}

	data, err := app.GetMeasurementData(ctx, "", application.WithAggrMethods("rate"), application.WithTimeUnit("day"), application.WithTimeRel("between", timeAt, endTimeAt))
	if err != nil {
		return nil, 0, err
	}

	sets := make(map[string]shared.ChartDataset, 0)
	datasets := make([]shared.ChartDataset, 0)

	for _, v := range data.Values {
		m := fmt.Sprintf("%d-%02d", v.Timestamp.Year(), v.Timestamp.Month())
		ds, ok := sets[m]
		if !ok {
			ds = shared.NewChartDataset(m, isDark)
			ds.BorderColor = ""
		}
		ds.Add(strconv.Itoa(v.Timestamp.Day()), v.Count)

		sets[m] = ds
	}

	for _, v := range sets {
		datasets = append(datasets, v)
	}

	return datasets, uint(max), nil
}

func getPaging(pageIndex, pageLast, pageSize, offset int, pages []int64, args url.Values) shared.PagingViewModel {
	return shared.PagingViewModel{
		PageIndex: pageIndex,
		PageLast:  pageLast,
		PageSize:  pageSize,
		Offset:    offset,
		Pages:     pages,
		Query:     args.Encode(),
		TargetURL: "/components/tables/alarms",
		TargetID:  "#tableview",
	}
}
