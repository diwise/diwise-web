package devmode

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"slices"

	"github.com/diwise/diwise-web/internal/pkg/application"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

//go:embed json/*.json
var jsonFiles embed.FS

var fileMap = map[string]string{
	"/things":       "json/things.json",
	"/things/tags":  "json/tags.json",
	"/things/types": "json/types.json",
	"/things/2a15509b-8e32-428d-b2d9-cdaa1c59ed65": "json/2a15509b-8e32-428d-b2d9-cdaa1c59ed65.json",
	"/things/f47ac10b-58cc-4372-a567-0e02b2c3d479": "json/f47ac10b-58cc-4372-a567-0e02b2c3d479.json",
	"/things/17662c5d-27d2-4b43-8547-66df60ee6ba3": "json/17662c5d-27d2-4b43-8547-66df60ee6ba3.json",
	"/things/333b31cd-cd78-4fc7-bc30-e4e1753d4070": "json/333b31cd-cd78-4fc7-bc30-e4e1753d4070.json",
	"/things/1a9d7d59-4ada-42fb-8c06-33f3fcc2d205": "json/1a9d7d59-4ada-42fb-8c06-33f3fcc2d205.json",
	"/things/d9b2d63d-a233-4123-847a-8e5d8b2d4b5e": "json/d9b2d63d-a233-4123-847a-8e5d8b2d4b5e.json",
	"/things/2230d55e-0934-4759-b68b-92f82a358414": "json/2230d55e-0934-4759-b68b-92f82a358414.json",
	"/things/8b449db2-a3c5-48e0-a4fd-82fd50f0f8ae": "json/8b449db2-a3c5-48e0-a4fd-82fd50f0f8ae.json",
	"/things/edfc6fca-735a-49ee-ab9b-5c38843c61f9": "json/edfc6fca-735a-49ee-ab9b-5c38843c61f9.json",
	"/things/3f2504e0-4f89-11d3-9a0c-0305e82c3301": "json/3f2504e0-4f89-11d3-9a0c-0305e82c3301.json",
}

func NewThingsHandler(ctx context.Context) http.HandlerFunc {
	logger := logging.GetFromContext(ctx)

	return func(w http.ResponseWriter, r *http.Request) {

		u := r.URL.String()
		p := r.URL.Path
		response := application.ApiResponse{}

		logger.Info("DEVMODE THINGS REQUEST", "path", p, "url", u)

		if p == "/things/values" {
			thingID := r.URL.Query().Get("thingid")
			valuesResponse, err := newLatestValuesResponse(thingID)
			if err != nil {
				logger.Error("DEVMODE THINGS ERROR", "error", err)
				http.Error(w, "could not render thing values", http.StatusInternalServerError)
				return
			}

			w.Header()["Content-Type"] = []string{"application/json"}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(&valuesResponse)
			return
		}

		fileName, ok := fileMap[p]
		if !ok {
			http.NotFound(w, r)
			return
		}

		c, err := jsonFiles.ReadFile(fileName)
		if err != nil {
			logger.Error("DEVMODE THINGS ERROR", "error", err)
			http.Error(w, "could not read things from json", http.StatusInternalServerError)
			return
		}

		err = json.Unmarshal(c, &response)
		if err != nil {
			logger.Error("DEVMODE THINGS ERROR", "error", err)
			http.Error(w, "could not render things", http.StatusInternalServerError)
			return
		}

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(&response)
	}
}

func newLatestValuesResponse(thingID string) (application.ApiResponse, error) {
	if thingID == "" {
		return application.ApiResponse{
			Meta: &application.Meta{
				Count:        0,
				TotalRecords: 0,
			},
			Data:  []byte("[]"),
			Links: &application.Links{},
		}, nil
	}

	fileName, ok := fileMap["/things/"+thingID]
	if !ok {
		return application.ApiResponse{
			Meta: &application.Meta{
				Count:        0,
				TotalRecords: 0,
			},
			Data:  []byte("[]"),
			Links: &application.Links{},
		}, nil
	}

	c, err := jsonFiles.ReadFile(fileName)
	if err != nil {
		return application.ApiResponse{}, err
	}

	var thingResp application.ApiResponse
	if err := json.Unmarshal(c, &thingResp); err != nil {
		return application.ApiResponse{}, err
	}

	var thing application.Thing
	if err := json.Unmarshal(thingResp.Data, &thing); err != nil {
		return application.ApiResponse{}, err
	}

	values := make([]application.Measurement, 0)
	for _, grouped := range thing.Values {
		if len(grouped) == 0 {
			continue
		}

		latest := slices.MaxFunc(grouped, func(a, b application.Measurement) int {
			switch {
			case a.Timestamp.Before(b.Timestamp):
				return -1
			case a.Timestamp.After(b.Timestamp):
				return 1
			default:
				return 0
			}
		})
		values = append(values, latest)
	}

	data, err := json.Marshal(values)
	if err != nil {
		return application.ApiResponse{}, err
	}

	count := uint64(len(values))
	return application.ApiResponse{
		Meta: &application.Meta{
			Count:        count,
			TotalRecords: count,
		},
		Data:  data,
		Links: &application.Links{},
	}, nil
}
