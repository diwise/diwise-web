package sensors

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
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components/ui"
)

func NewSensorsPage(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	version := helpers.GetVersion(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetSensors(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := ui.SensorListViewModel{
			Sensors: make([]ui.SensorViewModel, 0),
			Pageing: getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
		}

		for _, sensor := range result.Sensors {
			tvm := sensorToViewModel(sensor)
			tvm.BatteryLevel = getBatterLevel(ctx, app, sensor.DeviceID)
			model.Sensors = append(model.Sensors, tvm)
		}

		model.Statistics = getStatistics(ctx, app)

		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			model.MapView = true
		}

		sensorList := ui.SensorsList(localizer, model)
		page := components.StartPage(version, localizer, assets, sensorList)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		w.WriteHeader(http.StatusOK)

		err = page.Render(ctx, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
		}

	}
	return http.HandlerFunc(fn)
}

func NewSensorsTable(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetSensors(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := ui.SensorListViewModel{
			Statistics: ui.StatisticsViewModel{},
			Sensors:    make([]ui.SensorViewModel, 0),
			Pageing:    getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
		}

		for _, sensor := range result.Sensors {
			tvm := sensorToViewModel(sensor)
			tvm.BatteryLevel = getBatterLevel(ctx, app, sensor.DeviceID)
			model.Sensors = append(model.Sensors, tvm)
		}

		component := ui.SensorsTable(localizer, model)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		err = component.Render(ctx, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)

	}
	return http.HandlerFunc(fn)
}

func NewSensorsList(ctx context.Context, l10n locale.Bundle, assets assets.AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Header().Add("Cache-Control", "no-cache")
		w.Header().Add("Strict-Transport-Security", "max-age=86400; includeSubDomains")

		ctx = helpers.Decorate(
			r.Context(),
			components.CurrentComponent, "sensors",
		)

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		pageIndex := helpers.UrlParamOrDefault(r, "page", "1")
		offset, limit := helpers.GetOffsetAndLimit(r)

		args := r.URL.Query()
		helpers.SanitizeParams(args, "page", "limit", "offset")

		result, err := app.GetSensors(ctx, offset, limit, args)
		if err != nil {
			http.Error(w, "could not fetch sensors", http.StatusInternalServerError)
			return
		}

		pageIndex_, _ := strconv.Atoi(pageIndex)
		pageLast := int(math.Ceil(float64(result.TotalRecords) / float64(limit)))

		model := ui.SensorListViewModel{
			Statistics: ui.StatisticsViewModel{},
			Sensors:    make([]ui.SensorViewModel, 0),
			Pageing:    getPaging(pageIndex_, pageLast, limit, offset, helpers.PagerIndexes(pageIndex_, pageLast), args),
		}

		for _, sensor := range result.Sensors {
			tvm := sensorToViewModel(sensor)
			tvm.BatteryLevel = getBatterLevel(ctx, app, sensor.DeviceID)
			model.Sensors = append(model.Sensors, tvm)
		}

		if mv, ok := args["mapview"]; ok && mv[0] == "true" {
			model.MapView = true
		}

		component := ui.DataList(localizer, model)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, pageLast,
			components.PageSize, limit,
		)

		err = component.Render(ctx, w)
		if err != nil {
			http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
		}

		w.WriteHeader(http.StatusOK)

	}
	return http.HandlerFunc(fn)
}

func getStatistics(ctx context.Context, app application.DeviceManagement) ui.StatisticsViewModel {
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	sumOfStuff := app.GetStatistics(ctx)

	stats := ui.StatisticsViewModel{
		Total:    sumOfStuff.Total,
		Active:   sumOfStuff.Active,
		Inactive: sumOfStuff.Inactive,
		Online:   sumOfStuff.Online,
		Unknown:  sumOfStuff.Unknown,
	}

	return stats
}

func getBatterLevel(ctx context.Context, app application.DeviceManagement, deviceID string) int {
	batteryLevelID := fmt.Sprintf("%s/3/9", deviceID)
	data, err := app.GetMeasurementData(ctx, batteryLevelID, application.WithLastN(true), application.WithLimit(1))
	if err != nil {
		return -1
	}

	if len(data.Values) > 0 {
		if len(data.Values) > 0 && data.Values[0].Value != nil {
			v := int(math.Min(math.Max(*data.Values[0].Value, 0), 100))
			return v
		}
	}

	return -1
}

func sensorToViewModel(sensor application.Sensor) ui.SensorViewModel {
	s := ui.SensorViewModel{
		HasAlerts:    false,
		Active:       sensor.Active,
		DeviceID:     sensor.DeviceID,
		DevEUI:       sensor.SensorID,
		Name:         sensor.Name,
		BatteryLevel: 0,
		LastSeen:     sensor.DeviceState.ObservedAt,
		Latitude:     sensor.Location.Latitude,
		Longitude:    sensor.Location.Longitude,
	}

	return s
}

func getPaging(pageIndex, pageLast, pageSize, offset int, pages []int64, args url.Values) ui.PagingViewModel {
	return ui.PagingViewModel{
		PageIndex: pageIndex,
		PageLast:  pageLast,
		PageSize:  pageSize,
		Offset:    offset,
		Pages:     pages,
		Query:     args.Encode(),
		TargetURL: "/components/tables/sensors",
		TargetID:  "#sensors-table",
	}
}
