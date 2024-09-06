package sensors

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/a-h/templ"
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
		pageLast := float64(result.TotalRecords) / float64(limit)

		model := ui.SensorListViewModel{
			Statistics: getStatistics(ctx, app),
			Sensors:    make([]ui.SensorViewModel, 0),
			Pageing: ui.PagingViewModel{
				PageIndex: pageIndex_,
				PageLast:  int(math.Ceil(pageLast)),
				PageSize:  limit,
				Offset:    offset,
				Pages:     helpers.PagerIndexes(pageIndex_, int(math.Ceil(pageLast))),
				Query:     args.Encode(),
				TargetURL: "/components/tables/sensors",
				TargetID:  "#sensors-table",
			},
		}

		for _, sensor := range result.Sensors {
			tvm := ui.SensorViewModel{
				HasAlerts:    false,
				Active:       sensor.Active,
				DeviceID:     sensor.DeviceID,
				DevEUI:       sensor.SensorID,
				Name:         sensor.Name,
				BatteryLevel: getBatterLevel(ctx, app, sensor.DeviceID),
				LastSeen:     sensor.DeviceState.ObservedAt,
			}

			model.Sensors = append(model.Sensors, tvm)
		}

		sensorList := ui.SensorsList(localizer, model)
		page := components.StartPage(version, localizer, assets, sensorList)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, int(math.Ceil(pageLast)),
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
		pageLast := float64(result.TotalRecords) / float64(limit)

		model := ui.SensorListViewModel{
			Statistics: ui.StatisticsViewModel{},
			Sensors:    make([]ui.SensorViewModel, 0),
			Pageing: ui.PagingViewModel{
				PageIndex: pageIndex_,
				PageLast:  int(math.Ceil(pageLast)),
				PageSize:  limit,
				Offset:    offset,
				Pages:     helpers.PagerIndexes(pageIndex_, int(math.Ceil(pageLast))),
				Query:     args.Encode(),
				TargetURL: "/components/tables/sensors",
				TargetID:  "#sensors-table",
			},
		}

		for _, sensor := range result.Sensors {
			tvm := ui.SensorViewModel{
				HasAlerts:    false,
				Active:       sensor.Active,
				DeviceID:     sensor.DeviceID,
				DevEUI:       sensor.SensorID,
				Name:         sensor.Name,
				BatteryLevel: getBatterLevel(ctx, app, sensor.DeviceID),
				LastSeen:     sensor.DeviceState.ObservedAt,
			}

			model.Sensors = append(model.Sensors, tvm)
		}

		component := ui.SensorsTable(localizer, model)

		ctx = helpers.Decorate(
			ctx,
			components.PageIndex, pageIndex_,
			components.PageLast, int(math.Ceil(pageLast)),
			components.PageSize, limit,
		)
		/*
			err = component.Render(ctx, w)
			if err != nil {
				http.Error(w, fmt.Sprintf("could not render things page - %s", err.Error()), http.StatusInternalServerError)
			}
		*/
		w.WriteHeader(http.StatusOK)
		templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)

	}
	return http.HandlerFunc(fn)
}
