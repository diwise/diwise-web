package servicerunner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/handlers"
)

type runOpts[T any] struct {
	worker func(context.Context, *T) error

	terminationTimeout time.Duration
}

type Runner[T any] interface {
	Run(ctx context.Context, options ...func(*runOpts[T])) error
}

type muxCfg[T any] struct {
	name   string
	port   string
	listen string

	k8sProbesEnabled bool
	probers          map[string]handlers.ServiceProber
	isAlive          func() error

	pprofEnabled   bool
	tracingEnabled bool

	initFunc func(ctx context.Context, identifier, port string, svcCfg *T, handler *http.ServeMux) error
}

type runnerCfg[T any] struct {
	muxes []*muxCfg[T]

	onInit              func(context.Context, *T) error
	initHookTimeout     time.Duration
	onStarting          func(context.Context, *T) error
	startingHookTimeout time.Duration
	onRunning           func(context.Context, *T) error
	runningHookTimeout  time.Duration
	onShutdown          func(context.Context, *T) error
	shutdownHookTimeout time.Duration
}

func IfNot[T any](exclude bool, opt func(*runnerCfg[T])) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		if !exclude {
			opt(cfg)
		}
	}
}

func OnInit[T any](initFunc func(context.Context, *T) error) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.onInit = initFunc
	}
}

func WithInitTimeout[T any](timeout time.Duration) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.initHookTimeout = timeout
	}
}

func OnStarting[T any](startupFunc func(context.Context, *T) error) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.onStarting = startupFunc
	}
}

func WithStartingTimeout[T any](tmo time.Duration) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.startingHookTimeout = tmo
	}
}

func OnRunning[T any](runningFunc func(context.Context, *T) error) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.onRunning = runningFunc
	}
}

func WithRunningTimeout[T any](tmo time.Duration) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.runningHookTimeout = tmo
	}
}

func OnShutdown[T any](shutdownFunc func(context.Context, *T) error) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.onShutdown = shutdownFunc
	}
}

func WithShutdownTimeout[T any](tmo time.Duration) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		cfg.shutdownHookTimeout = tmo
	}
}

func WithListenAddr[T any](listen string) func(*muxCfg[T]) {
	return func(cfg *muxCfg[T]) {
		cfg.listen = listen
	}
}

func WithPort[T any](port string) func(*muxCfg[T]) {
	return func(cfg *muxCfg[T]) {
		cfg.port = port
	}
}

func WithK8SLivenessProbe[T any](isAlive func() error) func(*muxCfg[T]) {
	return func(cfg *muxCfg[T]) {
		cfg.k8sProbesEnabled = true
		cfg.isAlive = isAlive
	}
}

func WithK8SReadinessProbes[T any](probers map[string]handlers.ServiceProber) func(*muxCfg[T]) {
	return func(cfg *muxCfg[T]) {
		cfg.k8sProbesEnabled = true
		cfg.probers = map[string]handlers.ServiceProber{}

		for name, prober := range probers {
			cfg.probers[strings.ToLower(name)] = prober
		}
	}
}

func WithPPROF[T any]() func(*muxCfg[T]) {
	return func(cfg *muxCfg[T]) {
		cfg.pprofEnabled = true
	}
}

func WithTracing[T any](enabled bool) func(*muxCfg[T]) {
	return func(cfg *muxCfg[T]) {
		cfg.tracingEnabled = enabled
	}
}

func OnMuxInit[T any](initFunc func(ctx context.Context, identifier, port string, svcCfg *T, handler *http.ServeMux) error) func(*muxCfg[T]) {
	return func(cfg *muxCfg[T]) {
		cfg.initFunc = initFunc
	}
}

func WithHTTPServeMux[T any](identifer string, opts ...func(*muxCfg[T])) func(*runnerCfg[T]) {
	return func(cfg *runnerCfg[T]) {
		mcfg := &muxCfg[T]{
			name:             identifer,
			listen:           "127.0.0.1",
			port:             "0",
			pprofEnabled:     false,
			tracingEnabled:   false,
			k8sProbesEnabled: false,
			isAlive:          func() error { return nil },
			initFunc: func(ctx context.Context, identifier, port string, svcCfg *T, handler *http.ServeMux) error {
				return nil
			},
		}

		for _, option := range opts {
			option(mcfg)
		}

		cfg.muxes = append(cfg.muxes, mcfg)
	}
}

type httpSrvr struct {
	listener net.Listener
	mux      *http.ServeMux
	server   *http.Server
}

type runner[T any] struct {
	cfg    *runnerCfg[T]
	svcCfg *T

	httpServers []*httpSrvr

	configError error
}

func doHook[T any](ctx context.Context, hook func(context.Context, *T) error, svcCfg *T, timeout time.Duration) (err error) {
	hookResult := make(chan error)
	hookContext, hookDone := context.WithTimeout(ctx, timeout)
	defer hookDone()

	go func() {
		defer close(hookResult)
		defer func() {
			if r := recover(); r != nil {
				hookResult <- fmt.Errorf("service runner hook paniced: %v", r)
			}
		}()

		hookResult <- hook(ctx, svcCfg)
	}()

	select {
	case <-hookContext.Done():
		if errors.Is(context.Cause(hookContext), context.DeadlineExceeded) {
			err = fmt.Errorf("hook did not return within the maximum allowed %s", timeout.String())
		}
	case err = <-hookResult:
	}

	return
}

