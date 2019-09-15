package goji

import (
	"context"
	"net/http"
	"testing"
)

func TestMuxHandlerInterface(t *testing.T) {
	var _ http.Handler = &Mux{}
}

func TestMuxExistingPath(t *testing.T) {
	m := New()
	m.HandleFunc(boolMatcher(true), func(res http.ResponseWriter, req *http.Request) {
		if path := req.Context().Value(pathKey).(string); path != "/" {
			t.Errorf("expected path=/, got %q", path)
		}
	})
	res, req := resreq()
	m.ServeHTTP(res, req.WithContext(context.WithValue(context.Background(), pathKey, "/hello")))
}

func TestSubMuxExistingPath(t *testing.T) {
	m := NewSubMux()
	m.HandleFunc(boolMatcher(true), func(res http.ResponseWriter, req *http.Request) {
		if path := req.Context().Value(pathKey).(string); path != "/hello" {
			t.Errorf("expected path=/hello, got %q", path)
		}
	})
	res, req := resreq()
	m.ServeHTTP(res, req.WithContext(context.WithValue(context.Background(), pathKey, "/hello")))
}

func TestMiddleware(t *testing.T) {
	m := New()
	ch := make(chan string, 10)
	m.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ch <- "before one"
			h.ServeHTTP(res, req)
			ch <- "after one"
		})
	})
	m.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ch <- "before two"
			h.ServeHTTP(res, req)
			ch <- "after two"
		})
	})
	m.Handle(boolMatcher(true), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		ch <- "handler"
	}))
	m.ServeHTTP(resreq())
	expectSequence(t, ch, "before one", "before two", "handler", "after two", "after one")
}

func TestMiddlewareReconfigure(t *testing.T) {
	m := New()
	ch := make(chan string, 10)
	m.Use(makeMiddleware(ch, "one"))
	m.Use(makeMiddleware(ch, "two"))
	m.Handle(boolMatcher(true), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		ch <- "handler"
	}))

	m.ServeHTTP(resreq())
	expectSequence(t, ch, "before one", "before two", "handler", "after two", "after one")
	m.Use(makeMiddleware(ch, "three"))
	m.ServeHTTP(resreq())
	expectSequence(t, ch, "before one", "before two", "before three", "handler", "after three", "after two", "after one")
}

func TestHandle(t *testing.T) {
	m := New()
	var called bool
	m.Handle(boolMatcher(true), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	m.ServeHTTP(resreq())
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestHandleFunc(t *testing.T) {
	m := New()
	var called bool
	m.HandleFunc(boolMatcher(true), func(http.ResponseWriter, *http.Request) {
		called = true
	})
	m.ServeHTTP(resreq())
	if !called {
		t.Error("expected handler to be called")
	}
}

func TestNotFoundHandler(t *testing.T) {
	var h http.Handler = New()
	res, req := resreq()
	h.ServeHTTP(res, req)
	if res.Code != 404 {
		t.Errorf("status: expected %d, got %d", 404, res.Code)
	}

	var h2 http.Handler = New(NotFoundFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(123)
	}))

	res, req = resreq()
	h2.ServeHTTP(res, req)
	if res.Code != 123 {
		t.Errorf("status: expected %d, got %d", 123, res.Code)
	}
}

func expectSequence(t *testing.T, ch chan string, seq ...string) {
	for i, str := range seq {
		if msg := <-ch; msg != str {
			t.Errorf("[%d] expected %s, got %s", i, str, msg)
		}
	}
}

func makeMiddleware(ch chan string, name string) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ch <- "before " + name
			h.ServeHTTP(res, req)
			ch <- "after " + name
		})
	}
}
