package servicerunner_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/handlers"
	"github.com/diwise/service-chassis/pkg/infrastructure/servicerunner"
	"github.com/matryer/is"
)

type cfg struct{}

var OnStarting = servicerunner.OnStarting[cfg]
var OnRunning = servicerunner.OnRunning[cfg]
var OnShutdown = servicerunner.OnShutdown[cfg]

var WithHTTPServeMux = servicerunner.WithHTTPServeMux[cfg]
var WithPort = servicerunner.WithPort[cfg]
var WithK8SLivenessProbe = servicerunner.WithK8SLivenessProbe[cfg]
var WithK8SReadinessProbes = servicerunner.WithK8SReadinessProbes[cfg]
var OnMuxInit = servicerunner.OnMuxInit[cfg]

var WithWorker = servicerunner.WithWorker[cfg]
var WithStartingTimeout = servicerunner.WithStartingTimeout[cfg]

var ignored int

func TestStartingAndStopping(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)

	ctx, r := servicerunner.New(ctx, cfg{}, OnRunning(stopRunner))

	is.NoErr(r.Run(ctx))
}

func TestPanicHook(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)

	ctx, r := servicerunner.New(ctx,
		cfg{},
		OnStarting(func(_ context.Context, _ *cfg) error {
			panic("WEAREGONNADIE!!!")
		}),
		OnRunning(stopRunner))

	err := r.Run(ctx)
	is.Equal(err.Error(), "service runner hook paniced: WEAREGONNADIE!!!")
}

func TestStartupAndShutdownHooksAreCalled(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	var startupCalled atomic.Bool
	var shutdownCalled atomic.Bool

	ctx, r := servicerunner.New(ctx,
		cfg{},
		OnRunning(stopRunner),
		OnStarting(func(_ context.Context, _ *cfg) error {
			startupCalled.Store(true)
			return nil
		}),
		OnShutdown(func(_ context.Context, _ *cfg) error {
			shutdownCalled.Store(true)
			return nil
		}),
	)

	is.NoErr(r.Run(ctx))
	is.True(startupCalled.Load())  // startup hook should have been called
	is.True(shutdownCalled.Load()) // shutdown hook should have been called
}

func TestPresentsIdentifierOnMuxInit(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	const expectedMuxName string = "therearemanymuxeslikeitbutthisoneismine"

	ctx, r := servicerunner.New(ctx,
		cfg{},
		OnRunning(stopRunner),
		WithHTTPServeMux(
			expectedMuxName,
			OnMuxInit(func(_ context.Context, identifier string, _ string, _ *cfg, _ *http.ServeMux) error {
				is.Equal(expectedMuxName, identifier)
				return nil
			}),
		),
	)

	is.NoErr(r.Run(ctx))
}

func TestK8SLivenessProbe(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	var serverPort string

	ctx, r := servicerunner.New(ctx,
		cfg{},
		WithHTTPServeMux(
			"test",
			WithK8SLivenessProbe(func() error { return nil }),
			OnMuxInit(saveServerPort(&serverPort)),
		),
		OnRunning(func(ctx context.Context, tc *cfg) (err error) {
			defer stopRunner(ctx, tc)

			response, _ := httpRequest(http.MethodGet, serverPort, "/health", nil)
			is.Equal(response.StatusCode, http.StatusNoContent)

			return
		}),
	)

	is.NoErr(r.Run(ctx))
}

func TestK8SReadinessProbes(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	var serverPort string

	var tsProbeCount int
	var rmqProbeCount int

	tsProbe := probe(&tsProbeCount, nil)
	rmqProbe := probe(&rmqProbeCount, nil)

	probes := map[string]handlers.ServiceProber{
		"timescale": tsProbe,
		"rabbit":    rmqProbe,
	}

	ctx, r := servicerunner.New(ctx,
		cfg{},
		WithHTTPServeMux(
			"test",
			OnMuxInit(saveServerPort(&serverPort)),
			WithK8SReadinessProbes(probes),
		),
	)

	err := r.Run(ctx, WithWorker(func(ctx context.Context, tc *cfg) error {
		defer stopRunner(ctx, tc)

		response, _ := httpRequest(http.MethodGet, serverPort, "/readyz", nil)
		is.Equal(response.StatusCode, http.StatusNoContent)

		return nil
	}))

	is.NoErr(err)
	is.Equal(tsProbeCount, 1)
	is.Equal(rmqProbeCount, 1)
}

func TestK8SReadinessProbesWithVerboseOutput(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	var serverPort string

	probes := map[string]handlers.ServiceProber{"timescale": probe(&ignored, nil), "rabbit": probe(&ignored, nil)}

	ctx, r := servicerunner.New(ctx,
		cfg{},
		WithHTTPServeMux(
			"test",
			OnMuxInit(saveServerPort(&serverPort)),
			WithK8SReadinessProbes(probes),
		),
	)

	err := r.Run(ctx, WithWorker(func(ctx context.Context, tc *cfg) error {
		defer stopRunner(ctx, tc)

		response, body := httpRequest(http.MethodGet, serverPort, "/readyz?verbose", nil)
		is.Equal(response.StatusCode, http.StatusOK)
		is.Equal("[+]rabbit ok\n[+]timescale ok\nhealthz check passed\n", string(body))

		return nil
	}))

	is.NoErr(err)
}

