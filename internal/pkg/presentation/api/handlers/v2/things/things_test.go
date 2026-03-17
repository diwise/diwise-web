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

func TestNormalizeTypeFilterSplitsCommaSeparatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("type=Container-Sandstorage%2CSewer-CombinedSewerOverflow&tags=a")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"Container-Sandstorage", "Sewer-CombinedSewerOverflow"}, selected)
	is.Equal("tags=a&type=Container-Sandstorage&type=Sewer-CombinedSewerOverflow", params.Encode())
}

func TestNormalizeTypeFilterPreservesRepeatedValues(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("type=Container-Sandstorage&type=Sewer-CombinedSewerOverflow&type=Container-Sandstorage")
	is.NoErr(err)

	selected := normalizeTypeFilter(params)

	is.Equal([]string{"Container-Sandstorage", "Sewer-CombinedSewerOverflow"}, selected)
	is.Equal("type=Container-Sandstorage&type=Sewer-CombinedSewerOverflow", params.Encode())
}

func TestNormalizeMultiValueFilterSplitsCommaSeparatedTags(t *testing.T) {
	is := is.New(t)

	params, err := url.ParseQuery("tags=sandficka%2Calno&tags=sandficka")
	is.NoErr(err)

	selected := normalizeMultiValueFilter(params, "tags")

	is.Equal([]string{"sandficka", "alno"}, selected)
	is.Equal("tags=sandficka&tags=alno", params.Encode())
}
