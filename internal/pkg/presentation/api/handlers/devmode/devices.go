package devmode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/diwise/diwise-web/internal/pkg/application"
)

var testDevices = []testDevice{
	{active, online_, "default", "a", "b", "numero uno", "unknown", pos{62.389765, 17.306348}},
	{inactv, offline, "default", "a", "b", "numero due", "calcosonic", pos{62.398436, 17.294149}},
}

const deviceJsonFormat string = `{
	"active": %t,
	"sensorID": "%s",
	"deviceID": "%s",
	"tenant": "%s",
	"name": "%s",
	"description": "",
	"location": {
	  "latitude": %f,
	  "longitude": %f
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

	return func(w http.ResponseWriter, r *http.Request) {

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		var lim int64 = 10000
		if r.FormValue("limit") != "" {
			lim, _ = strconv.ParseInt(r.FormValue("limit"), 10, 64)
		}

		response := newDeviceResponseFromFilters(int(lim), newFiltersFromRequest(r)...)
		json.NewEncoder(w).Encode(&response)
	}
}

func newFiltersFromRequest(r *http.Request) []func(*testDevice) bool {
	filters := make([]func(*testDevice) bool, 0, 10)

	if r.FormValue("active") != "" {
		filters = append(filters, isActive(r.FormValue("active") == "true"))
	}

	if r.FormValue("online") != "" {
		filters = append(filters, isOnline(r.FormValue("online") == "true"))
	}

	if r.FormValue("type") != "" {
		filters = append(filters, isType(r.FormValue("type")))
	}

	if len(filters) == 0 {
		filters = append(filters, allDevices)
	}

	return filters
}

func isActive(status bool) func(*testDevice) bool {
	return func(d *testDevice) bool { return status == d.active }
}

func isOnline(status bool) func(*testDevice) bool {
	return func(d *testDevice) bool { return status == d.online }
}

func isType(theType string) func(*testDevice) bool {
	return func(d *testDevice) bool { return strings.EqualFold(d.profilename, theType) }
}

func allDevices(d *testDevice) bool { return true }

func deviceJson(active, online bool, sID, dID, tenant, name, profilename string, loc pos) string {
	return fmt.Sprintf(deviceJsonFormat, active, sID, dID, tenant, name, loc.lat, loc.lon, profilename, online)
}

func newDeviceResponseFromFilters(limit int, filters ...func(d *testDevice) bool) application.ApiResponse {
	var totalCount int
	jsons := make([]string, 0, len(testDevices))

	for _, conf := range testDevices {
		include := true
		for _, match := range filters {
			if !match(&conf) {
				include = false
				break
			}
		}

		if include {
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
						conf.location,
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

type pos struct {
	lat float64
	lon float64
}
type testDevice struct {
	active      bool
	online      bool
	tenant      string
	sensorID    string
	deviceID    string
	name        string
	profilename string

	location pos
}
