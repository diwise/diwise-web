package rules

// Fixture är ett exempel-message per event-kind för preview-formuläret.
// Kopierade från iot-transform-fiware tester/seed 2026-09-24 (register_test,
// values/relations/lifecycle-tester, 93/94-seed): mätpack i produktionsform
// (bn/bt), värdesevent med deskriptor, relations set/removed med
// from/to-descriptors, lifecycle created/deleted.
type Fixture struct {
	Name    string
	Label   string
	Kind    string
	Message string
}

func Fixtures() []Fixture {
	return []Fixture{
		{
			Name:  "measurement",
			Label: "Mätvärdespack",
			Kind:  "measurement",
			Message: `{"pack": [
  {"bn": "dev-1/3303/", "bt": 1725979200, "n": "0", "vs": "urn:oma:lwm2m:ext:3303"},
  {"n": "5700", "v": 22.5},
  {"bn": "dev-1/", "n": "env", "vs": "indoors"},
  {"bn": "dev-1/", "n": "tenant", "vs": "t1"}
], "timestamp": "2024-09-10T12:00:00Z"}`,
		},
		{
			Name:  "values",
			Label: "things.v1.values",
			Kind:  "things.v1.values",
			Message: `{"schema": "v1", "eventId": "v1/t1/room-a/m1", "tenant": "t1",
  "thingId": "room-a", "thingType": "room", "thingTypeVersion": "1",
  "configRevision": 2, "publishedAt": "2024-09-10T12:00:00Z",
  "thing": {"thingId": "room-a", "name": "Rum A", "category": "room"},
  "values": [
    {"thingId": "room-a", "tenant": "t1", "propertyId": "temperature",
     "value": 21.5, "quality": "ok", "unit": "Cel",
     "source": "s/3303/5700", "observedAt": "2024-09-10T12:00:00Z"}
  ]}`,
		},
		{
			Name:  "relations-set",
			Label: "things.v1.relations (set)",
			Kind:  "things.v1.relations",
			Message: `{"relationId": "partof-room-a", "type": "partOf",
  "fromId": "room-a", "toId": "building-1", "tenant": "t1",
  "revision": 2, "changedAt": "2024-09-10T12:00:00Z",
  "fromThingType": "room",
  "from": {"thingId": "room-a", "name": "Rum A", "category": "room"},
  "toThingType": "building",
  "to": {"thingId": "building-1", "name": "Huset", "category": "building"}}`,
		},
		{
			Name:  "relations-removed",
			Label: "things.v1.relations (removed)",
			Kind:  "things.v1.relations",
			Message: `{"relationId": "partof-room-a", "type": "partOf",
  "fromId": "room-a", "toId": "building-1", "removed": true, "tenant": "t1",
  "revision": 3, "changedAt": "2024-09-10T12:00:00Z",
  "fromThingType": "room",
  "from": {"thingId": "room-a", "name": "Rum A", "category": "room"},
  "toThingType": "building",
  "to": {"thingId": "building-1", "name": "Huset", "category": "building"}}`,
		},
		{
			Name:  "lifecycle-created",
			Label: "things.v1.lifecycle (created)",
			Kind:  "things.v1.lifecycle",
			Message: `{"eventId": "v1/t1/room-a/lifecycle/created/1", "tenant": "t1",
  "thingId": "room-a", "kind": "created",
  "thingType": "room", "thingTypeVersion": "1",
  "configRevision": 1, "changedAt": "2024-09-10T12:00:00Z",
  "thing": {"thingId": "room-a", "name": "Rum A", "category": "room"}}`,
		},
		{
			Name:  "lifecycle-deleted",
			Label: "things.v1.lifecycle (deleted)",
			Kind:  "things.v1.lifecycle",
			Message: `{"eventId": "v1/t1/room-a/lifecycle/deleted/4", "tenant": "t1",
  "thingId": "room-a", "kind": "deleted",
  "thingType": "room", "thingTypeVersion": "1",
  "configRevision": 4, "changedAt": "2024-09-10T12:00:00Z",
  "thing": {"thingId": "room-a", "name": "Rum A", "category": "room"}}`,
		},
	}
}

// FixtureByName slår upp en fixture; okänd ger tom.
func FixtureByName(name string) (Fixture, bool) {
	for _, f := range Fixtures() {
		if f.Name == name {
			return f, true
		}
	}
	return Fixture{}, false
}
