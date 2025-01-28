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

		activeTab := strings.ToLower(r.URL.Query().Get("tab"))

		today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC)
		timeAt := getTime(r, "timeAt", today)
		endOfDay := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, time.UTC)
		endTimeAt := getTime(r, "endTimeAt", endOfDay)

		var chart, table templ.Component
		datasets := []components.ChartDataset{}

		label := localizer.Get(activeTab)
		isDark := helpers.IsDarkMode(r)

		var maxvalue *uint
		var stepsize *uint
		var minvalue *uint

		chartType := "line"
		keepRatio := false
		zero := uint(0)
		one := uint(1)
		ten := uint(10)
		hundred := uint(100)

		n := strings.ReplaceAll(activeTab, "-", "/")

		q := url.Values{}
		q.Add("timerel", "between")
		q.Add("timeat", timeAt.Format(time.RFC3339))
		q.Add("endTimeAt", endTimeAt.Format(time.RFC3339))
		q.Add("options", "groupByRef")
		q.Add("n", n)

		// FillingLevel (2 = percentage, 3 = meter)
		if strings.HasPrefix(n, "3435/") {
			minvalue = &zero
			maxvalue = &hundred
			stepsize = &ten
		}

		// Door 10351 (50 = state)
		if strings.HasPrefix(n, "10351/") {
			q.Add("timeunit", "hour")
			q.Add("vb", "true")
			q.Del("options")

			minvalue = &zero
			stepsize = &one
			chartType = "bar"
		}

		// Precense (presence = 5500)
		if strings.HasPrefix(n, "3302/") {
			q.Add("timeunit", "hour")
			q.Add("vb", "true")
			q.Del("options")

			stepsize = &one
		}

		// Stopwatch (5850 = OnOff, 5544 = cumulative time)
		if strings.HasPrefix(n, "3350/") {
			if n == "3350/5544" {
				q.Add("op", "gt")
				q.Add("value", "0")

				stepsize = &one
			}
			if n == "3350/5850" {
				q.Add("timeunit", "hour")
				q.Add("vb", "true")

				minvalue = &zero
				stepsize = &one
				chartType = "bar"
			}
			q.Del("options")
		}

		thing, err := app.GetThing(ctx, id, q)
		if err != nil {
			http.Error(w, "could not fetch thing", http.StatusInternalServerError)
			return
		}

		for _, values := range thing.Values {
			datasets = append(datasets, toDataset(label, isDark, values))
		}

		if len(datasets) == 0 {
			datasets = append(datasets, components.NewChartDataset(label, isDark))
		}

		if strings.HasSuffix(n, "/3") && thing.TypeValues.MaxDistance != nil && *thing.TypeValues.MaxDistance > 0 {
			m := uint(math.Ceil(*thing.TypeValues.MaxDistance))
			maxvalue = &m
			stepsize = &one
		}

		chart = components.StatisticsChart(datasets, chartType, stepsize, minvalue, maxvalue, keepRatio, isDark)

		tsAt := timeAt.UTC().Format(time.RFC3339)
		endTsAt := endTimeAt.UTC().Format(time.RFC3339)

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
