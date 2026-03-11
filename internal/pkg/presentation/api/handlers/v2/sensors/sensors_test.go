package sensors

import (
	"net/url"
	"testing"

	"github.com/matryer/is"
)

func TestNormalizeTypeFilterSplitsCommaSeparatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("search=&type=elsys%2Cmilesight&active=&online=&lastseen=")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"elsys", "milesight"}, selected)
	is.Equal("active=&lastseen=&online=&search=&type=elsys&type=milesight", params.Encode())
}

func TestNormalizeTypeFilterPreservesRepeatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("type=elsys&type=milesight&type=elsys")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"elsys", "milesight"}, selected)
	is.Equal("type=elsys&type=milesight", params.Encode())
}
