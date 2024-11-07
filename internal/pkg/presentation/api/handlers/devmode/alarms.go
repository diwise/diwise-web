package devmode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

var testAlarms = []testAlarm{
	{"548dd37b-45a2-4663-88c9-6d30750ec52c", time.Now(), "DeviceNotObserved"},
}

const alarmJsonFormat string = `{"deviceID":"%s","observedAt":"%s","types":["%s"]}`

func NewAlarmsHandler(ctx context.Context) http.HandlerFunc {

	narf := newAlarmResponseFromFilter
	responses := map[string]application.ApiResponse{
		"/alarms?info=true&limit=5&offset=0": narf(allAlarms, 5),
	}

	return func(w http.ResponseWriter, r *http.Request) {

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		if response, ok := responses[r.URL.String()]; ok {
			json.NewEncoder(w).Encode(&response)
		} else {
			logging.GetFromContext(ctx).Error("DEVMODE DATA MISSING FOR PATH", "url", r.URL.String())
			json.NewEncoder(w).Encode(&emptyResponse)
		}
	}
}

func allAlarms(d *testAlarm) bool { return true }

func alarmJson(deviceID string, observed time.Time, alarmType string) string {
	return fmt.Sprintf(alarmJsonFormat, deviceID, observed.Format(time.RFC3339), alarmType)
}

func newAlarmResponseFromFilter(include func(a *testAlarm) bool, limit int) application.ApiResponse {
	var totalCount int
	jsons := make([]string, 0, len(testDevices))

	for _, conf := range testAlarms {
		if include(&conf) {
			totalCount++
			if totalCount <= limit {
				jsons = append(
					jsons,
					alarmJson(
						conf.deviceID,
						conf.observedAt,
						conf.alarmType,
					),
				)
			}
		}
	}
	return newResponseFromJsons(totalCount, jsons)
}

type testAlarm struct {
	deviceID   string
	observedAt time.Time
	alarmType  string
}
