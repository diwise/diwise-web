package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type ThingManagement interface {
	NewThing(ctx context.Context, t Thing) error
	GetThing(ctx context.Context, id string, params map[string][]string) (Thing, error)
	GetThings(ctx context.Context, offset, limit int, params map[string][]string) (ThingResult, error)
	UpdateThing(ctx context.Context, thingID string, fields map[string]any) error
	DeleteThing(ctx context.Context, thingID string) error
	GetTenants(ctx context.Context) []string
	GetTags(ctx context.Context) ([]string, error)
	GetTypes(ctx context.Context) ([]string, error)
	GetValidSensors(ctx context.Context, types []string) ([]SensorIdentifier, error)
	ConnectSensor(ctx context.Context, thingID string, refDevices []string) error
}

type Thing struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	SubType         string    `json:"subType,omitempty"`
	Name            string    `json:"name"`
	AlternativeName string    `json:"alternativeName,omitempty"`
	Description     string    `json:"description"`
	Location        Location  `json:"location,omitempty"`
	RefDevices      []Device  `json:"refDevices,omitempty"`
	Tags            []string  `json:"tags,omitempty"`
	Tenant          string    `json:"tenant"`
	ObservedAt      time.Time `json:"observedAt,omitempty"`
	ValidURNs       []string  `json:"validURN,omitempty"`

	Values     [][]Measurement `json:"-"`
	TypeValues ThingTypeValues `json:"-"`
}

type ThingTypeValues struct {
	// building
	Energy *float64 `json:"energy"`
	Power  *float64 `json:"power"`

	// lifebuoy
	Presence *bool `json:"presence"`

	// room
	Temperature *float64 `json:"temperature"`

	// container
	MaxDistance  *float64 `json:"maxd,omitempty"`
	MaxLevel     *float64 `json:"maxl,omitempty"`
	MeanLevel    *float64 `json:"meanl,omitempty"`
	Offset       *float64 `json:"offset,omitempty"`
	Angle        *float64 `json:"angle,omitempty"`
	CurrentLevel *float64 `json:"currentLevel"`
	Percent      *float64 `json:"percent"`

	// passage
	CumulatedNumberOfPassages *int64 `json:"cumulatedNumberOfPassages"`
	PassagesToday             *int64 `json:"passagesToday"`
	CurrentState              *bool  `json:"currentState"`

	// pumpingstation
	PumpingObserved   *bool          `json:"pumpingObserved"`
	PumpingObservedAt *time.Time     `json:"pumpingObservedAt"`
	PumpingDuration   *time.Duration `json:"pumpingDuration"`

	// sewer
	OverflowDuration   *time.Duration `json:"overflowDuration"`
	OverflowObserved   *bool          `json:"overflowObserved"`
	OverflowObservedAt *time.Time     `json:"overflowObservedAt"`

	// watermeter
	CumulativeVolume *float64 `json:"cumulativeVolume"`
	Leakage          *bool    `json:"leakage"`
	Burst            *bool    `json:"burst"`
	Backflow         *bool    `json:"backflow"`
	Fraud            *bool    `json:"fraud"`
}

func (t *Thing) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		return nil
	}

	t2 := struct {
		ID              string    `json:"id"`
		Type            string    `json:"type"`
		SubType         string    `json:"subType,omitempty"`
		Name            string    `json:"name"`
		AlternativeName string    `json:"alternativeName,omitempty"`
		Description     string    `json:"description"`
		Location        Location  `json:"location,omitempty"`
		RefDevices      []Device  `json:"refDevices,omitempty"`
		Tags            []string  `json:"tags,omitempty"`
		Tenant          string    `json:"tenant"`
		ObservedAt      time.Time `json:"observedAt,omitempty"`
		ValidURNs       []string  `json:"validURN,omitempty"`
	}{}
	err := json.Unmarshal(data, &t2)
	if err != nil {
		return err
	}

	t.ID = t2.ID
	t.Type = t2.Type
	t.SubType = t2.SubType
	t.Name = t2.Name
	t.AlternativeName = t2.AlternativeName
	t.Description = t2.Description
	t.Location = t2.Location
	t.RefDevices = t2.RefDevices
	t.Tags = t2.Tags
	t.Tenant = t2.Tenant
	t.ObservedAt = t2.ObservedAt
	t.ValidURNs = t2.ValidURNs

	typeValues := ThingTypeValues{}
	if err := json.Unmarshal(data, &typeValues); err == nil {
		t.TypeValues = typeValues
	}

	values := [][]Measurement{}
	v := struct {
		Values []Measurement `json:"values,omitempty"`
	}{}
	m := struct {
		Values map[string][]Measurement `json:"values,omitempty"`
	}{}

	if err := json.Unmarshal(data, &v); err == nil {
		values = append(values, v.Values)
	} else if err := json.Unmarshal(data, &m); err == nil {
		for _, vv := range m.Values {
			values = append(values, vv)
		}
	}

	t.Values = values

	return nil
}

type Device struct {
	DeviceID string `json:"deviceID"`
}

