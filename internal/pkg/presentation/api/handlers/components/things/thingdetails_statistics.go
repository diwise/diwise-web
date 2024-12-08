package things

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	//lint:ignore ST1001 it is OK when we do it
	. "github.com/diwise/frontend-toolkit"
)

func NewMeasurementComponentHandler(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.ThingManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "no id found in url", http.StatusBadRequest)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := logging.NewContextWithLogger(r.Context(), log)

		thingType := strings.ToLower(r.URL.Query().Get("type"))
		thingSubType := strings.ToLower(r.URL.Query().Get("subType"))

		if thingSubType != "" {
			thingType += ":" + thingSubType
		}

		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
		timeAt := getTime(r, "timeAt", today)
		endOfDay := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, time.UTC)
		endTimeAt := getTime(r, "endTimeAt", endOfDay)

		q := url.Values{}

		q.Add("timerel", "between")
		q.Add("timeat", timeAt.Format(time.RFC3339))
		q.Add("endTimeAt", endTimeAt.Format(time.RFC3339))
		q.Add("options", "groupByRef")

		label := ""
		switch thingType {
		case "pointofinterest:beach":
			fallthrough
		case "pointofinterest":
			q.Add("n", "3303/5700") //Temperature
			label = localizer.Get("3303-5700")
		case "building":
			q.Add("n", "3331/5700") // Energy
			label = localizer.Get("3331-5700")
		case "container:wastecontainer":
			fallthrough
		case "container":
			q.Add("n", "3435/2") //FillingLevel/Percentage
			label = localizer.Get("3435-2")
		case "lifebuoy":
			q.Add("n", "3302/5500") //Presence/State
			q.Add("timeunit", "hour")
			q.Add("vb", "false")
			q.Del("options")
		case "passage":
			q.Add("n", "10351/50") //Door/State
			q.Add("timeunit", "hour")
			q.Add("vb", "true")
			q.Del("options")
			label = localizer.Get("10351-50")
		case "pumpingstation":
			q.Add("n", "3350/5850") //Stopwatch/OnOff
			q.Add("timeunit", "hour")
			q.Add("vb", "true")
			q.Del("options")
		case "room":
			q.Add("n", "3303/5700") //Temperature
			label = localizer.Get("3303-5700")
		case "sewer":
			q.Add("n", "3435/2") //FillingLevel/Percentage
			label = localizer.Get("3435-2")
		case "sewer:combinedseweroverflow":
			q.Add("n", "3350/5850") //Stopwatch/OnOff
			label = localizer.Get("3200-5500")
		case "watermeter":
			q.Add("n", "3424/1") //WaterMeter/CumulativeVolume
			label = localizer.Get("3424-1")
		}

		thing, err := app.GetThing(ctx, id, q)
		if err != nil {
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		datasets := []components.ChartDataset{}

		for _, values := range thing.Values {
			datasets = append(datasets, toDataset(label, values))
		}

		var component templ.Component
		keepRatio := false

		switch thingType {
		//case "beach":
		//	fallthrough
		//case "pointofinterest":
		//	component = components.MeasurementChart(datasets, keepRatio)
		//case "building":
		//	component = components.MeasurementChart(datasets, keepRatio)
		case "container:wastecontainer":
			fallthrough
		case "container":
			component = components.WastecontainerChart(datasets)
		//case "lifebuoy":
		//	component = components.MeasurementChart(datasets, keepRatio)
		case "passage":
			component = components.PassagesChart(datasets)
		//case "pumpingstation":
		//	component = components.MeasurementChart(datasets, keepRatio)
		case "room":
			component = components.RoomChart(datasets)
		//case "sewer":
		//	component = components.MeasurementChart(datasets, keepRatio)
		//case "watermeter":
		//	component = components.MeasurementChart(datasets, keepRatio)
		default:
			component = components.MeasurementChart(datasets, keepRatio)
		}

		helpers.WriteComponentResponse(ctx, w, r, component, 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func toDataset(label string, measurements []application.Measurement) components.ChartDataset {
	dataset := components.NewChartDataset(label)
	previousValue := 0

	for _, v := range measurements {
		if dataset.Label == "" {
			dataset.Label = v.Unit
		}

		if v.Value != nil {
			dataset.Add(v.Timestamp.Format(time.DateTime), *v.Value)
		}

		if v.Value == nil && v.Count != nil && *v.Count > 0 {
			dataset.Add(v.Timestamp.Format(time.DateTime), float64(*v.Count))
		}

		if v.Value == nil && v.BoolValue != nil {
			vb := 0
			if *v.BoolValue {
				vb = 1
			}

			if vb != previousValue {
				// append value when 0->1 and 1->0
				dataset.Add(v.Timestamp.Format(time.DateTime), previousValue)
				previousValue = vb
			}

			dataset.Add(v.Timestamp.Format(time.DateTime), vb)
		}
	}
	return dataset
}

func getTime(r *http.Request, key string, def time.Time) time.Time {
	layout := "2006-01-02T15:04"

	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	t, err := time.Parse(layout, v)
	if err != nil {
		return def
	}
	return t
}
