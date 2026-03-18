package sensors

import (
	"context"
	"net/http"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	featuresensors "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/features/sensors"
	shared "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/shared"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	. "github.com/diwise/frontend-toolkit"
)

func NewMeasurementComponentHandler(ctx context.Context, _ LocaleBundle, _ AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := logging.NewContextWithLogger(r.Context(), log)
		id := r.URL.Query().Get("sensorMeasurementTypes")

		layout := "2006-01-02T15:04"
		t := r.URL.Query().Get("timeAt")
		if t == "" {
			t = time.Now().Add(-24 * time.Hour).Format(layout)
		}
		startTime, err := time.Parse(layout, t)
		if err != nil {
			http.Error(w, "could not parse timeAt", http.StatusBadRequest)
			return
		}

		et := r.URL.Query().Get("endTimeAt")
		if et == "" {
			et = time.Now().Format(layout)
		}
		endTime, err := time.Parse(layout, et)
		if err != nil {
			http.Error(w, "could not parse endTimeAt", http.StatusBadRequest)
			return
		}

		measurements := application.MeasurementData{Values: []application.MeasurementValue{}}
		if id != "" {
			measurements, err = app.GetMeasurementData(
				ctx,
				id,
				application.WithLastN(true),
				application.WithTimeRel("between", startTime, endTime),
				application.WithLimit(100),
				application.WithReverse(true),
			)
			if err != nil {
				http.Error(w, "could not fetch measurement data", http.StatusBadRequest)
				return
			}
		}

		component := featuresensors.MeasurementChartComponent(measurementChartConfig(r, measurements))
		helpers.WriteComponentResponse(ctx, w, r, component, 20*1024, 5*time.Minute)
	}
}

func measurementChartConfig(r *http.Request, measurements application.MeasurementData) shared.AdvancedChartConfig {
	beginAtZero := false
	theme := chartTheme(helpers.IsDarkMode(r))

	return shared.AdvancedChartConfig{
		Type: "line",
		Data: shared.AdvancedChartData{
			Labels:   measurementLabels(measurements),
			Datasets: []shared.AdvancedChartDataset{measurementDataset(r, measurements)},
		},
		Options: shared.AdvancedChartOptions{
			Responsive:          true,
			MaintainAspectRatio: false,
			Animation:           false,
			Interaction: &shared.Interaction{
				Intersect: false,
				Axis:      "xy",
				Mode:      "index",
			},
			Plugins: &shared.Plugins{
				Legend: &shared.PluginLegend{
					Display: true,
					Labels: &shared.PluginLegendLabels{
						Color: theme.Foreground,
					},
				},
				Tooltip: &shared.PluginTooltip{
					BackgroundColor: theme.Background,
					BodyColor:       theme.MutedForeground,
					TitleColor:      theme.Foreground,
					BorderColor:     theme.Border,
					BorderWidth:     1,
				},
			},
			Scales: map[string]shared.AxisScale{
				"x": measurementTimeScale(theme),
				"y": {
					Offset:      boolPtr(true),
					BeginAtZero: &beginAtZero,
					Ticks: &shared.AxisTicks{
						Color: theme.MutedForeground,
					},
					Grid: &shared.AxisGrid{
						Display: boolPtr(true),
						Color:   theme.Grid,
					},
					Border: &shared.AxisBorder{
						Display: true,
						Color:   theme.Border,
					},
				},
			},
		},
	}
}

func measurementDataset(r *http.Request, measurements application.MeasurementData) shared.AdvancedChartDataset {
	dataset := shared.AdvancedChartDataset{
		Data:                 make([]any, 0, len(measurements.Values)),
		BorderColor:          chartColor(helpers.IsDarkMode(r)),
		BorderWidth:          2,
		PointBackgroundColor: chartColor(helpers.IsDarkMode(r)),
		PointBorderColor:     chartColor(helpers.IsDarkMode(r)),
		PointRadius:          1,
		PointHoverRadius:     6,
		Fill:                 false,
		Tension:              0.2,
	}

	for _, value := range measurements.Values {
		if dataset.Label == "" {
			dataset.Label = value.Unit
		}

		switch {
		case value.Value != nil:
			dataset.Data = append(dataset.Data, *value.Value)
		case value.BoolValue != nil:
			if *value.BoolValue {
				dataset.Data = append(dataset.Data, 1.0)
			} else {
				dataset.Data = append(dataset.Data, 0.0)
			}
		default:
			dataset.Data = append(dataset.Data, nil)
		}
	}

	return dataset
}

