package errors

import (
	"encoding/json"
	"net/http"
)

// ProblemDetails stores details about a certain problem according to RFC7807
// See https://tools.ietf.org/html/rfc7807
type ProblemDetails interface {
	Code() int
	Type() string
	Title() string
	Detail() string
	Instance() string
	TraceID() string

	ContentType() string
	WriteResponse(w http.ResponseWriter) error
}

const (
	//ProblemReportContentType as required by https://tools.ietf.org/html/rfc7807
	ProblemReportContentType string = "application/problem+json"
)

type DetailsFunc func(p *problem) *problem

func NewProblemReport(code int, problemType string, details ...DetailsFunc) ProblemDetails {
	p := &problem{
		Code_: code,
		Type_: problemType,
	}

	for _, d := range details {
		p = d(p)
	}

	return p
}

type problem struct {
	Code_          int          `json:"status"`
	Type_          string       `json:"type"`
	Title_         string       `json:"title,omitempty"`
	Detail_        string       `json:"detail,omitempty"`
	Instance_      string       `json:"instance,omitempty"`
	TraceID_       string       `json:"transaction-id,omitempty"`
	InvalidParams_ []invalparam `json:"invalid-parameters,omitempty"`
}

func (p *problem) Code() int {
	return p.Code_
}

func (p *problem) ContentType() string {
	return ProblemReportContentType
}

func (p *problem) Type() string {
	return p.Type_
}

func (p *problem) Title() string {
	return p.Title_
}

func (p *problem) Detail() string {
	return p.Detail_
}

func (p *problem) Instance() string {
	return p.Instance_
}

func (p *problem) TraceID() string {
	return p.TraceID_
}

func (p *problem) WriteResponse(w http.ResponseWriter) error {
	bytes, _ := json.Marshal(p)

	w.Header().Add("Content-Type", p.ContentType())
	w.WriteHeader(p.Code())
	_, err := w.Write(bytes)

	return err
}

func Detail(detail string) DetailsFunc {
	return func(p *problem) *problem {
		p.Detail_ = detail
		return p
	}
}

func Instance(instance string) DetailsFunc {
	return func(p *problem) *problem {
		p.Instance_ = instance
		return p
	}
}

func InvalidParameter(property string, value any, reason string) DetailsFunc {
	return func(p *problem) *problem {
		p.InvalidParams_ = append(
			p.InvalidParams_,
			invalparam{
				Property: property,
				Value:    value,
				Reason:   reason,
			},
		)
		return p
	}
}

func Title(t string) DetailsFunc {
	return func(p *problem) *problem {
		p.Title_ = t
		return p
	}
}

func TraceID(traceID string) DetailsFunc {
	return func(p *problem) *problem {
		p.TraceID_ = traceID
		return p
	}
}

type invalparam struct {
	Property string `json:"property"`
	Value    any    `json:"value"`
	Reason   string `json:"reason"`
}
