package devices

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"

	"github.com/diwise/diwise-web/internal/application/client"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("diwise-web/app/devices")

type Service struct {
	client *client.Client
}

func NewService(client *client.Client) *Service {
	return &Service{client: client}
}

func (s *Service) GetDevice(ctx context.Context, id string) (Device, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var res *client.ApiResponse
	res, err = s.client.Get(ctx, s.client.DeviceManagementURL(), id, url.Values{})
	if err != nil {
		return Device{}, err
	}

	var device Device
	err = json.Unmarshal(res.Data, &device)
	if err != nil {
		return Device{}, err
	}

	return device, nil
}

func (s *Service) GetDevices(ctx context.Context, offset, limit int, args map[string][]string) (DeviceResult, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-devices")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))
	maps.Copy(params, args)

	var res *client.ApiResponse
	res, err = s.client.Get(ctx, s.client.DeviceManagementURL(), "", params)
	if err != nil {
		return DeviceResult{}, err
	}

	var devices []Device
	err = json.Unmarshal(res.Data, &devices)
	if err != nil {
		return DeviceResult{}, err
	}

	return DeviceResult{
		Devices:      devices,
		TotalRecords: int(res.Meta.TotalRecords),
		Offset:       int(*res.Meta.Offset),
		Limit:        int(*res.Meta.Limit),
		Count:        len(devices),
	}, nil
}

func (s *Service) GetSensors(ctx context.Context, offset, limit int, args map[string][]string) (SensorResult, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-devices")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("offset", fmt.Sprintf("%d", offset))
	maps.Copy(params, args)

	var res *client.ApiResponse
	res, err = s.client.Get(ctx, s.client.SensorsManagementURL(), "", params)
	if err != nil {
		return SensorResult{}, err
	}

	var sensors []Sensor
	err = json.Unmarshal(res.Data, &sensors)
	if err != nil {
		return SensorResult{}, err
	}

	return SensorResult{
		Sensors:      sensors,
		TotalRecords: int(res.Meta.TotalRecords),
		Offset:       int(*res.Meta.Offset),
		Limit:        int(*res.Meta.Limit),
		Count:        len(sensors),
	}, nil
}

func (s *Service) GetSensorStatus(ctx context.Context, id string) ([]SensorStatus, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-sensor-status")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	res, err := s.client.Get(ctx, s.client.DeviceManagementURL(), fmt.Sprintf("/%s/status", id), url.Values{})
	if err != nil {
		return []SensorStatus{}, err
	}

	var statuses []SensorStatus
	err = json.Unmarshal(res.Data, &statuses)
	if err != nil {
		return []SensorStatus{}, err
	}

	return statuses, nil
}

func (s *Service) UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error {
	var err error
	ctx, span := tracer.Start(ctx, "update-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var b []byte
	b, err = json.Marshal(fields)
	if err != nil {
		return err
	}

	return s.client.Patch(ctx, s.client.DeviceManagementURL(), deviceID, b)
}

func (s *Service) UpdateSensor(ctx context.Context, sensorID string, fields map[string]any) error {
	var err error
	ctx, span := tracer.Start(ctx, "update-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	var b []byte
	b, err = json.Marshal(fields)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/%s", s.client.SensorsManagementURL(), sensorID)
	return s.client.Put(ctx, endpoint, b)
}

func (s *Service) Attach(ctx context.Context, deviceID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "attach-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	sensorID, ok := AttachSensorIDFromContext(ctx)
	if !ok || sensorID == "" {
		return fmt.Errorf("missing sensorID")
	}

	body, err := json.Marshal(map[string]string{"sensorID": sensorID})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/%s/sensor", s.client.DeviceManagementURL(), deviceID)
	err = s.client.Put(ctx, endpoint, body)
	return err
}

func (s *Service) Deattach(ctx context.Context, deviceID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "deattach-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	endpoint := fmt.Sprintf("%s/%s/sensor", s.client.DeviceManagementURL(), deviceID)
	err = s.client.Delete(ctx, endpoint)
	return err
}

func (s *Service) GetStatistics(ctx context.Context) (Statistics, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-statistics")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	errs := make(chan error, 5)
	count := func(key, value string, result *int) {
		go func() {
			var e error
			defer func() { errs <- e }()

			params := url.Values{}
			params.Add("limit", "1")
			if key != "" && value != "" {
				params.Add(key, value)
			}

			res, e := s.client.Get(ctx, s.client.DeviceManagementURL(), "", params)
			if e == nil && res.Meta != nil {
				*result = int(res.Meta.TotalRecords)
			} else {
				*result = 0
			}
		}()
	}

	stats := Statistics{}
	count("", "", &stats.Total)
	count("online", "true", &stats.Online)
	count("active", "true", &stats.Active)
	count("active", "false", &stats.Inactive)
	count("profilename", "unknown", &stats.Unknown)

	for range 5 {
		err = errors.Join(err, <-errs)
	}

	return stats, err
}
