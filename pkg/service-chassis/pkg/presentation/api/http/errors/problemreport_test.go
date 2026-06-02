package errors

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/matryer/is"
)

func TestMe(t *testing.T) {
	is := is.New(t)

	p := NewProblemReport(http.StatusOK, "mytype")
	is.Equal(p.Type(), "mytype")
}

func TestInvalidArgument(t *testing.T) {
	is := is.New(t)

	p := NewProblemReport(http.StatusBadRequest, "badrequest", InvalidParameter("param1", 129, "must be in range [0,127]"))
	b, _ := json.Marshal(p)

	const expectation string = `{"status":400,"type":"badrequest","invalid-parameters":[{"property":"param1","value":129,"reason":"must be in range [0,127]"}]}`
	is.Equal(string(b), expectation)
}
