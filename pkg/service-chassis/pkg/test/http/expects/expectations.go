package expects

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/matryer/is"
)

func AnyInput() func(*is.I, *http.Request) {
	return func(*is.I, *http.Request) {}
}

func RequestBody(body string) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		reqBytes, err := io.ReadAll(r.Body)
		is.NoErr(err)

		reqString := string(reqBytes)
		is.Equal(reqString, body)
	}
}

func RequestBodyContaining(substrings ...string) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		reqBytes, err := io.ReadAll(r.Body)
		is.NoErr(err)

		reqString := string(reqBytes)

		for _, subs := range substrings {
			if !strings.Contains(reqString, subs) {
				is.Equal("expectation", fmt.Sprintf("request body does not contain %s", subs))
			}
		}
	}
}

func RequestBodyIsForm(match func(*is.I, url.Values)) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		reqBytes, err := io.ReadAll(r.Body)
		is.NoErr(err)
		defer r.Body.Close()

		v, err := url.ParseQuery(string(reqBytes))
		is.NoErr(err)

		match(is, v)
	}
}

func RequestBodyOfType[T any](match func(*is.I, T)) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		reqBytes, err := io.ReadAll(r.Body)
		is.NoErr(err)
		defer r.Body.Close()

		var s T
		err = json.Unmarshal(reqBytes, &s)
		is.NoErr(err)

		match(is, s)
	}
}

func RequestHeaderContains(key, expectation string) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		allValues, headerExists := r.Header[http.CanonicalHeaderKey(key)]
		if !headerExists {
			is.Equal(
				"expectation",
				fmt.Sprintf("request does not contain expected header %s", key),
			)
		}

		for _, headerValue := range allValues {
			if strings.EqualFold(headerValue, expectation) {
				return
			}
		}

		is.Equal(
			"expectation",
			fmt.Sprintf("request header %s does not contain value %s", key, expectation),
		)
	}
}

func RequestMethod(method string) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		is.Equal(r.Method, method)
	}
}

func RequestPath(path string) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		is.Equal(r.URL.Path, path)
	}
}

func QueryParamContains(name, value string) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		is.True(r.URL.Query().Has(name)) // query param should exist

		for _, v := range strings.Split(r.URL.Query().Get(name), ",") {
			if v == value {
				return // it is a match!
			}
		}

		is.Fail() // query params did not contain expected value
	}
}

func QueryParamEquals(name, value string) func(*is.I, *http.Request) {
	return func(is *is.I, r *http.Request) {
		is.True(r.URL.Query().Has(name))         // query param should exist
		is.Equal(r.URL.Query().Get(name), value) // query param should match
	}
}
