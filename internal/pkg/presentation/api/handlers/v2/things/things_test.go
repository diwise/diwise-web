package things

import (
	"net/url"
	"testing"

	"github.com/matryer/is"
)

func TestSelectedValuesSplitsCommaSeparatedAndDeduplicates(t *testing.T) {
	is := is.New(t)

	values, err := url.ParseQuery("type=container-wastecontainer,sewer&type=sewer")
	is.NoErr(err)

	selected := selectedValues(values, "type")

	is.Equal([]string{"container-wastecontainer", "sewer"}, selected)
}

func TestSelectedValuesReturnsNilForMissingKey(t *testing.T) {
	is := is.New(t)

	selected := selectedValues(url.Values{}, "tags")

	is.Equal(0, len(selected))
}
