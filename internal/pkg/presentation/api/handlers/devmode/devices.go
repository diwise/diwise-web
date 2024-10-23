package devmode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

var testDevices = []testDevice{
	{active, online_, "default", "a", "b", "numero uno", "unknown"},
	{inactv, offline, "default", "a", "b", "numero due", "calcosonic"},
}

const deviceJsonFormat string = `{
	"active": %t,
	"sensorID": "%s",
	"deviceID": "%s",
	"tenant": "%s",
	"name": "%s",
	"description": "",
	"location": {
	  "latitude": 0,
	  "longitude": 0
	},
	"types": [
	  {
		"urn": "urn:oma:lwm2m:ext:3424",
		"name": "WaterMeter"
	  }
	],
	"deviceProfile": {
	  "name": "%s",
	  "decoder": "qalcosonic",
	  "interval": 172800,
	  "types": [
		"urn:oma:lwm2m:ext:3",
		"urn:oma:lwm2m:ext:3424",
		"urn:oma:lwm2m:ext:3303"
	  ]
	},
	"deviceStatus": {
	  "batteryLevel": -1,
	  "observedAt": "2024-10-22T12:28:05.866842374Z"
	},
	"deviceState": {
	  "online": %t,
	  "state": 1,
	  "observedAt": "2024-10-22T12:28:05.866842374Z"
	}
  }`

var emptyResponse = newResponseFromJsons(0, []string{})

func NewDevicesHandler(ctx context.Context) http.HandlerFunc {

	ndrf := newDeviceResponseFromFilter
	responses := map[string]application.ApiResponse{
		"/devices":                             ndrf(allDevices, noLimit),
		"/devices?limit=1":                     ndrf(allDevices, limitOne),
		"/devices?active=true&limit=1":         ndrf(isActive(true), limitOne),
		"/devices?active=false&limit=1":        ndrf(isActive(false), limitOne),
		"/devices?limit=1&online=true":         ndrf(isOnline(true), limitOne),
		"/devices?limit=1&profilename=unknown": ndrf(unknown, limitOne),
		"/devices?limit=15&offset=0":           ndrf(allDevices, 15),
	}

	return func(w http.ResponseWriter, r *http.Request) {

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		// why is mapview sent to the api?
		theURL := strings.ReplaceAll(r.URL.String(), "&mapview=false", "")

		if response, ok := responses[theURL]; ok {
			json.NewEncoder(w).Encode(&response)
		} else {
			logging.GetFromContext(ctx).Error("DEVMODE DATA MISSING FOR PATH", "url", r.URL.String())
			json.NewEncoder(w).Encode(&emptyResponse)
		}
	}
}

func isActive(status bool) func(*testDevice) bool {
	return func(d *testDevice) bool { return status == d.active }
}

func isOnline(status bool) func(*testDevice) bool {
	return func(d *testDevice) bool { return status == d.online }
}

func allDevices(d *testDevice) bool { return true }
func unknown(d *testDevice) bool    { return strings.EqualFold(d.profilename, "unknown") }

func deviceJson(active, online bool, sID, dID, tenant, name, profilename string) string {
	return fmt.Sprintf(deviceJsonFormat, active, sID, dID, tenant, name, profilename, online)
}

func newDeviceResponseFromFilter(include func(d *testDevice) bool, limit int) application.ApiResponse {
	var totalCount int
	jsons := make([]string, 0, len(testDevices))

	for _, conf := range testDevices {
		if include(&conf) {
			totalCount++
			if totalCount <= limit {
				jsons = append(
					jsons,
					deviceJson(
						conf.active,
						conf.online,
						conf.sensorID,
						conf.deviceID,
						conf.tenant,
						conf.name,
						conf.profilename,
					),
				)
			}
		}
	}
	return newResponseFromJsons(totalCount, jsons)
}

func newResponseFromJsons(totalRecords int, jsons []string) application.ApiResponse {
	zero := uint64(0)
	bignum := uint64(9223372036854775807)
	count := uint64(len(jsons))

	response := application.ApiResponse{
		Meta: &application.Meta{
			Count:        count,
			TotalRecords: uint64(totalRecords),
			Offset:       &zero,
			Limit:        &bignum,
		},
		Data:  []byte("[" + strings.Join(jsons, ",") + "]"),
		Links: &application.Links{},
	}

	return response
}

const active bool = true
const inactv bool = false

const online_ bool = true
const offline bool = false

const limitOne int = 1
const noLimit int = 10000

type testDevice struct {
	active      bool
	online      bool
	tenant      string
	sensorID    string
	deviceID    string
	name        string
	profilename string
}
