package shared

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/matryer/is"
)

func TestSelectBoxFieldRemoteSearchUsesCustomSelectboxMarkup(t *testing.T) {
	is := is.New(t)

	component := SelectBoxField(SelectBoxFieldProps{
		FieldID:           "thing-current-device",
		Name:              "currentDevice",
		Label:             "Connected sensor",
		Placeholder:       "Select sensor",
		SearchPlaceholder: "Search sensor",
		SearchURL:         "/components/things/search-compatible-sensor-options",
		SearchExtraQuery: map[string]string{
			"usage":   "multi",
			"thingID": "thing-1",
		},
		RemoteSearch: true,
		LoadingText:  "Searching...",
		EmptyText:    "No sensors found.",
		Options: []SelectBoxFieldOption{{
			Value:    "sensor-1",
			Label:    "sensor-1",
			Selected: true,
		}},
	})

	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)

	body := buf.String()
	is.NoErr(err)
	is.True(strings.Contains(body, `hx-get="/components/things/search-compatible-sensor-options?thingID=thing-1&amp;usage=multi"`))
	is.True(strings.Contains(body, `hx-target="#thing-current-device-options"`))
	is.True(strings.Contains(body, `hx-trigger="input changed delay:300ms"`))
	is.True(strings.Contains(body, `name="currentDevice"`))
	is.True(strings.Contains(body, `data-diwise-selectbox-options-root`))
	is.True(strings.Contains(body, `data-tui-selectbox-value="sensor-1"`))
}

func TestMultiSelectBoxFieldRemoteSearchSeedsSelectedValues(t *testing.T) {
	is := is.New(t)

	component := MultiSelectBoxField(MultiSelectBoxFieldProps{
		FieldID:           "thing-current-device",
		Name:              "currentDevice",
		Label:             "Connected sensors",
		Placeholder:       "Select sensors",
		SearchPlaceholder: "Search sensor",
		SearchURL:         "/components/things/search-compatible-sensor-options",
		SearchExtraQuery: map[string]string{
			"usage":   "multi",
			"thingID": "thing-1",
		},
		RemoteSearch: true,
		LoadingText:  "Searching...",
		EmptyText:    "No sensors found.",
		ShowPills:    true,
		Options: []SelectBoxFieldOption{
			{Value: "sensor-1", Label: "sensor-1", Selected: true},
			{Value: "sensor-2", Label: "sensor-2", Selected: true},
		},
	})

	var buf bytes.Buffer
	err := component.Render(context.Background(), &buf)

	body := buf.String()
	is.NoErr(err)
	is.True(strings.Contains(body, `data-tui-selectbox-multiple="true"`))
	is.True(strings.Contains(body, `data-tui-selectbox-show-pills="true"`))
	is.True(strings.Contains(body, `value="sensor-1,sensor-2"`))
	is.True(strings.Contains(body, `data-diwise-selectbox-options-root`))
	is.True(strings.Contains(body, `data-tui-selectbox-value="sensor-1"`))
	is.True(strings.Contains(body, `data-tui-selectbox-value="sensor-2"`))
}
