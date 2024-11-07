package application

import (
	"encoding/json"
	"testing"

	"github.com/matryer/is"
)

func TestUnmarshalThing(t *testing.T) {
	is := is.New(t)
	thing := Thing{}
	err := json.Unmarshal([]byte(groupByRefThing), &thing)
	is.NoErr(err)

	err = json.Unmarshal([]byte(flat), &thing)
	is.NoErr(err)
}

const groupByRefThing = `
{
	"currentLevel": 0.24,
	"description": "",
	"id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"location": {
		"latitude": 62.425242,
		"longitude": 17.417382
	},
	"maxd": 0.94,
	"maxl": 0.76,
	"name": "Sandficka - Alnö",
	"observedAt": "2024-10-22T12:47:00Z",
	"percent": 31.57894736842105,
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
	"values": {
		"milesight:193/3330/5700": [
			{
				"id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
				"urn": "urn:oma:lwm2m:ext:3435",
				"v": 61.8421052631579,
				"unit": "%",
				"timestamp": "2024-10-21T05:54:43Z",
				"ref": "milesight:193/3330/5700"
			}
		],
		"milesight:194/3330/5700": [
			{
				"id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
				"urn": "urn:oma:lwm2m:ext:3435",
				"v": 36.8421052631579,
				"unit": "%",
				"timestamp": "2024-10-18T13:44:52Z",
				"ref": "milesight:194/3330/5700"
			}
		],
		"milesight:195/3330/5700": [
			{
				"id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
				"urn": "urn:oma:lwm2m:ext:3435",
				"v": 31.57894736842105,
				"unit": "%",
				"timestamp": "2024-10-18T14:06:55Z",
				"ref": "milesight:195/3330/5700"
			}
		]
	}
}
`

const flat string = `
 {
	"currentLevel": 0.18,
	"description": "",
	"id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
	"location": {
		"latitude": 62.425242,
		"longitude": 17.417382
	},
	"maxd": 0.94,
	"maxl": 0.76,
	"name": "Sandficka - Alnö",
	"observedAt": "2024-10-22T15:47:06Z",
	"percent": 23.684210526315788,
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
	"values": [
		{
			"id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
			"urn": "urn:oma:lwm2m:ext:3435",
			"v": 36.8421052631579,
			"unit": "%",
			"timestamp": "2024-10-18T13:44:52Z",
			"ref": "milesight:194/3330/5700"
		},
		{
			"id": "f47ac10b-58cc-4372-a567-0e02b2c3d479/3435/2",
			"urn": "urn:oma:lwm2m:ext:3435",
			"v": 31.57894736842105,
			"unit": "%",
			"timestamp": "2024-10-18T14:06:55Z",
			"ref": "milesight:195/3330/5700"
		}
	]
    
}
`
