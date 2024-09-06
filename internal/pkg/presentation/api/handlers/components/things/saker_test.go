package things

import (
	"net/url"
	"testing"

	"github.com/diwise/diwise-web/internal/pkg/presentation/api/helpers"
	"github.com/matryer/is"
)

func TestExtractArgs(t *testing.T) {
	is := is.New(t)
	params, err := url.ParseQuery("limit=5&limit=5&limit=5&limit=5&limit=5&type=wastecontainer&type=sewer&type=sewagepumpingstation&email=")
	is.NoErr(err)

	helpers.SanitizeParams(params)

	is.Equal("limit=5&type=wastecontainer&type=sewer&type=sewagepumpingstation", params.Encode())

	helpers.SanitizeParams(params, "limit")

	is.Equal("type=wastecontainer&type=sewer&type=sewagepumpingstation", params.Encode())
}
