package devmode

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func NewThingsHandler(ctx context.Context) http.HandlerFunc {
	logger := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info("DEVMODE THINGS REQUEST", "url", r.URL.String())

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		response := application.ApiResponse{}
		err := json.Unmarshal([]byte(thingsJsonFormat), &response)
		if err != nil {
			logger.Error("DEVMODE THINGS ERROR", "error", err)
			http.Error(w, "could not render things", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(&response)
	}
}

func NewThingHandler(ctx context.Context) http.HandlerFunc {
	logger := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info("DEVMODE THINGS REQUEST", "url", r.URL.String())

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		id := r.PathValue("id")
		options := r.URL.Query().Get("options")

		if options == "groupByRef" {
			id += "/groupByRef"
		}

		response := application.ApiResponse{}
		err := json.Unmarshal([]byte(thingsStorage[id]), &response)
		if err != nil {
			logger.Error("DEVMODE THINGS ERROR", "error", err)
			http.Error(w, "could not render things", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(&response)
	}
}

func NewThingsTagsHandler(ctx context.Context) http.HandlerFunc {
	logger := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info("DEVMODE THINGS REQUEST", "url", r.URL.String())

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		response := application.ApiResponse{}
		err := json.Unmarshal([]byte(thingsTagsJsonFormat), &response)
		if err != nil {
			logger.Error("DEVMODE THINGS ERROR", "error", err)
			http.Error(w, "could not render things", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(&response)
	}
}

func NewThingsTypesHandler(ctx context.Context) http.HandlerFunc {
	logger := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {

		logger.Info("DEVMODE THINGS REQUEST", "url", r.URL.String())

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)

		response := application.ApiResponse{}
		err := json.Unmarshal([]byte(thingsTypesJsonFormat), &response)
		if err != nil {
			logger.Error("DEVMODE THINGS ERROR", "error", err)
			http.Error(w, "could not render things", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(&response)
	}
}

var thingsStorage = map[string]string{
	"17662c5d-27d2-4b43-8547-66df60ee6ba3":            wasteContainerJsonFormat,
	"17662c5d-27d2-4b43-8547-66df60ee6ba3/groupByRef": wasteContainerStatsJsonFormat,
	"f47ac10b-58cc-4372-a567-0e02b2c3d479":            sandStorageJsonFormat,
	"f47ac10b-58cc-4372-a567-0e02b2c3d479/groupByRef": sandStorageStatsJsonFormat,
}

var wasteContainerJsonFormat = `
{
    "meta": {
        "totalRecords": 8
    },
    "data": {
        "currentLevel": 0.37,
        "description": "Beskrivning",
        "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3",
        "location": {
            "latitude": 62.37894,
            "longitude": 17.33997
        },
        "maxd": 0.94,
        "maxl": 0.76,
        "name": "Soptunna-054",
        "observedAt": "2024-10-24T12:15:00Z",
        "percent": 48.68421052631579,
        "refDevices": [
            {
                "deviceID": "milesight:54"
            }
        ],
        "subType": "WasteContainer",
        "tags": [
            "Soptunna"
        ],
        "tenant": "default",
        "type": "Container",
        "validURN": [
            "urn:oma:lwm2m:ext:3330"
        ],
        "values": [
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 56.578947368421055,
                "unit": "%",
                "timestamp": "2024-10-24T09:44:56Z",
                "ref": "milesight:54/3330/5700"
            },
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/3",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 0.43,
                "unit": "m",
                "timestamp": "2024-10-24T09:44:56Z",
                "ref": "milesight:54/3330/5700"
            },
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 56.578947368421055,
                "unit": "%",
                "timestamp": "2024-10-24T10:14:57Z",
                "ref": "milesight:54/3330/5700"
            },
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/3",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 0.43,
                "unit": "m",
                "timestamp": "2024-10-24T10:14:57Z",
                "ref": "milesight:54/3330/5700"
            },
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 42.10526315789474,
                "unit": "%",
                "timestamp": "2024-10-24T10:44:55Z",
                "ref": "milesight:54/3330/5700"
            },
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/3",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 0.32,
                "unit": "m",
                "timestamp": "2024-10-24T10:44:55Z",
                "ref": "milesight:54/3330/5700"
            },
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 40.78947368421053,
                "unit": "%",
                "timestamp": "2024-10-24T12:15:00Z",
                "ref": "milesight:54/3330/5700"
            },
            {
                "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/3",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 0.31,
                "unit": "m",
                "timestamp": "2024-10-24T12:15:00Z",
                "ref": "milesight:54/3330/5700"
            }
        ]
    }
}
`

var wasteContainerStatsJsonFormat = `
{
    "meta": {
        "totalRecords": 4
    },
    "data": {
        "currentLevel": 0.37,
        "description": "Beskrivning",
        "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3",
        "location": {
            "latitude": 62.37894,
            "longitude": 17.33997
        },
        "maxd": 0.94,
        "maxl": 0.76,
        "name": "Soptunna-054",
        "observedAt": "2024-10-24T12:15:00Z",
        "percent": 48.68421052631579,
        "refDevices": [
            {
                "deviceID": "milesight:54"
            }
        ],
        "subType": "WasteContainer",
        "tags": [
            "Soptunna"
        ],
        "tenant": "default",
        "type": "Container",
        "validURN": [
            "urn:oma:lwm2m:ext:3330"
        ],
        "values": {
            "milesight:54/3330/5700": [
                {
                    "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 56.578947368421055,
                    "unit": "%",
                    "timestamp": "2024-10-24T09:44:56Z",
                    "ref": "milesight:54/3330/5700"
                },
                {
                    "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 56.578947368421055,
                    "unit": "%",
                    "timestamp": "2024-10-24T10:14:57Z",
                    "ref": "milesight:54/3330/5700"
                },
                {
                    "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 42.10526315789474,
                    "unit": "%",
                    "timestamp": "2024-10-24T10:44:55Z",
                    "ref": "milesight:54/3330/5700"
                },
                {
                    "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 40.78947368421053,
                    "unit": "%",
                    "timestamp": "2024-10-24T12:15:00Z",
                    "ref": "milesight:54/3330/5700"
                }
            ]
        }
    }
}
`

var sandStorageJsonFormat = `
{
    "meta": {
        "totalRecords": 44
    },
    "data": {
        "currentLevel": 0.27,
        "description": "",
        "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
        "location": {
            "latitude": 62.425242,
            "longitude": 17.417382
        },
        "maxd": 0.94,
        "maxl": 0.76,
        "name": "Sandficka - Alnö",
        "observedAt": "2024-10-24T13:40:07Z",
        "percent": 35.526315789473685,
        "refDevices": [
            {
                "deviceID": "milesight:193"
            },
            {
                "deviceID": "milesight:194"
            },
            {
                "deviceID": "milesight:195"
            }
        ],
        "subType": "Sandstorage",
        "tags": [
            "Sandficka",
            "Alnö"
        ],
        "tenant": "default",
        "type": "Container",
        "validURN": [
            "urn:oma:lwm2m:ext:3330"
        ],
        "values": [
            {
                "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/3",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 0.12,
                "unit": "m",
                "timestamp": "2024-10-24T08:48:02Z",
                "ref": "milesight:194/3330/5700"
            },
            {
                "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                "urn": "urn:oma:lwm2m:ext:3435",
                "v": 15.789473684210526,
                "unit": "%",
                "timestamp": "2024-10-24T08:48:02Z",
                "ref": "milesight:194/3330/5700"
            }
        ]
    }
}
`

var sandStorageStatsJsonFormat = `
{
    "meta": {
        "totalRecords": 23
    },
    "data": {
        "currentLevel": 0.33,
        "description": "",
        "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
        "location": {
            "latitude": 62.425242,
            "longitude": 17.417382
        },
        "maxd": 0.94,
        "maxl": 0.76,
        "name": "Sandficka - Alnö",
        "observedAt": "2024-10-24T13:42:42Z",
        "percent": 43.421052631578945,
        "refDevices": [
            {
                "deviceID": "milesight:193"
            },
            {
                "deviceID": "milesight:194"
            },
            {
                "deviceID": "milesight:195"
            }
        ],
        "subType": "Sandstorage",
        "tags": [
            "Sandficka",
            "Alnö"
        ],
        "tenant": "default",
        "type": "Container",
        "validURN": [
            "urn:oma:lwm2m:ext:3330"
        ],
        "values": {
            "milesight:193/3330/5700": [
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 38.1578947368421,
                    "unit": "%",
                    "timestamp": "2024-10-24T12:12:44Z",
                    "ref": "milesight:193/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 51.31578947368421,
                    "unit": "%",
                    "timestamp": "2024-10-24T12:42:42Z",
                    "ref": "milesight:193/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 60.526315789473685,
                    "unit": "%",
                    "timestamp": "2024-10-24T13:42:42Z",
                    "ref": "milesight:193/3330/5700"
                }
            ],
            "milesight:194/3330/5700": [
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 15.789473684210526,
                    "unit": "%",
                    "timestamp": "2024-10-24T08:48:02Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 14.473684210526315,
                    "unit": "%",
                    "timestamp": "2024-10-24T09:18:01Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 14.473684210526315,
                    "unit": "%",
                    "timestamp": "2024-10-24T09:48:03Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 15.789473684210526,
                    "unit": "%",
                    "timestamp": "2024-10-24T10:18:07Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 14.473684210526315,
                    "unit": "%",
                    "timestamp": "2024-10-24T10:48:05Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 22.36842105263158,
                    "unit": "%",
                    "timestamp": "2024-10-24T11:18:05Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 19.736842105263158,
                    "unit": "%",
                    "timestamp": "2024-10-24T11:48:07Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 21.05263157894737,
                    "unit": "%",
                    "timestamp": "2024-10-24T12:18:07Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 35.526315789473685,
                    "unit": "%",
                    "timestamp": "2024-10-24T12:48:08Z",
                    "ref": "milesight:194/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 35.526315789473685,
                    "unit": "%",
                    "timestamp": "2024-10-24T13:18:11Z",
                    "ref": "milesight:194/3330/5700"
                }
            ],
            "milesight:195/3330/5700": [
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 35.526315789473685,
                    "unit": "%",
                    "timestamp": "2024-10-24T09:10:00Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 22.36842105263158,
                    "unit": "%",
                    "timestamp": "2024-10-24T09:40:01Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 21.05263157894737,
                    "unit": "%",
                    "timestamp": "2024-10-24T10:10:01Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 21.05263157894737,
                    "unit": "%",
                    "timestamp": "2024-10-24T10:40:03Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 21.05263157894737,
                    "unit": "%",
                    "timestamp": "2024-10-24T11:10:03Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 21.05263157894737,
                    "unit": "%",
                    "timestamp": "2024-10-24T11:40:03Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 25,
                    "unit": "%",
                    "timestamp": "2024-10-24T12:10:05Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 27.63157894736842,
                    "unit": "%",
                    "timestamp": "2024-10-24T12:40:06Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 25,
                    "unit": "%",
                    "timestamp": "2024-10-24T13:10:04Z",
                    "ref": "milesight:195/3330/5700"
                },
                {
                    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
                    "urn": "urn:oma:lwm2m:ext:3435",
                    "v": 27.63157894736842,
                    "unit": "%",
                    "timestamp": "2024-10-24T13:40:07Z",
                    "ref": "milesight:195/3330/5700"
                }
            ]
        }
    }
}
`

var thingsJsonFormat = `
{
    "meta": {
        "totalRecords": 11
    },
    "data": [
        {
            "description": "",
            "energy": 475,
            "id": "2a15509b-8e32-428d-b2d9-cdaa1c59ed65",
            "location": {
                "latitude": 62.377963,
                "longitude": 17.34542
            },
            "name": "UC Framåt",
            "observedAt": "2024-10-24T12:29:37Z",
            "power": 0,
            "refDevices": [
                {
                    "deviceID": "63356661-3566-6636-3235-636631346439"
                },
                {
                    "deviceID": "34616163-3666-6232-3632-353730396163"
                },
                {
                    "deviceID": "a94dd95e-4da4-49e2-8431-d8d37894e5cc"
                }
            ],
            "tags": [
                "Byggnad"
            ],
            "temperature": 20.7,
            "tenant": "default",
            "type": "Building",
            "validURN": [
                "urn:oma:lwm2m:ext:3331",
                "urn:oma:lwm2m:ext:3328",
                "urn:oma:lwm2m:ext:3303"
            ]
        },
        {
            "currentLevel": 0.19,
            "description": "",
            "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
            "location": {
                "latitude": 62.425242,
                "longitude": 17.417382
            },
            "maxd": 0.94,
            "maxl": 0.76,
            "name": "Sandficka - Alnö",
            "observedAt": "2024-10-24T12:18:07Z",
            "percent": 25,
            "refDevices": [
                {
                    "deviceID": "milesight:193"
                },
                {
                    "deviceID": "milesight:194"
                },
                {
                    "deviceID": "milesight:195"
                }
            ],
            "subType": "Sandstorage",
            "tags": [
                "Sandficka",
                "Alnö"
            ],
            "tenant": "default",
            "type": "Container",
            "validURN": [
                "urn:oma:lwm2m:ext:3330"
            ]
        },
        {
            "currentLevel": 0.37,
            "description": "Beskrivning",
            "id": "17662c5d-27d2-4b43-8547-66df60ee6ba3",
            "location": {
                "latitude": 62.37894,
                "longitude": 17.33997
            },
            "maxd": 0.94,
            "maxl": 0.76,
            "name": "Soptunna-054",
            "observedAt": "2024-10-24T12:15:00Z",
            "percent": 48.68421052631579,
            "refDevices": [
                {
                    "deviceID": "milesight:54"
                }
            ],
            "subType": "WasteContainer",
            "tags": [
                "Soptunna"
            ],
            "tenant": "default",
            "type": "Container",
            "validURN": [
                "urn:oma:lwm2m:ext:3330"
            ]
        },
        {
            "description": "",
            "id": "333b31cd-cd78-4fc7-bc30-e4e1753d4070",
            "location": {
                "latitude": 62.34634,
                "longitude": 17.36489
            },
            "name": "Lifebuoy-001",
            "observedAt": "2024-10-24T12:29:40Z",
            "presence": false,
            "refDevices": [
                {
                    "deviceID": "df334c16-4f9b-4d88-a72c-629592a73d13"
                }
            ],
            "tags": [
                "Livboj"
            ],
            "tenant": "default",
            "type": "Lifebuoy",
            "validURN": [
                "urn:oma:lwm2m:ext:3200",
                "urn:oma:lwm2m:ext:3302"
            ]
        },
        {
            "cumulatedNumberOfPassages": 110,
            "currentState": false,
            "description": "Dörrbrytare",
            "id": "1a9d7d59-4ada-42fb-8c06-33f3fcc2d205",
            "location": {
                "latitude": 62.397726,
                "longitude": 17.340344
            },
            "name": "Gillbergsgatan",
            "observedAt": "2024-10-24T12:28:24Z",
            "passagesToday": 11,
            "refDevices": [
                {
                    "deviceID": "c43967c6-3382-48c4-9e93-ac9bd9ae8306"
                }
            ],
            "tags": [
                "Dörrbrytare"
            ],
            "tenant": "default",
            "type": "Passage",
            "validURN": [
                "urn:oma:lwm2m:ext:3200"
            ]
        },
        {
            "description": "",
            "id": "d9b2d63d-a233-4123-847a-8e5d8b2d4b5e",
            "location": {
                "latitude": 62.36955,
                "longitude": 17.27541
            },
            "name": "Sidsjön",
            "observedAt": "2024-10-24T12:13:38Z",
            "refDevices": [
                {
                    "deviceID": "fa9869d8-7c25-48ef-9274-0eb4c050bc82"
                }
            ],
            "subType": "Beach",
            "temperature": 7.031499999999999,
            "tenant": "default",
            "type": "PointOfInterest",
            "validURN": [
                "urn:oma:lwm2m:ext:3303"
            ]
        },
        {
            "description": "",
            "id": "2230d55e-0934-4759-b68b-92f82a358414",
            "location": {
                "latitude": 62.34634,
                "longitude": 17.36489
            },
            "name": "PumpingStation-001",
            "observedAt": "2024-10-24T12:29:40Z",
            "pumpingCumulativeTime": 1088000000000,
            "pumpingDuration": 46000000000,
            "pumpingObserved": false,
            "pumpingObservedAt": "2024-10-24T12:28:54Z",
            "refDevices": [
                {
                    "deviceID": "df334c16-4f9b-4d88-a72c-629592a73d13"
                },
                {
                    "deviceID": "d1c5a06d-e0e8-455b-b3b8-083cf8bd1cc8"
                }
            ],
            "tags": [
                "Pumpstation"
            ],
            "tenant": "default",
            "type": "PumpingStation",
            "validURN": [
                "urn:oma:lwm2m:ext:3200"
            ]
        },
        {
            "description": "DigIT",
            "id": "8b449db2-a3c5-48e0-a4fd-82fd50f0f8ae",
            "location": {
                "latitude": 62.34634,
                "longitude": 17.36489
            },
            "name": "DigIT-001",
            "observedAt": "2024-10-24T12:29:39Z",
            "refDevices": [
                {
                    "deviceID": "c43967c6-3382-48c4-9e93-ac9bd9ae8306"
                },
                {
                    "deviceID": "0ffbade6-802e-4581-bb75-4439526bd5e1"
                },
                {
                    "deviceID": "3b0d4af2-f498-4115-9f29-6ab3d688a9b2"
                }
            ],
            "tags": [
                "Rum",
                "DigIT"
            ],
            "temperature": 20.303999999999995,
            "tenant": "default",
            "type": "Room",
            "validURN": [
                "urn:oma:lwm2m:ext:3303"
            ]
        },
        {
            "currentLevel": 0.42,
            "description": "",
            "id": "edfc6fca-735a-49ee-ab9b-5c38843c61f9",
            "location": {
                "latitude": 62.34634,
                "longitude": 17.36489
            },
            "maxd": 0.94,
            "maxl": 0.76,
            "name": "Sewer-001",
            "observedAt": "2024-10-24T12:28:27Z",
            "overflowCumulativeTime": 0,
            "overflowDuration": null,
            "overflowObserved": false,
            "overflowObservedAt": null,
            "percent": 55.26315789473684,
            "refDevices": [
                {
                    "deviceID": "braddmatare-05"
                },
                {
                    "deviceID": "milesight:77"
                }
            ],
            "tags": [
                "Avlopp"
            ],
            "tenant": "default",
            "type": "Sewer",
            "validURN": [
                "urn:oma:lwm2m:ext:3330",
                "urn:oma:lwm2m:ext:3200"
            ]
        },
        {
            "backflow": false,
            "burst": false,
            "cumulativeVolume": 0,
            "description": "",
            "fraud": false,
            "id": "c693e877-b26d-4101-bf21-8cfef818c806",
            "leakage": false,
            "location": {
                "latitude": 0,
                "longitude": 0
            },
            "name": "06663",
            "observedAt": "0001-01-01T00:00:00Z",
            "refDevices": [
                {
                    "deviceID": "se:servanet:lora:msva:06663"
                }
            ],
            "tags": [
                "Vattenmätare"
            ],
            "tenant": "default",
            "type": "WaterMeter",
            "validURN": [
                "urn:oma:lwm2m:ext:3424"
            ]
        }
    ]
}
`

var thingsTypesJsonFormat = `
{
    "meta": {
        "totalRecords": 13
    },
    "data": [
        {
            "type": "Building",
            "name": "Building"
        },
        {
            "type": "Container",
            "name": "Container"
        },
        {
            "type": "Container",
            "subType": "WasteContainer",
            "name": "Container:WasteContainer"
        },
        {
            "type": "Container",
            "subType": "Sandstorage",
            "name": "Container:Sandstorage"
        },
        {
            "type": "Lifebuoy",
            "name": "Lifebuoy"
        },
        {
            "type": "Passage",
            "name": "Passage"
        },
        {
            "type": "PointOfInterest",
            "name": "PointOfInterest"
        },
        {
            "type": "PointOfInterest",
            "subType": "Beach",
            "name": "PointOfInterest:Beach"
        },
        {
            "type": "Pumpingstation",
            "name": "Pumpingstation"
        },
        {
            "type": "Room",
            "name": "Room"
        },
        {
            "type": "Sewer",
            "name": "Sewer"
        },
        {
            "type": "Sewer",
            "subType": "CombinedSewerOverflow",
            "name": "Sewer:CombinedSewerOverflow"
        },
        {
            "type": "WaterMeter",
            "name": "WaterMeter"
        }
    ]
}
`

var thingsTagsJsonFormat = `
{
    "meta": {
        "totalRecords": 11
    },
    "data": [
        "Alnö",
        "Avlopp",
        "Byggnad",
        "DigIT",
        "Dörrbrytare",
        "Livboj",
        "Pumpstation",
        "Rum",
        "Sandficka",
        "Soptunna",
        "Vattenmätare"
    ]
}
`
