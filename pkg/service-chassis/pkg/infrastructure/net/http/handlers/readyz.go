package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

type ServiceProber func(context.Context) (status string, err error)

func NewReadinessHandler(ctx context.Context, probes map[string]ServiceProber) http.HandlerFunc {

	// We need to build a sorted list of the probes since the iteration
	// order of a map is not strictly ordered
	allnames := make([]string, 0, len(probes))
	for probeName := range probes {
		allnames = append(allnames, probeName)
	}

	slices.Sort(allnames)

	allprobes := make([]ServiceProber, 0, len(probes))
	for _, probeName := range allnames {
		allprobes = append(allprobes, probes[probeName])
	}

	return func(w http.ResponseWriter, r *http.Request) {

		params, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		excludedChecks := params["exclude"]
		_, verbose := params["verbose"]

		var sb strings.Builder

		for idx, probeName := range allnames {
			isExcluded := false

			for _, excl := range excludedChecks {
				if strings.EqualFold(probeName, excl) {
					isExcluded = true
					break
				}
			}

			var result error
			if !isExcluded {
				_, result = allprobes[idx](ctx)
			}

			if verbose {
				sb.WriteString("[+]")
				sb.WriteString(probeName)

				if result != nil {
					sb.WriteString(" error\n")
				} else if isExcluded {
					sb.WriteString(" excluded: ok\n")
				} else {
					sb.WriteString(" ok\n")
				}
			}

			err = errors.Join(result)
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else if verbose {
			sb.WriteString("healthz check passed\n")

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(sb.String()))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

func NewSingleReadinessHandler(ctx context.Context, probes map[string]ServiceProber) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		probeName := r.PathValue("check")
		probe, ok := probes[probeName]

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		params, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, verbose := params["verbose"]

		var sb strings.Builder

		_, err = probe(ctx)

		if verbose {
			sb.WriteString("[+]")
			sb.WriteString(probeName)

			if err != nil {
				sb.WriteString(" error\n")
			} else {
				sb.WriteString(" ok\n")
			}
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		} else if verbose {
			sb.WriteString("healthz check passed\n")

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(sb.String()))
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
	}
}
