package sensors

import (
	"context"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/diwise-web/internal/pkg/presentation/web/components"
)

func composeSensorDetailsViewModel(ctx context.Context, id string, app application.DeviceManagement) (*components.SensorDetailsViewModel, error) {
	sensor, err := app.GetSensor(ctx, id)
	if err != nil {
		return nil, err
	}

	tenants := app.GetTenants(ctx)
	deviceProfiles := app.GetDeviceProfiles(ctx)

	dp := []components.DeviceProfile{}
	for _, p := range deviceProfiles {
		types := []string{}
		if p.Types != nil {
			types = *p.Types
		}
		dp = append(dp, components.DeviceProfile{
			Name:     p.Name,
			Decoder:  p.Decoder,
			Interval: p.Interval,
			Types:    types,
		})
	}

	types := []string{}
	for _, tp := range sensor.Types {
		types = append(types, tp.URN)
	}

	detailsViewModel := components.SensorDetailsViewModel{
		DeviceID:          sensor.DeviceID,
		Name:              sensor.Name,
		Latitude:          sensor.Location.Latitude,
		Longitude:         sensor.Location.Longitude,
		DeviceProfileName: sensor.DeviceProfile.Name,
		Tenant:            sensor.Tenant,
		Description:       sensor.Description,
		Active:            sensor.Active,
		Types:             types,
		Organisations:     tenants,
		DeviceProfiles:    dp,
	}
	return &detailsViewModel, nil
}

func composeSensorListViewModel(ctx context.Context, offset, limit, pageIndex int, app application.DeviceManagement) (*components.SensorListViewModel, error) {
	sensorResult, err := app.GetSensors(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	sumOfStuff := app.GetStatistics(ctx)

	listViewModel := components.SensorListViewModel{
		Statistics: components.StatisticsViewModel{
			Total:    sumOfStuff.Total,
			Active:   sumOfStuff.Active,
			Inactive: sumOfStuff.Inactive,
			Online:   sumOfStuff.Online,
			Unknown:  sumOfStuff.Unknown,
		},
		Meta: components.Meta{
			TotalRecords: sensorResult.TotalRecords,
			Offset:       sensorResult.Offset,
			Limit:        sensorResult.Limit,
			Count:        sensorResult.Count,
			PageIndex:    pageIndex,
		},
	}

	for _, sensor := range sensorResult.Sensors {
		listViewModel.Sensors = append(listViewModel.Sensors, components.SensorViewModel{
			Active:       sensor.Active,
			DevEUI:       sensor.SensorID,
			DeviceID:     sensor.DeviceID,
			Name:         sensor.Name,
			BatteryLevel: sensor.DeviceStatus.BatteryLevel,
			LastSeen:     sensor.DeviceState.ObservedAt,
			HasAlerts:    false, //TODO: fix this
		})
	}

	return &listViewModel, nil
}
