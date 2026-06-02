package http

import (
	"net/http"
	"testing"

	"github.com/diwise/service-chassis/pkg/test/http/expects"
	"github.com/diwise/service-chassis/pkg/test/http/response"
	"github.com/matryer/is"
)

var method = expects.RequestMethod
var path = expects.RequestPath

func TestMockService(t *testing.T) {
	is := is.New(t)

	s := NewMockServiceThat(
		Expects(is,
			method(http.MethodGet),
			path("/hello"),
		),
		Returns(
			response.ContentType("application/ld+json"),
			response.Code(http.StatusCreated),
		),
	)
	defer s.Close()

	resp, err := http.Get(s.URL() + "/hello")

	is.NoErr(err)
	is.Equal(resp.StatusCode, http.StatusCreated)
	is.Equal(s.RequestCount(), 1)
}
