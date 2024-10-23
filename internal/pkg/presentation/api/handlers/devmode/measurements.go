package devmode

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func NewMeasurementsHandler(_ context.Context) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		timeAt, _ := url.QueryUnescape(r.FormValue("timeAt"))
		endTimeAt, _ := url.QueryUnescape(r.FormValue("endTimeAt"))

		date, _ := time.Parse(time.RFC3339, timeAt)
		endDate, _ := time.Parse(time.RFC3339, endTimeAt)

		const jsonFmt string = `{"timestamp":"%sT00:00:00Z","sum":%d}`
		jsons := make([]string, 0, 100)

		rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
		maxSum := 100000 + (rnd.Float32() * 100000)
		minSum := rnd.Float32() * maxSum
		currentSum := minSum

		for date.Before(endDate) && date.Before(time.Now()) {
			days := endDate.Sub(date) / (24 * time.Hour)
			currentSum += float32(maxSum-currentSum) / float32(days)

			jsons = append(jsons, fmt.Sprintf(jsonFmt, date.Format("2006-01-02"), int(currentSum)))
			date = date.Add(24 * time.Hour)
		}

		const responseFmt string = `{"meta":{"totalRecords":%d}, "data":{"values":[%s]}}`
		response := fmt.Sprintf(responseFmt, len(jsons), strings.Join(jsons, ","))

		w.Header()["Content-Type"] = []string{"application/json"}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}
}