func TestK8SReadinessProbesWithVerboseOutputAndExcludedCheck(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	var serverPort string

	probes := map[string]handlers.ServiceProber{"timescale": probe(&ignored, nil), "rabbit": probe(&ignored, nil)}

	ctx, r := servicerunner.New(ctx,
		cfg{},
		WithHTTPServeMux(
			"test",
			OnMuxInit(saveServerPort(&serverPort)),
			WithK8SReadinessProbes(probes),
		),
	)

	err := r.Run(ctx, WithWorker(func(ctx context.Context, tc *cfg) error {
		defer stopRunner(ctx, tc)

		response, body := httpRequest(http.MethodGet, serverPort, "/readyz?verbose&exclude=rabbit", nil)
		is.Equal(response.StatusCode, http.StatusOK)
		is.Equal("[+]rabbit excluded: ok\n[+]timescale ok\nhealthz check passed\n", string(body))

		return nil
	}))

	is.NoErr(err)
}

func TestK8SReadinessProbesWithVerboseOutputAndSingleCheck(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	var serverPort string

	probes := map[string]handlers.ServiceProber{"timescale": probe(&ignored, nil), "rabbit": probe(&ignored, nil)}

	ctx, r := servicerunner.New(ctx,
		cfg{},
		WithHTTPServeMux(
			"test",
			OnMuxInit(saveServerPort(&serverPort)),
			WithK8SReadinessProbes(probes),
		),
	)

	err := r.Run(ctx, WithWorker(func(ctx context.Context, tc *cfg) error {
		defer stopRunner(ctx, tc)

		response, body := httpRequest(http.MethodGet, serverPort, "/readyz/timescale?verbose", nil)
		is.Equal(response.StatusCode, http.StatusOK)
		is.Equal("[+]timescale ok\nhealthz check passed\n", string(body))

		return nil
	}))

	is.NoErr(err)
}

func TestFailsIfMoreThanOneMuxSharesPort(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)

	ctx, r := servicerunner.New(ctx,
		cfg{},
		OnRunning(stopRunner),
		WithHTTPServeMux("test1", WithPort("8000")),
		WithHTTPServeMux("test2", WithPort("8000")),
	)

	err := r.Run(ctx)

	is.Equal(err.Error(), "failed to listen: listen tcp 127.0.0.1:8000: bind: address already in use")
}

func TestStopsIfWorkerSignalsAnError(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)
	expectedError := errors.New("thisiswronginsomanyways!!!")
	defer stopRunner(ctx, nil)

	ctx, r := servicerunner.New(ctx, cfg{})
	worker := func(context.Context, *cfg) error {
		return expectedError
	}

	err := r.Run(ctx, WithWorker(worker))

	is.True(err != nil)
	is.Equal(expectedError.Error(), err.Error())
}

func TestCancelsWorkerContextIfParentContextIsCanceled(t *testing.T) {
	is, ctx, stopRunner := setupTest(t)

	ctx, r := servicerunner.New(ctx, cfg{})
	worker := func(ctx context.Context, tc *cfg) error {
		go stopRunner(ctx, tc)
		<-ctx.Done()
		return nil
	}

	err := r.Run(ctx, WithWorker(worker))

	is.NoErr(err)
}

func TestStartingHookMustNotBlock(t *testing.T) {
	is, ctx, _ := setupTest(t)

	ctx, r := servicerunner.New(ctx,
		cfg{},
		WithStartingTimeout(10*time.Millisecond),
		OnStarting(func(context.Context, *cfg) error {
			time.Sleep(30 * time.Second)
			return nil
		}),
	)

	err := r.Run(ctx)

	is.Equal(err.Error(), "hook did not return within the maximum allowed 10ms")
}

func saveServerPort(here *string) func(context.Context, string, string, *cfg, *http.ServeMux) error {
	return func(_ context.Context, _ string, port string, _ *cfg, _ *http.ServeMux) error {
		*here = port
		return nil
	}
}

func setupTest(t *testing.T) (*is.I, context.Context, func(context.Context, *cfg) error) {
	const testTimeout time.Duration = (5 * time.Second)

	ctx, cancelFunc := context.WithTimeout(context.Background(), testTimeout)

	runnerStopper := func(context.Context, *cfg) error {
		cancelFunc()
		return nil
	}

	return is.New(t), ctx, runnerStopper
}

func httpRequest(method, port, path string, body io.Reader) (*http.Response, string) {
	req, _ := http.NewRequest(method, "http://localhost:"+port+path, body)

	resp, _ := http.DefaultClient.Do(req)
	respBody, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	return resp, string(respBody)
}

func probe(count *int, result error) func(context.Context) (string, error) {
	return func(context.Context) (string, error) {
		(*count)++
		return "ok", result
	}
}
