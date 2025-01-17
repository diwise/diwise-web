package things

import (
	"context"
	"fmt"
	"math"
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

const (
	DoorState = "10351/50"
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
		activeTab := strings.ToLower(r.URL.Query().Get("tab"))

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
		datasets := []components.ChartDataset{}
		var chart, table templ.Component

		n := strings.ReplaceAll(activeTab, "-", "/")

		switch thingType {
		case "pointofinterest:beach":
			fallthrough
		case "pointofinterest":
			q.Add("n", n) //Temperature
			label = localizer.Get(activeTab)
		case "building":
			q.Add("n", n) // Energy
			label = localizer.Get(activeTab)
		case "container:wastecontainer":
			fallthrough
		case "container:sandstorage":
			fallthrough
		case "container":
			q.Add("n", n) //FillingLevel/Percentage
			label = localizer.Get(activeTab)
		case "desk":
			q.Add("n", n) //Presence/State
			q.Add("timeunit", "hour")
			q.Add("vb", "true")
			q.Del("options")
		case "lifebuoy":
			q.Add("n", n) //Presence/State
			q.Del("options")
		case "passage":
			q.Add("n", n) //Door/State = 10351/50
			if n == DoorState {
				q.Add("timeunit", "hour")
				q.Add("vb", "true")
				q.Del("options")
			}
			label = localizer.Get(activeTab)
		case "pumpingstation":
			q.Add("n", n) //Stopwatch/OnOff
			if n == "3350/5544" {
				q.Add("op", "gt")
				q.Add("value", "0")
			}
			if n == "3350/5850" {
				q.Add("timeunit", "hour")
				q.Add("vb", "true")
			}
			q.Del("options")
		case "room":
			q.Add("n", n) //Temperature
			label = localizer.Get(activeTab)
		case "sewer":
			q.Add("n", n) //FillingLevel/Percentage
			label = localizer.Get(activeTab)
		case "sewer:combinedseweroverflow":
			q.Add("n", n) //Stopwatch/OnOff
			label = localizer.Get(activeTab)
		case "watermeter":
			q.Add("n", n) //WaterMeter/CumulativeVolume
			label = localizer.Get(activeTab)
		}

		thing, err := app.GetThing(ctx, id, q)
		if err != nil {
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		isDark := helpers.IsDarkMode(r)

		for _, values := range thing.Values {
			datasets = append(datasets, toDataset(label, isDark, values))
		}

		if len(datasets) == 0 {
			datasets = append(datasets, components.NewChartDataset(label, isDark))
		}

		tsAt := timeAt.UTC().Format(time.RFC3339)
		endTsAt := endTimeAt.UTC().Format(time.RFC3339)

		switch thingType {
		//case "pointofinterest":
		//case "pointofinterest:beach":
		//case "building":
		case "container:sandstorage":
			fallthrough
		case "container:wastecontainer":
			fallthrough
		case "container":
			maxvalue := uint(100)
			stepsize := uint(10)

			if strings.HasSuffix(n, "/3") && thing.TypeValues.MaxDistance != nil && *thing.TypeValues.MaxDistance > 0 {
				maxvalue = uint(math.Ceil(*thing.TypeValues.MaxDistance))
			}

			chart = components.StatisticsChart(datasets, "line", &stepsize, nil, &maxvalue, false, isDark)
		case "desk":
			stepsize := uint(1)
			chart = components.StatisticsChart(datasets, "line", &stepsize, nil, nil, false, isDark)
		case "lifebuoy":
			stepsize := uint(1)
			chart = components.StatisticsChart(datasets, "line", &stepsize, nil, nil, false, isDark)

		case "passage":
			if n == DoorState {
				minvalue := uint(0)
				stepsize := uint(1)
				chart = components.StatisticsChart(datasets, "bar", &stepsize, &minvalue, nil, false, isDark)
			} else {
				stepsize := uint(1)
				chart = components.StatisticsChart(datasets, "line", &stepsize, nil, nil, false, isDark)
			}

		case "pumpingstation":

			if n == "3350/5544" {
				minvalue := uint(0)
				stepsize := uint(1)
				chart = components.StatisticsChart(datasets, "bar", &stepsize, &minvalue, nil, false, isDark)
			}
			if n == "3350/5850" {
				minvalue := uint(0)
				stepsize := uint(1)
				chart = components.StatisticsChart(datasets, "bar", &stepsize, &minvalue, nil, false, isDark)
			}

		case "room":
			stepsize := uint(1)
			chart = components.StatisticsChart(datasets, "line", &stepsize, nil, nil, false, isDark)

		//case "sewer":
		//case "sewer:combinedseweroverflow":
		//case "watermeter":
		default:
			chart = components.StatisticsChart(datasets, "line", nil, nil, nil, false, isDark)
		}

		table = components.StatisticsTable(localizer, datasets[0], tsAt, endTsAt)
		helpers.WriteComponentResponse(ctx, w, r, templ.Join(chart, table), 1024, 0)
	}

	return http.HandlerFunc(fn)
}

func toDataset(label string, isDark bool, measurements []application.Measurement) components.ChartDataset {
	dataset := components.NewChartDataset(label, isDark)
	previousValue := 0

	for _, v := range measurements {
		if dataset.Label == "" {
			dataset.Label = v.Unit
		}

		if v.Value != nil {
			dataset.Add(v.Timestamp.Format(time.DateTime), fmt.Sprintf("%.1f", *v.Value))
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
