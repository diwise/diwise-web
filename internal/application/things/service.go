package things

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"time"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/things")

type Service struct {
	client *client.Client
}

func NewService(client *client.Client) *Service {
	return &Service{client: client}
}

func (s *Service) NewThing(ctx context.Context, t Thing) error {
	var err error
	ctx, span := tracer.Start(ctx, "new-thing")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	if strings.Contains(t.Type, "-") {
		parts := strings.Split(t.Type, "-")
		t.Type = parts[0]
		t.SubType = parts[1]
	}

	b, err := json.Marshal(t)
	if err != nil {
		return err
	}

	return s.client.Post(ctx, s.client.ThingManagementURL(), b)
}

func (s *Service) GetThing(ctx context.Context, id string, args map[string][]string) (Thing, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-thing")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("timerel", "after")
	params.Add("timeat", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	maps.Copy(params, args)

	res, err := s.client.Get(ctx, s.client.ThingManagementURL(), id, params)
	if err != nil {
		return Thing{}, err
	}

	var thing Thing
	if err = json.Unmarshal(res.Data, &thing); err != nil {
		return Thing{}, err
	}

	return thing, nil
}

func (s *Service) GetLatestValues(ctx context.Context, thingID string) ([]Measurement, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-latest-values")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("thingid", thingID)
	params.Add("latest", "true")

	res, err := s.client.Get(ctx, s.client.ThingManagementURL(), "values", params)
	if err != nil {
		return []Measurement{}, err
	}

	measurements := []Measurement{}
	if err = json.Unmarshal(res.Data, &measurements); err != nil {
		return []Measurement{}, err
	}

	return measurements, nil
}

func (s *Service) GetThings(ctx context.Context, offset, limit int, args map[string][]string) (Result, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-things")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))
	maps.Copy(params, args)

	if t := params.Get("type"); t != "" && strings.Contains(t, "-") {
		parts := strings.Split(t, "-")
		params.Set("type", parts[0])
		params.Set("subType", parts[1])
	}

	res, err := s.client.Get(ctx, s.client.ThingManagementURL(), "", params)
	if err != nil {
		return Result{}, err
	}

	var things []Thing
	if err = json.Unmarshal(res.Data, &things); err != nil {
		return Result{}, err
	}

	total, off, lim := 0, offset, limit
	if res.Meta != nil {
		total = int(res.Meta.TotalRecords)
		if res.Meta.Limit != nil {
			lim = int(*res.Meta.Limit)
		}
		if res.Meta.Offset != nil {
			off = int(*res.Meta.Offset)
		}
	}

	return Result{Things: things, TotalRecords: total, Offset: off, Limit: lim, Count: len(things)}, nil
}

func (s *Service) ConnectSensor(ctx context.Context, thingID string, refDevices []string) error {
	var err error
	ctx, span := tracer.Start(ctx, "connect-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	t, err := s.GetThing(ctx, thingID, nil)
	if err != nil {
		return err
	}

	devices := struct {
		RefDevices []RefDevice `json:"refDevices"`
	}{}
	for _, ref := range refDevices {
		devices.RefDevices = append(devices.RefDevices, RefDevice{DeviceID: ref})
	}

	b, err := json.Marshal(devices)
	if err != nil {
		return err
	}

	return s.client.Patch(ctx, s.client.ThingManagementURL(), t.ID, b)
}

func (s *Service) GetValidSensors(ctx context.Context, urns []string) ([]SensorIdentifier, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-valid-sensors")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{"urn": urns}
	res, err := s.client.Get(ctx, s.client.DeviceManagementURL(), "", params)
	if err != nil {
		return []SensorIdentifier{}, err
	}

	var devices []struct {
		SensorID      string `json:"sensorID,omitempty"`
		DeviceID      string `json:"deviceID"`
		DeviceProfile struct {
			Decoder string `json:"decoder,omitempty"`
		} `json:"deviceProfile"`
	}
	if err = json.Unmarshal(res.Data, &devices); err != nil {
		return []SensorIdentifier{}, err
	}

	ids := make([]SensorIdentifier, 0, len(devices))
	for _, d := range devices {
		ids = append(ids, SensorIdentifier{SensorID: d.SensorID, DeviceID: d.DeviceID, Decoder: d.DeviceProfile.Decoder})
	}

	return ids, nil
}

func (s *Service) UpdateThing(ctx context.Context, thingID string, fields map[string]any) error {
	var err error
	ctx, span := tracer.Start(ctx, "update-thing")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	b, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	return s.client.Patch(ctx, s.client.ThingManagementURL(), thingID, b)
}

func (s *Service) DeleteThing(ctx context.Context, thingID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "delete-thing")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	u, err := url.Parse(fmt.Sprintf("%s/%s", strings.TrimSuffix(s.client.ThingManagementURL(), "/"), thingID))
	if err != nil {
		return err
	}

	return s.client.Delete(ctx, u.String())
}

func (s *Service) GetTags(ctx context.Context) ([]string, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-tags")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	res, err := s.client.Get(ctx, s.client.ThingManagementURL(), "tags", url.Values{})
	if err != nil {
		return []string{}, err
	}

	var tags []string
	if err = json.Unmarshal(res.Data, &tags); err != nil {
		return []string{}, err
	}

	return tags, nil
}

func (s *Service) GetTypes(ctx context.Context) ([]string, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-types")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	res, err := s.client.Get(ctx, s.client.ThingManagementURL(), "types", url.Values{})
	if err != nil {
		return []string{}, err
	}

	var thingTypes = []struct {
		Type    string `json:"type"`
		SubType string `json:"subType,omitempty"`
		Name    string `json:"name"`
	}{}
	if err = json.Unmarshal(res.Data, &thingTypes); err != nil {
		return []string{}, err
	}

	types := make([]string, 0, len(thingTypes))
	for _, t := range thingTypes {
		types = append(types, t.Name)
	}

	return types, nil
}