type Measurement struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Urn         string    `json:"urn"`
	BoolValue   *bool     `json:"vb,omitempty"`
	StringValue *string   `json:"vs,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Value       *float64  `json:"v,omitempty"`
	Count       *float64  `json:"count,omitempty"`
	RefDevice   string    `json:"ref,omitempty"`
}

type SensorIdentifier struct {
	SensorID string `json:"sensorID,omitempty"`
	DeviceID string `json:"deviceID"`
	Decoder  string `json:"decoder"`
}

type ThingResult struct {
	Things       []Thing
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}

func (a *App) GetThing(ctx context.Context, id string, args map[string][]string) (Thing, error) {
	params := url.Values{}
	params.Add("timerel", "after")
	params.Add("timeat", time.Now().Add(-24*time.Hour).Format(time.RFC3339))

	for k, v := range args {
		params[k] = v
	}

	res, err := a.get(ctx, a.thingManagementURL, id, params)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	err = json.Unmarshal(res.Data, &thing)
	if err != nil {
		return Thing{}, err
	}

	return thing, nil
}

func (a *App) GetThings(ctx context.Context, offset, limit int, args map[string][]string) (ThingResult, error) {
	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))

	for k, v := range args {
		params[k] = v
	}

	if t := params.Get("type"); t != "" {
		if strings.Contains(t, "-") {
			params.Set("type", strings.Split(t, "-")[0])
			params.Set("subType", strings.Split(t, "-")[1])
		}
	}

	res, err := a.get(ctx, a.thingManagementURL, "", params)
	if err != nil {
		return ThingResult{}, err
	}

	var things []Thing
	err = json.Unmarshal(res.Data, &things)
	if err != nil {
		return ThingResult{}, err
	}

	var total, off, lim int
	off = offset
	lim = limit

	if res.Meta != nil {
		total = int(res.Meta.TotalRecords)
		if res.Meta.Limit != nil {
			lim = int(*res.Meta.Limit)
		}
		if res.Meta.Offset != nil {
			off = int(*res.Meta.Offset)
		}
	}

	return ThingResult{
		Things:       things,
		TotalRecords: total,
		Offset:       off,
		Limit:        lim,
		Count:        len(things),
	}, nil
}

func (a *App) ConnectSensor(ctx context.Context, thingID string, refDevices []string) error {
	t, err := a.GetThing(ctx, thingID, nil)
	if err != nil {
		return err
	}

	devices := struct {
		RefDevices []Device `json:"refDevices"`
	}{}

	for _, ref := range refDevices {
		devices.RefDevices = append(devices.RefDevices, Device{DeviceID: ref})
	}

	b, err := json.Marshal(devices)
	if err != nil {
		return err
	}

	err = a.patch(ctx, a.thingManagementURL, t.ID, b)
	if err != nil {
		return err
	}

	return nil
}

func (a *App) NewThing(ctx context.Context, t Thing) error {
	if strings.Contains(t.Type, "-") {
		parts := strings.Split(t.Type, "-")
		t.Type = parts[0]
		t.SubType = parts[1]
	}

	b, err := json.Marshal(t)
	if err != nil {
		return err
	}

	return a.post(ctx, a.thingManagementURL, b)
}

func (a *App) GetValidSensors(ctx context.Context, types []string) ([]SensorIdentifier, error) {
	params := url.Values{
		"urn": types,
	}
	res, err := a.get(ctx, a.deviceManagementURL, "", params)
	if err != nil {
		return []SensorIdentifier{}, err
	}

	var sensors []Sensor
	err = json.Unmarshal(res.Data, &sensors)
	if err != nil {
		return []SensorIdentifier{}, err
	}

	var sensorIDs []SensorIdentifier
	for _, s := range sensors {
		sensorIDs = append(sensorIDs, SensorIdentifier{
			SensorID: s.SensorID,
			DeviceID: s.DeviceID,
			Decoder:  s.DeviceProfile.Decoder,
		})
	}

	return sensorIDs, nil
}

func (a *App) UpdateThing(ctx context.Context, thingID string, fields map[string]any) error {
	b, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	return a.patch(ctx, a.thingManagementURL, thingID, b)

}

func (a *App) DeleteThing(ctx context.Context, thingID string) error {
	u, err := url.Parse(fmt.Sprintf("%s/%s", strings.TrimSuffix(a.thingManagementURL, "/"), thingID))
	if err != nil {
		return err
	}

	return a.delete(ctx, u.String())
}

func (a *App) GetTypes(ctx context.Context) ([]string, error) {
	res, err := a.get(ctx, a.thingManagementURL, "types", url.Values{})
	if err != nil {
		return []string{}, err
	}

	var thingTypes = []struct {
		Type    string `json:"type"`
		SubType string `json:"subType,omitempty"`
		Name    string `json:"name"`
	}{}

	err = json.Unmarshal(res.Data, &thingTypes)
	if err != nil {
		return []string{}, err
	}

	var types []string

	for _, t := range thingTypes {
		types = append(types, t.Name)
	}

	return types, nil
}
