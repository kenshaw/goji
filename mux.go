package goji

import (
	"context"
	"net/http"
)

// Mux is a HTTP multiplexer and router similar to net/http.ServeMux.
//
// Muxes multiplex traffic between many http.Handlers by selecting the first
// route registered to a supplied Matcher.
//
// They then call a common middleware stack, finally passing
// control to the selected http.Handler. See the documentation on the Handle
// function for more information about how routing is performed, the documentation
// on the Pattern type for more information about request matching, and the
// documentation for the Use method for more about middleware.
//
// Muxes cannot be configured concurrently from multiple goroutines, nor can they
// be configured concurrently with requests.
type Mux struct {
	router     Router
	handler    http.Handler
	middleware []func(http.Handler) http.Handler
	notFound   http.Handler
	sub        bool
}

// New returns a new Mux with no configured middleware using the default
// router.
func New(opts ...MuxOption) *Mux {
	m := &Mux{
		router:   new(router),
		notFound: http.HandlerFunc(http.NotFound),
	}
	for _, o := range opts {
		o(m)
	}
	m.buildChain()
	return m
}

// NewSubMux returns a new sub-Mux with no configured middleware using the
// default router.
func NewSubMux(opts ...MuxOption) *Mux {
	return New(append(opts, SubMux)...)
}

// buildChain builds the http.Handler chain to use during dispatch.
func (m *Mux) buildChain() {
	m.handler = http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if h := req.Context().Value(handlerKey); h != nil {
			h.(http.Handler).ServeHTTP(res, req)
			return
		}
		m.notFound.ServeHTTP(res, req)
	})
	for i := len(m.middleware) - 1; i >= 0; i-- {
		m.handler = m.middleware[i](m.handler)
	}
}

// Use appends a middleware to the Mux's middleware stack.
//
// Middleware are composable pieces of functionality that augment
// http.Handlers.  Common examples of middleware include request loggers,
// authentication checkers, and metrics gatherers.
//
// Middleware are evaluated in the reverse order in which they were added, but
// the resulting http.Handlers execute in "normal" order (i.e., the
// http.Handler returned by the first Middleware to be added gets called
// first).
//
// For instance, given middleware A, B, and C, added in that order, Goji will
// behave similarly to this snippet:
//
// 	augmentedHandler := A(B(C(yourHandler)))
// 	augmentedHandler.ServeHTTP(res, req)
//
// Assuming each of A, B, and C look something like this:
//
// 	func A(inner http.Handler) http.Handler {
// 		log.Print("A: called")
// 		mw := func(res http.ResponseWriter, req *http.Request) {
// 			log.Print("A: before")
// 			inner.ServeHTTP(res, req)
// 			log.Print("A: after")
// 		}
// 		return http.HandlerFunc(mw)
// 	}
//
// we'd expect to see the following in the log:
//
// 	C: called
// 	B: called
// 	A: called
// 	---
// 	A: before
// 	B: before
// 	C: before
// 	yourHandler: called
// 	C: after
// 	B: after
// 	A: after
//
// Note that augmentedHandler will called many times, producing the log output
// below the divider, while the outer middleware functions (the log output
// above the divider) will only be called a handful of times at application
// boot.
//
// Middleware in Goji is called after routing has been performed. Therefore it
// is possible to examine any routing information placed into the Request
// context by Patterns, or to view or modify the http.Handler that will be
// routed to.  Middleware authors should read the documentation for the
// "middleware" subpackage for more information about how this is done.
//
// The http.Handler returned by the given middleware must be safe for
// concurrent use by multiple goroutines. It is not safe to concurrently
// register middleware from multiple goroutines, or to register middleware
// concurrently with requests.
func (m *Mux) Use(middleware func(http.Handler) http.Handler) {
	m.middleware = append(m.middleware, middleware)
	m.buildChain()
}

// Handle adds a new route to the Mux. Requests that match the given Matcher will
// be dispatched to the given http.Handler.
//
// Routing is performed in the order in which routes are added: the first route
// with a matching Matcher will be used. In particular, Goji guarantees that
// routing is performed in a manner that is indistinguishable from the following
// algorithm:
//
// 	// Assume routes is a slice that every call to Handle appends to
// 	for _, route := range routes {
// 		// For performance, Matchers can opt out of this call to Match.
// 		// See the documentation for Matcher for more.
// 		if req2 := route.pattern.Match(req); req2 != nil {
// 			route.handler.ServeHTTP(res, req2)
// 			break
// 		}
// 	}
//
// It is not safe to concurrently register routes from multiple goroutines, or to
// register routes concurrently with requests.
func (m *Mux) Handle(matcher Matcher, handler http.Handler) {
	m.router.Handle(matcher, handler)
}

// HandleFunc adds a new route to the Mux. It is equivalent to calling Handle on a
// handler wrapped with http.HandlerFunc, and is provided only for convenience.
func (m *Mux) HandleFunc(matcher Matcher, handler func(http.ResponseWriter, *http.Request)) {
	m.Handle(matcher, http.HandlerFunc(handler))
}

// ServeHTTP satisfies the http.Handler interface.
func (m *Mux) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if !m.sub {
		req = req.WithContext(context.WithValue(req.Context(), pathKey, req.URL.EscapedPath()))
	}
	m.handler.ServeHTTP(res, m.router.Route(req))
}

// MuxOption is a Mux option.
type MuxOption func(*Mux)

// SubMux is a mux option to toggle the mux a sub mux.
func SubMux(m *Mux) {
	m.sub = true
}

// NotFound is a mux option to set  not found (404) handler.
func NotFound(h http.Handler) MuxOption {
	return func(m *Mux) {
		m.notFound = h
	}
}

// NotFoundFunc is a mux option to set a not found (404) handler func.
func NotFoundFunc(f http.HandlerFunc) MuxOption {
	return func(m *Mux) {
		m.notFound = f
	}
}