func measurementLabels(measurements application.MeasurementData) []string {
	labels := make([]string, 0, len(measurements.Values))
	for _, value := range measurements.Values {
		labels = append(labels, value.Timestamp.Format("2006-01-02 15:04"))
	}
	return labels
}

func NewStatusChartsComponentHandler(ctx context.Context, l10n LocaleBundle, _ AssetLoaderFunc, app application.DeviceManagement) http.HandlerFunc {
	log := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")

		result, err := app.GetSensorStatus(ctx, id)
		if err != nil {
			log.Error("could not fetch status for sensor", "device_id", id, "err", err)
			http.Error(w, "could not fetch status for sensor", http.StatusInternalServerError)
			return
		}

		localizer := l10n.For(r.Header.Get("Accept-Language"))
		labels := make([]string, 0, len(result))

		batteryLevel := newStatusDataset(localizer.Get("batterylevel"), "yBatteryLevel")
		dr := newStatusDataset(localizer.Get("dr"), "yDR")
		frequency := newStatusDataset(localizer.Get("frequency"), "yFrequency")
		loRaSNR := newStatusDataset(localizer.Get("loraSNR"), "yLoRaSNR")
		rssi := newStatusDataset(localizer.Get("rssi"), "yRSSI")

		for _, status := range result {
			labels = append(labels, status.ObservedAt.Format("2006-01-02 15:04"))

			if status.BatteryLevel > 0 {
				batteryLevel.Data = append(batteryLevel.Data, float64(status.BatteryLevel))
			} else {
				batteryLevel.Data = append(batteryLevel.Data, nil)
			}
			if status.DR != nil {
				dr.Data = append(dr.Data, float64(*status.DR))
			} else {
				dr.Data = append(dr.Data, nil)
			}
			if status.Frequency != nil {
				frequency.Data = append(frequency.Data, float64(*status.Frequency/1000000))
			} else {
				frequency.Data = append(frequency.Data, nil)
			}
			if status.LoRaSNR != nil {
				loRaSNR.Data = append(loRaSNR.Data, float64(*status.LoRaSNR))
			} else {
				loRaSNR.Data = append(loRaSNR.Data, nil)
			}
			if status.RSSI != nil {
				rssi.Data = append(rssi.Data, float64(*status.RSSI))
			} else {
				rssi.Data = append(rssi.Data, nil)
			}
		}

		component := featuresensors.StatusChartComponent(statusChartConfig(r, labels, []shared.AdvancedChartDataset{
			dr, frequency, loRaSNR, rssi, batteryLevel,
		}, map[string]shared.AxisScale{
			"yBatteryLevel": statusScaleConfig(r, localizer.Get("batterylevel"), "right", 1, 99),
			"yDR":           statusScaleConfig(r, localizer.Get("dr"), "left", 0, 7),
			"yFrequency":    statusScaleConfig(r, localizer.Get("frequency"), "left", 863, 870),
			"yLoRaSNR":      statusScaleConfig(r, localizer.Get("loraSNR"), "left", -20, 10),
			"yRSSI":         statusScaleConfig(r, localizer.Get("rssi"), "left", -120, -30),
		}))
		helpers.WriteComponentResponse(ctx, w, r, component, 12*1024, 10*time.Second)
	}
}

