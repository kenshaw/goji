// Package goji is a minimalistic and flexible context based route matching
// request multiplexer.
package goji

import (
	"context"
	"net/http"
)

// contextKey is a the context key type.
type contextKey int

const (
	// matcherKey is the context key used for the last matched Matcher.
	matcherKey contextKey = iota

	// handlerKey is the context key used for the last matched Handler.
	handlerKey

	// pathKey is the context key used for path prefixes.
	pathKey
)

// nameKey is the context key type for names of variables extracted from URLs.
type nameKey string

// WithHandler returns a child context with the passed handler.
func WithHandler(ctx context.Context, handler http.Handler) context.Context {
	return context.WithValue(ctx, handlerKey, handler)
}

// WithMatcher returns a child context with the passed Matcher.
func WithMatcher(ctx context.Context, matcher Matcher) context.Context {
	return context.WithValue(ctx, matcherKey, matcher)
}

// WithPath returns a child context with the passed path prefix.
func WithPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, pathKey, path)
}

// Path returns the path prefix from the context.
func Path(ctx context.Context) string {
	if path := ctx.Value(pathKey); path != nil {
		return path.(string)
	}
	return ""
}

// Param returns a bound, named named variable from the context.
//
// For example, given a mux with a single GET route:
//
//	mux := goji.NewMux()
//	mux.HandleFunc(goji.Get("/user/:name"), http.HandlerFunc(/* ... */)
//
// When a HTTP request for is issued:
//
//	GET /user/carl
//
// Then a call to goji.Param(req.Context(), "name") will return "carl".
//
// Note: caller should ensure that the variable has been bound. Attempts to
// access variables that have not been set (or which have been invalidly set)
// are considered programmer errors and will trigger a panic.
func Param(req *http.Request, name string) string {
	return req.Context().Value(nameKey(name)).(string)
}
