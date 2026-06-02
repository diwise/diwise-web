package router

import (
	"iter"
	"net/http"
	"slices"
	"strings"
)

type ServeMux interface {
	// Group creates a new mux, calls fn with this mux and wraps it after return with the current stack of middleware
	Group(fn func(ServeMux))

	// Route adds the route argument to the current pattern prefix and calls fn with a copy of
	// the current ServeMux. Any middleware that is added within fn will not affect any other routes.
	Route(route string, fn func(ServeMux))

	// Add one or more middleware handlers to the stack of the current router. The middlewares will be invoked in the order they are passed in.
	Use(middlewares ...func(http.Handler) http.Handler)

	// Register a CONNECT handler h with the given pattern added to the current prefix
	Connect(pattern string, h http.HandlerFunc)
	// Register a DELETE handler h with the given pattern added to the current prefix
	Delete(pattern string, h http.HandlerFunc)
	// Register a GET handler h with the given pattern added to the current prefix
	Get(pattern string, h http.HandlerFunc)
	// Register a HEAD handler h with the given pattern added to the current prefix
	Head(pattern string, h http.HandlerFunc)
	// Register a OPTIONS handler h with the given pattern added to the current prefix
	Options(pattern string, h http.HandlerFunc)
	// Register a PATCH handler h with the given pattern added to the current prefix
	Patch(pattern string, h http.HandlerFunc)
	// Register a POST handler h with the given pattern added to the current prefix
	Post(pattern string, h http.HandlerFunc)
	// Register a PUT handler h with the given pattern added to the current prefix
	Put(pattern string, h http.HandlerFunc)
	// Register a TRACE handler h with the given pattern added to the current prefix
	Trace(pattern string, h http.HandlerFunc)

	// AllowedMethods returns an iterator over the methods that have been used when registering handlers with this ServeMux.
	AllowedMethods() iter.Seq[string]
}

type opt struct {
	prefix string
}

func WithPrefix(prefix string) func(*opt) {
	return func(o *opt) {
		o.prefix = prefix
	}
}

// Deprecated: spans are automatically annotated with the route attribute.
func WithTaggedRoutes(_ bool) func(*opt) {
	return func(_ *opt) {}
}

func New(mux *http.ServeMux, options ...func(*opt)) ServeMux {
	o := &opt{}

	for _, applyOption := range options {
		applyOption(o)
	}

	return &impl{
		prefix:         o.prefix,
		mux:            mux,
		middlewares:    make([]func(http.Handler) http.Handler, 0, 16),
		allowedMethods: map[string]struct{}{},
		handlers:       map[string]http.Handler{},
		patterns:       map[string]struct{}{},
	}
}

type impl struct {
	prefix      string
	mux         *http.ServeMux
	middlewares []func(http.Handler) http.Handler

	allowedMethods map[string]struct{}
	handlers       map[string]http.Handler
	patterns       map[string]struct{}
}

func joinPath(prefix, path string) string {
	if prefix == "" {
		prefix = "/"
	}

	if path == "" {
		return prefix
	}

	if prefix == "/" {
		return "/" + strings.TrimPrefix(path, "/")
	}

	if strings.HasSuffix(prefix, "/") {
		return prefix + strings.TrimPrefix(path, "/")
	}

	if strings.HasPrefix(path, "/") {
		return prefix + path
	}

	return prefix + "/" + path
}

func exactPattern(path string) string {
	if path == "" || path == "/" {
		return "/{$}"
	}

	if strings.HasSuffix(path, "/") {
		return path + "{$}"
	}

	return path
}

func subtreePattern(path string) string {
	if path == "" || path == "/" {
		return "/"
	}

	if strings.HasSuffix(path, "/") {
		return path
	}

	return path + "/"
}

func (i *impl) register(method, pattern string, h http.Handler) {
	fullPattern := exactPattern(joinPath(i.prefix, pattern))
	wrappedHandler := i.wrap(h)

	i.allowedMethods[method] = struct{}{}
	i.handlers[method+" "+fullPattern] = wrappedHandler
	i.patterns[method+" "+fullPattern] = struct{}{}
	i.mux.Handle(method+" "+fullPattern, wrappedHandler)
}

func (i *impl) wrap(h http.Handler) http.Handler {
	handler := h
	for _, mw := range slices.Backward(i.middlewares) {
		handler = mw(handler)
	}
	return handler
}

func (i *impl) AllowedMethods() iter.Seq[string] {
	return func(yield func(string) bool) {
		for method := range i.allowedMethods {
			if !yield(method) {
				return
			}
		}
	}
}

// Connect implements [ServeMux]
func (i *impl) Connect(pattern string, h http.HandlerFunc) {
	i.register(http.MethodConnect, pattern, h)
}

func (i *impl) Delete(pattern string, h http.HandlerFunc) {
	i.register(http.MethodDelete, pattern, h)
}

func (i *impl) Get(pattern string, h http.HandlerFunc) {
	i.register(http.MethodGet, pattern, h)
}

func (i *impl) Group(fn func(ServeMux)) {
	groupMux := http.NewServeMux()
	groupRouter := New(groupMux, WithPrefix(i.prefix)).(*impl)

	fn(groupRouter)

	registerSynthetic := func(method, pattern string, h http.Handler) {
		wrappedHandler := i.wrap(h)
		key := method + " " + pattern

		i.allowedMethods[method] = struct{}{}
		i.handlers[key] = wrappedHandler
		i.patterns[key] = struct{}{}
		i.mux.Handle(key, wrappedHandler)
	}

	handler := http.Handler(groupMux)
	subtree := subtreePattern(i.prefix)
	exact := exactPattern(i.prefix)
	slashExact := exactPattern(subtree)

	for m := range groupRouter.AllowedMethods() {
		registerSynthetic(m, subtree, handler)
		if _, ok := groupRouter.patterns[m+" "+exact]; ok {
			registerSynthetic(m, exact, handler)
		}
		if slashHandler, ok := groupRouter.handlers[m+" "+slashExact]; ok {
			registerSynthetic(m, slashExact, slashHandler)
			continue
		}
		if exactHandler, ok := groupRouter.handlers[m+" "+exact]; ok {
			registerSynthetic(m, slashExact, exactHandler)
		}
	}
}

func (i *impl) Head(pattern string, h http.HandlerFunc) {
	i.register(http.MethodHead, pattern, h)
}

func (i *impl) Options(pattern string, h http.HandlerFunc) {
	i.register(http.MethodOptions, pattern, h)
}

func (i *impl) Patch(pattern string, h http.HandlerFunc) {
	i.register(http.MethodPatch, pattern, h)
}

func (i *impl) Post(pattern string, h http.HandlerFunc) {
	i.register(http.MethodPost, pattern, h)
}

func (i *impl) Put(pattern string, h http.HandlerFunc) {
	i.register(http.MethodPut, pattern, h)
}

func (i *impl) Route(route string, r func(ServeMux)) {
	copy := *i
	copy.allowedMethods = map[string]struct{}{}
	copy.handlers = map[string]http.Handler{}
	copy.patterns = map[string]struct{}{}
	copy.prefix = joinPath(copy.prefix, route)

	r(&copy)

	for m := range copy.AllowedMethods() {
		i.allowedMethods[m] = struct{}{}
	}
}

func (i *impl) Trace(pattern string, h http.HandlerFunc) {
	i.register(http.MethodTrace, pattern, h)
}

func (i *impl) Use(middlewares ...func(http.Handler) http.Handler) {
	i.middlewares = append(i.middlewares, middlewares...)
}
