package sensors

import (
	"github.com/a-h/templ"
	shared "github.com/diwise/diwise-web/internal/pkg/presentation/webv2/components/shared"
)

func MeasurementChartComponent(config shared.AdvancedChartConfig) templ.Component {
	return shared.AdvancedChart(shared.AdvancedChartProps{
		ID:     "measurement-chart",
		Config: config,
		Class:  "h-full w-full",
	})
}

func StatusChartComponent(config shared.AdvancedChartConfig) templ.Component {
	return shared.AdvancedChart(shared.AdvancedChartProps{
		ID:     "status-chart",
		Config: config,
		Class:  "h-full w-full",
	})
}
