package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featuresensors "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/features/sensors"
	shared "github.com/diwise/diwise-web/internal/pkg/presentation/web/components/shared"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	. "github.com/diwise/frontend-toolkit"
)

func NewBatteryLevelComponentHandler(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()
		id := r.PathValue("id")

		batteryLevelID := fmt.Sprintf("%s/3/9", id)

		data, err := app.GetMeasurementData(ctx, batteryLevelID, application.WithLastN(true), application.WithLimit(1))
		if err != nil {
			http.Error(w, "could not compose view model", http.StatusInternalServerError)
			return
		}

		var v string = "-"
		var u string = ""

		if len(data.Values) > 0 {
			if data.Values[0].Value != nil {
				v = fmt.Sprintf("%0.f", *data.Values[0].Value)
				u = data.Values[0].Unit
			}
		}

		component := shared.Text(fmt.Sprintf("%s%s", v, u))
		helpers.WriteComponentResponse(ctx, w, r, component, 1024, 10*time.Minute)
	}

	return http.HandlerFunc(fn)
}

func NewMeasurementComponentHandler(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {

		//localizer := l10n.For(r.Header.Get("Accept-Language"))
		ctx := logging.NewContextWithLogger(r.Context(), log)
		id := r.URL.Query().Get("sensorMeasurementTypes")

		/*
			Kolla om både timeAt och endTimeAt är satta. Då är timeRel = "between"
			Om bara endTimeAt är satt så är timeRel = "before"
			Om bara timeAt är satt så är timeRel = "after"

			Default borde vara "between" de senaste 24 timmarna.
		*/

		layout := "2006-01-02T15:04"
		t := r.URL.Query().Get("timeAt")
		if t == "" {
			t = time.Now().Add(time.Hour * -24).Format(layout)
		}
		startTime, err := time.Parse(layout, t)
		if err != nil {
			log.Error("failed to parse timeAt")
		}

		et := r.URL.Query().Get("endTimeAt")
		if et == "" {
			et = time.Now().Format(layout)
		}
		endTime, err := time.Parse(layout, et)
		if err != nil {
			log.Error("failed to parse endTimeAt")
		}

		measurements := application.MeasurementData{
			Values: []application.MeasurementValue{},
		}

		if id != "" {
			//now := time.Now()
			//today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			measurements, err = app.GetMeasurementData(ctx, id, application.WithLastN(true), application.WithTimeRel("between", startTime, endTime), application.WithLimit(100), application.WithReverse(true))
			if err != nil {
				http.Error(w, "could not fetch measurement data", http.StatusBadRequest)
				return
			}
		}

		isDark := helpers.IsDarkMode(r)

		dataset := shared.NewChartDataset("", isDark)

		previousValue := 0
		for _, v := range measurements.Values {
			if dataset.Label == "" {
				dataset.Label = v.Unit
			}

			if v.Value != nil {
				dataset.Add(v.Timestamp.Format(time.DateTime), *v.Value)
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

		component := featuresensors.MeasurementChart([]shared.ChartDataset{dataset}, true, isDark)
		helpers.WriteComponentResponse(ctx, w, r, component, 20*1024, 5*time.Minute)
	}

	return http.HandlerFunc(fn)
}

type chartPoint struct {
	X string  `json:"x"`
	Y float64 `json:"y"`
}

type chartDataset struct {
	Label   string       `json:"label"`
	Data    []chartPoint `json:"data"`
	YAxisID string       `json:"yAxisID"`
}

func newChartScale(label, position string, min, max float64) chartScale {
	return chartScale{
		Type: "linear",
		Grid: scaleGrid{DrawOnChartArea: false},
		Label: scaleLabel{
			Display: true,
			Label:   label,
		},
		ScaleTime: scaleTime{
			TooltipFormat: "DD T",
		},
		Position: position,
		Max:      max,
		Min:      min,
	}
}

type chartScale struct {
	Type      string     `json:"type"`
	Grid      scaleGrid  `json:"grid"`
	Label     scaleLabel `json:"title"`
	ScaleTime scaleTime  `json:"time"`
	Position  string     `json:"position,omitempty"`
	Max       float64    `json:"suggestedMax,omitzero"`
	Min       float64    `json:"suggestedMin,omitzero"`
}

type scaleLabel struct {
	Display bool   `json:"display"`
	Label   string `json:"text"`
}
type scaleTime struct {
	TooltipFormat string `json:"tooltipFormat"`
}
type scaleGrid struct {
	DrawOnChartArea bool `json:"drawOnChartArea"`
	DrawTicks       bool `json:"drawTicks"`
}

type chartResponse struct {
	Datasets  []chartDataset        `json:"datasets"`
	Scales    map[string]chartScale `json:"scales"`
	TimeAt    string                `json:"timeAt"`
	EndTimeAt string                `json:"endTimeAt"`
}

func NewStatusChartsComponentHandler(ctx context.Context, l10n LocaleBundle, assets AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")

		result, err := app.GetSensorStatus(ctx, id)
		if err != nil {
			log.Error("could not fetch status for sensor", "device_id", id, "err", err)
			http.Error(w, "could not fetch status for sensor", http.StatusInternalServerError)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))

		datasets := []chartDataset{}
		timeAt, endTimeAt := time.Now(), time.Time{}

		batteryLevel := chartDataset{
			Label:   localizer.Get("batterylevel"),
			YAxisID: "yBatteryLevel",
			Data:    make([]chartPoint, 0),
		}
		dr := chartDataset{
			Label:   localizer.Get("dr"),
			YAxisID: "yDR",
			Data:    make([]chartPoint, 0),
		}
		frequency := chartDataset{
			Label:   localizer.Get("frequency"),
			YAxisID: "yFrequency",
			Data:    make([]chartPoint, 0),
		}
		loRaSNR := chartDataset{
			Label:   localizer.Get("loraSNR"),
			YAxisID: "yLoRaSNR",
			Data:    make([]chartPoint, 0),
		}
		rssi := chartDataset{
			Label:   localizer.Get("rssi"),
			YAxisID: "yRSSI",
			Data:    make([]chartPoint, 0),
		}
		spreadingFactor := chartDataset{
			Label:   localizer.Get("spreadingFactor"),
			YAxisID: "ySpreadingFactor",
			Data:    make([]chartPoint, 0),
		}

		for _, s := range result {
			if s.BatteryLevel > 0 {
				batteryLevel.Data = append(batteryLevel.Data, chartPoint{
					X: s.ObservedAt.Format(time.RFC3339),
					Y: float64(s.BatteryLevel),
				})
			}
			if s.DR != nil {
				dr.Data = append(dr.Data, chartPoint{
					X: s.ObservedAt.Format(time.RFC3339),
					Y: float64(*s.DR),
				})
			}
			if s.Frequency != nil {
				frequency.Data = append(frequency.Data, chartPoint{
					X: s.ObservedAt.Format(time.RFC3339),
					Y: float64(*s.Frequency / 1000000),
				})
			}
			if s.LoRaSNR != nil {
				loRaSNR.Data = append(loRaSNR.Data, chartPoint{
					X: s.ObservedAt.Format(time.RFC3339),
					Y: float64(*s.LoRaSNR),
				})
			}
			if s.RSSI != nil {
				rssi.Data = append(rssi.Data, chartPoint{
					X: s.ObservedAt.Format(time.RFC3339),
					Y: float64(*s.RSSI),
				})
			}
			if s.SpreadingFactor != nil && *s.SpreadingFactor > 0 {
				spreadingFactor.Data = append(spreadingFactor.Data, chartPoint{
					X: s.ObservedAt.Format(time.RFC3339),
					Y: float64(*s.SpreadingFactor),
				})
			}
		}

		if len(dr.Data) > 0 {
			datasets = append(datasets, dr)
		}
		if len(frequency.Data) > 0 {
			datasets = append(datasets, frequency)
		}
		if len(loRaSNR.Data) > 0 {
			datasets = append(datasets, loRaSNR)
		}
		if len(rssi.Data) > 0 {
			datasets = append(datasets, rssi)
		}
		if len(spreadingFactor.Data) > 0 {
			datasets = append(datasets, spreadingFactor)
		}
		if len(batteryLevel.Data) > 0 {
			datasets = append(datasets, batteryLevel)
		}

		scales := make(map[string]chartScale)
		for i := range datasets {
			switch datasets[i].YAxisID {
			case "yBatteryLevel":
				scales[datasets[i].YAxisID] = newChartScale(datasets[i].Label, "right", 1, 99)
			case "yDR":
				scales[datasets[i].YAxisID] = newChartScale(datasets[i].Label, "left", 0, 7)
			case "yFrequency":
				scales[datasets[i].YAxisID] = newChartScale(datasets[i].Label, "left", 863, 870)
			case "yLoRaSNR":
				scales[datasets[i].YAxisID] = newChartScale(datasets[i].Label, "left", -20, 10)
			case "yRSSI":
				scales[datasets[i].YAxisID] = newChartScale(datasets[i].Label, "left", -120, -30)
			case "ySpreadingFactor":
				scales[datasets[i].YAxisID] = newChartScale(datasets[i].Label, "left", 0, 12)
			default:
				scales[datasets[i].YAxisID] = newChartScale(datasets[i].Label, "left", 0, 0)
			}
		}

		b, _ := json.Marshal(chartResponse{
			Datasets:  datasets,
			Scales:    scales,
			TimeAt:    timeAt.Format(time.DateTime),
			EndTimeAt: endTimeAt.Format(time.DateTime),
		})

		helpers.WriteResponse(ctx, w, r, b, 1024, 10*time.Second)
	}

	return http.HandlerFunc(fn)
}
