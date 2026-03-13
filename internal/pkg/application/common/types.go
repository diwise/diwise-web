package common

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Location struct {
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type Metadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type InputParam func(v *url.Values)

func WithReverse(reverse bool) InputParam {
	return func(v *url.Values) {
		v.Set("reverse", boolToString(reverse))
	}
}

func WithLimit(limit int) InputParam {
	return func(v *url.Values) {
		v.Set("limit", intToString(limit))
	}
}

func WithLastN(lastN bool) InputParam {
	return func(v *url.Values) {
		v.Set("lastN", boolToString(lastN))
	}
}

func WithTimeRel(timeRel string, timeAt, endTimeAt time.Time) InputParam {
	return func(v *url.Values) {
		v.Set("timeRel", timeRel)
		v.Set("timeAt", timeAt.UTC().Format(time.RFC3339))
		v.Set("endTimeAt", endTimeAt.UTC().Format(time.RFC3339))
	}
}

func WithAggrMethods(methods ...string) InputParam {
	return func(v *url.Values) {
		v.Set("aggrMethods", joinComma(methods...))
	}
}

func WithTimeUnit(timeUnit string) InputParam {
	return func(v *url.Values) {
		v.Set("timeUnit", timeUnit)
	}
}

func WithAfter(timeAt time.Time) InputParam {
	return func(v *url.Values) {
		v.Set("timeRel", "after")
		v.Set("timeAt", timeAt.UTC().Format(time.RFC3339))
	}
}

func WithBoolValue(boolValue bool) InputParam {
	return func(v *url.Values) {
		v.Set("vb", boolToString(boolValue))
	}
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func intToString(v int) string {
	return fmt.Sprintf("%d", v)
}

func joinComma(values ...string) string {
	return strings.Join(values, ",")
}