func statusChartConfig(r *http.Request, labels []string, datasets []shared.AdvancedChartDataset, yScales map[string]shared.AxisScale) shared.AdvancedChartConfig {
	theme := chartTheme(helpers.IsDarkMode(r))

	scales := map[string]shared.AxisScale{
		"x": measurementTimeScale(theme),
	}

	for key, scale := range yScales {
		scales[key] = scale
	}

	return shared.AdvancedChartConfig{
		Type: "line",
		Data: shared.AdvancedChartData{
			Labels:   labels,
			Datasets: datasets,
		},
		Options: shared.AdvancedChartOptions{
			Responsive:          true,
			MaintainAspectRatio: false,
			Animation:           false,
			Interaction: &shared.Interaction{
				Intersect: false,
				Axis:      "xy",
				Mode:      "index",
			},
			Plugins: &shared.Plugins{
				Legend: &shared.PluginLegend{
					Display: true,
					Labels: &shared.PluginLegendLabels{
						Color: theme.Foreground,
					},
				},
				Tooltip: &shared.PluginTooltip{
					BackgroundColor: theme.Background,
					BodyColor:       theme.MutedForeground,
					TitleColor:      theme.Foreground,
					BorderColor:     theme.Border,
					BorderWidth:     1,
				},
			},
			Scales: scales,
		},
	}
}

func newStatusDataset(label, yAxisID string) shared.AdvancedChartDataset {
	return shared.AdvancedChartDataset{
		Label:                label,
		Data:                 []any{},
		YAxisID:              yAxisID,
		BorderWidth:          2,
		PointRadius:          1,
		PointHoverRadius:     6,
		Fill:                 false,
		Tension:              0.2,
	}
}

func chartColor(isDark bool) string {
	if isDark {
		return "#FFFFFF"
	}
	return "#1F1F25"
}

func statusScaleConfig(r *http.Request, title, position string, min, max float64) shared.AxisScale {
	theme := chartTheme(helpers.IsDarkMode(r))

	return shared.AxisScale{
		Type:     "linear",
		Position: position,
		Min:      floatPtr(min),
		Max:      floatPtr(max),
		Title: &shared.AxisTitle{
			Display: true,
			Text:    title,
			Color:   theme.MutedForeground,
		},
		Ticks: &shared.AxisTicks{
			Color:         theme.MutedForeground,
			MaxTicksLimit: 8,
		},
		Grid: &shared.AxisGrid{
			DrawOnChartArea: false,
			Display:         boolPtr(true),
			Color:           theme.Grid,
		},
		Border: &shared.AxisBorder{
			Display: true,
			Color:   theme.Border,
		},
	}
}

func measurementTimeScale(theme chartThemeConfig) shared.AxisScale {
	return shared.AxisScale{
		Type:         "time",
		Distribution: "linear",
		Ticks: &shared.AxisTicks{
			Color:         theme.MutedForeground,
			MaxTicksLimit: 8,
		},
		Grid: &shared.AxisGrid{
			Display: boolPtr(false),
			Color:   theme.Border,
		},
		Border: &shared.AxisBorder{
			Display: true,
			Color:   theme.Border,
		},
		Time: &shared.ScaleTime{
			Unit:          "hour",
			TooltipFormat: "yyyy-MM-dd HH:mm",
			Parser:        "yyyy-MM-dd HH:mm",
			DisplayFormats: map[string]string{
				"hour": "HH:mm",
				"day":  "yyyy-MM-dd",
			},
		},
	}
}

type chartThemeConfig struct {
	Foreground      string
	Background      string
	MutedForeground string
	Border          string
	Grid            string
}

func chartTheme(isDark bool) chartThemeConfig {
	if isDark {
		return chartThemeConfig{
			Foreground:      "#FFFFFF",
			Background:      "#101012",
			MutedForeground: "#FFFFFF",
			Border:          "#FFFFFF",
			Grid:            "#FFFFFF4D",
		}
	}

	return chartThemeConfig{
		Foreground:      "#1F1F25",
		Background:      "#FFFFFF",
		MutedForeground: "#444450",
		Border:          "#1F1F25",
		Grid:            "#E2E2E8",
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