func WithWorker[T any](worker func(context.Context, *T) error) func(*runOpts[T]) {
	return func(opts *runOpts[T]) {
		opts.worker = worker
	}
}

func WithTerminationTimeout[T any](tmo time.Duration) func(*runOpts[T]) {
	return func(opts *runOpts[T]) {
		opts.terminationTimeout = tmo
	}
}

func (r *runner[T]) Run(ctx context.Context, opts ...func(*runOpts[T])) (err error) {

	runOptions := runOpts[T]{
		terminationTimeout: 10 * time.Second,
	}

	for _, option := range opts {
		if option == nil {
			return fmt.Errorf("nil run options not allowed")
		}

		option(&runOptions)
	}

	if r.configError != nil {
		return r.configError
	}

	err = doHook(ctx, r.cfg.onStarting, r.svcCfg, r.cfg.startingHookTimeout)
	if err != nil {
		return
	}

	errChan := make(chan error, len(r.httpServers)+1)
	defer close(errChan)

	var wg sync.WaitGroup

	for serverIndex := range r.httpServers {
		wg.Add(1)

		go func(s *httpSrvr, errc chan<- error) {
			defer wg.Done()

			if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
				errc <- fmt.Errorf(
					"failed to start request router on address %s: %s", s.server.Addr, err.Error(),
				)
			}
		}(r.httpServers[serverIndex], errChan)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer close(sigChan)

	err = doHook(ctx, r.cfg.onRunning, r.svcCfg, r.cfg.runningHookTimeout)
	if err != nil {
		return
	}

	if runOptions.worker != nil && context.Cause(ctx) == nil {

		wg.Go(func() {

			workerError := runOptions.worker(ctx, r.svcCfg)
			if workerError != nil {
				errChan <- workerError
			}
		})
	}

	select {
	case err = <-errChan:
	case <-ctx.Done():
	case <-sigChan:
	}

	err = errors.Join(
		err,
		doHook(context.WithoutCancel(ctx), r.cfg.onShutdown, r.svcCfg, r.cfg.shutdownHookTimeout),
	)

	for serverIndex := range r.httpServers {
		e := r.httpServers[serverIndex].server.Shutdown(ctx)
		if e != nil && !errors.Is(e, context.Canceled) {
			err = errors.Join(err, fmt.Errorf("failed to shutdown web server: %s", e.Error()))
		}
	}

	theWaitIsOver := make(chan struct{})
	go func() {
		defer close(theWaitIsOver)
		wg.Wait()
	}()

	select {
	case <-theWaitIsOver:
	case <-time.After(runOptions.terminationTimeout):
		err = fmt.Errorf("timed out waiting for shutdown")
	}

	return
}

func New[T any](ctx context.Context, svcCfg T, opts ...func(cfg *runnerCfg[T])) (context.Context, Runner[T]) {

	noop := func(context.Context, *T) error { return nil }

	cfg := &runnerCfg[T]{
		muxes: make([]*muxCfg[T], 0, 2),

		onInit:              noop,
		initHookTimeout:     30 * time.Second,
		onStarting:          noop,
		startingHookTimeout: 30 * time.Second,
		onRunning:           noop,
		runningHookTimeout:  30 * time.Second,
		onShutdown:          noop,
		shutdownHookTimeout: 30 * time.Second,
	}

	for _, option := range opts {
		option(cfg)
	}

	r := &runner[T]{
		cfg:         cfg,
		svcCfg:      &svcCfg,
		httpServers: make([]*httpSrvr, 0, len(cfg.muxes)),
	}

	r.configError = doHook(ctx, r.cfg.onInit, r.svcCfg, r.cfg.initHookTimeout)

	for _, muxConf := range r.cfg.muxes {
		mux := http.NewServeMux()

		if muxConf.pprofEnabled {
			mux.HandleFunc("GET /debug/pprof/", pprof.Index)
			mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
			mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
			mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
		}

		if muxConf.k8sProbesEnabled {
			nhh := handlers.NewHealthHandler(ctx, muxConf.probers)
			mux.HandleFunc("GET /health", nhh)
			mux.HandleFunc("GET /healthz", nhh)

			mux.HandleFunc("GET /livez", handlers.NewLivenessHandler(ctx, muxConf.isAlive))
			mux.HandleFunc("GET /readyz", handlers.NewReadinessHandler(ctx, muxConf.probers))
			mux.HandleFunc("GET /readyz/{check}", handlers.NewSingleReadinessHandler(ctx, muxConf.probers))
		}

		addr := muxConf.listen + ":" + muxConf.port
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			r.configError = fmt.Errorf("failed to listen: %s", err.Error())
			return ctx, r
		}

		_, port, _ := net.SplitHostPort(listener.Addr().String())

		err = muxConf.initFunc(ctx, muxConf.name, port, &svcCfg, mux)
		if err != nil {
			r.configError = fmt.Errorf("failed to init servemux: %s", err.Error())
			listener.Close()
			return ctx, r
		}

		s := &httpSrvr{
			listener: listener,
			mux:      mux,
			server:   &http.Server{Addr: addr, Handler: mux},
		}

		if muxConf.tracingEnabled {
			s.server.Handler = otelhttp.NewHandler(mux, muxConf.name,
				otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
			)
		}

		r.httpServers = append(r.httpServers, s)
	}

	return ctx, r
}
