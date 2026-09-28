package rules

// Fasta värdelistor för match-fält (PLAN002 steg 7.1). Källor:
// object/resource/env är uppräkningen ur iot-transform-fiware
// deployments/models (seed-reglerna); relation är den enda persisterade
// relationstypen i iot-things-v2 (övriga namn tillåts av backend men
// persisteras inte som länkar). Datalists är förslag, inte tvång —
// backend-valideringen (validate-knappen) förblir sanningen.
//
// sensorType/device/port lämnas fritext med flit: inga seed-värden finns
// att lista, och nya enheter/sensorer tillför egna värden löpande.
func MeasurementObjects() []string {
	return []string{
		"urn:oma:lwm2m:ext:3301",
		"urn:oma:lwm2m:ext:3302",
		"urn:oma:lwm2m:ext:3303",
		"urn:oma:lwm2m:ext:3304",
		"urn:oma:lwm2m:ext:3323",
		"urn:oma:lwm2m:ext:3324",
		"urn:oma:lwm2m:ext:3327",
		"urn:oma:lwm2m:ext:3424",
		"urn:oma:lwm2m:ext:3428",
		"urn:oma:lwm2m:ext:3434",
	}
}

func MeasurementEnvs() []string {
	// Exakt seed-uppräkningen (indoors/soil/air) — inga påhittade värden.
	return []string{"indoors", "soil", "air"}
}

func MeasurementResources() []string {
	return []string{"1", "3", "5", "9", "11", "15", "17", "19", "5500", "5700", "64007"}
}

func RelationNames() []string {
	return []string{"partOf"}
}
