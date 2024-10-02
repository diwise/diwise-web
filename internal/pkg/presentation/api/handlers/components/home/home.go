package home

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/locale"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/assets"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func NewHomePage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		//w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "home",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, _ := helpers.GetOffsetAndLimit(r)

		limit := 5

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		datasets := []components.ChartDataset{}
		max := 31

		model := components.HomeViewModel{
			UsageDatasets: datasets,
			XScaleMax:     uint(max),
			Alarms:        make([]components.AlarmViewModel, 0),
		}

		result, _ := app.GetAlarms(ctx, offset, limit, args)
		for _, a := range result.Alarms {
			model.Alarms = append(model.Alarms, components.AlarmViewModel{
				ID:          a.ID,
				AlarmType:   a.AlarmType,
				Description: a.Description,
				ObservedAt:  a.ObservedAt,
				RefID:       a.RefID,
				Severity:    a.Severity,
				Tenant:      a.Tenant,
			})
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))
		model.Pageing = getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args)

		home := components.Home(localizer, assets, model)

		component := components.StartPage(version, localizer, assets, home)

		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func NewAlarmsTable(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		//w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "home",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		model := components.HomeViewModel{
			Alarms: make([]components.AlarmViewModel, 0),
		}

		result, _ := app.GetAlarms(ctx, offset, limit, args)
		for _, a := range result.Alarms {
			model.Alarms = append(model.Alarms, components.AlarmViewModel{
				ID:          a.ID,
				AlarmType:   a.AlarmType,
				Description: a.Description,
				ObservedAt:  a.ObservedAt,
				RefID:       a.RefID,
				Severity:    a.Severity,
				Tenant:      a.Tenant,
			})
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))
		model.Pageing = getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args)

		component := components.AlarmsTable(localizer, model)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func NewOverviewCardsHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		//w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx = r.Context()
		stats := app.GetStatistics(ctx)

		component := components.OverviewCards(localizer, assets, components.StatisticsViewModel{
			Total:    stats.Total,
			Active:   stats.Active,
			Inactive: stats.Inactive,
			Online:   stats.Online,
			Unknown:  stats.Unknown,
		})

		component.Render(ctx, w)
	}
	return http.HandlerFunc(fn)
}

func NewUsageHandler(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "max-age=3600")
		//w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")
		w.WriteHeader(http.StatusOK)

		//localizer := l10n.For(r.Header.Get("Accept-Language"))

		ctx = r.Context()

		datasets, max, err := getUsageData(ctx, app)
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		component := components.UsageChart(max, datasets)
		component.Render(ctx, w)
	}

	return http.HandlerFunc(fn)
}

func getUsageData(ctx context.Context, app application.DeviceManagement) ([]components.ChartDataset, uint, error) {
	daysInMonth := func(ts time.Time) int {
		return time.Date(ts.Year(), ts.Month()+1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1).Day()
	}

	now := time.Now().UTC()
	timeAt := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	endTimeAt := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC).Add(-1 * time.Second)

	max := daysInMonth(timeAt)
	if daysInMonth(endTimeAt) > max {
		max = daysInMonth(endTimeAt)
	}

	data, err := app.GetMeasurementData(ctx, "", application.WithAggrMethods("rate"), application.WithTimeUnit("day"), application.WithTimeRel("between", timeAt, endTimeAt))
	if err != nil {
		return nil, 0, err
	}

	sets := make(map[string]components.ChartDataset, 0)
	datasets := make([]components.ChartDataset, 0)

	for _, v := range data.Values {
		m := fmt.Sprintf("%d-%02d", v.Timestamp.Year(), v.Timestamp.Month())
		ds, ok := sets[m]
		if !ok {
			ds = components.NewChartDataset(m)
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

func getPaging(pageIndex, pageLast, pageSize, offset int, pages []int64, args url.Values) components.PagingViewModel {
	return components.PagingViewModel{
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
