package goji

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestMatchContextInterface(t *testing.T) {
	var _ context.Context = match{}
}

func TestNoMatch(t *testing.T) {
	for _, r := range []Router{&router{}, &simpleRouter{}} {
		r.Handle(boolMatcher(false), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("did not expect handler to be called")
		}))
		_, req := resreq()
		ctx := context.Background()
		ctx = context.WithValue(ctx, matcherKey, boolMatcher(true))
		ctx = context.WithValue(ctx, matcherKey, boolMatcher(true))
		ctx = context.WithValue(ctx, nameKey("answer"), 42)
		ctx = context.WithValue(ctx, pathKey, "/")

		req = req.WithContext(ctx)
		req = r.Route(req)
		ctx = req.Context()

		if p := ctx.Value(matcherKey); p != nil {
			t.Errorf("unexpected pattern %v", p)
		}
		if h := ctx.Value(handlerKey); h != nil {
			t.Errorf("unexpected handler %v", h)
		}
		if h := ctx.Value(nameKey("answer")); h != 42 {
			t.Errorf("context didn't work: got %v, wanted %v", h, 42)
		}
	}
}

func TestRouter(t *testing.T) {
	// end-to-end torture route tests of routing semantics: a generated list
	// of patterns that can be turned off incrementally with a global "high
	// water mark."
	testMatchers := []testMatcher{
		{methods: nil, prefix: "/"},
		{methods: nil, prefix: "/a"},
		{methods: []string{"POST", "PUT"}, prefix: "/a"},
		{methods: []string{"GET", "POST"}, prefix: "/a"},
		{methods: []string{"GET"}, prefix: "/b"},
		{methods: nil, prefix: "/ab"},
		{methods: []string{"POST", "PUT"}, prefix: "/"},
		{methods: nil, prefix: "/ba"},
		{methods: nil, prefix: "/"},
		{methods: []string{}, prefix: "/"},
		{methods: nil, prefix: "/carl"},
		{methods: []string{"PUT"}, prefix: "/car"},
		{methods: nil, prefix: "/cake"},
		{methods: nil, prefix: "/car"},
		{methods: []string{"GET"}, prefix: "/c"},
		{methods: []string{"POST"}, prefix: "/"},
		{methods: []string{"PUT"}, prefix: "/"},
	}

	for _, r := range []Router{&router{}, &simpleRouter{}} {
		mark := new(int)
		for i, p := range testMatchers {
			i := i
			p.index = i
			p.mark = mark
			r.Handle(p, intHandler(i))
		}

		tests := []struct {
			method, path string
			results      []int
		}{
			{"GET", "/", []int{0, 8, 8, 8, 8, 8, 8, 8, 8, -1, -1, -1, -1, -1, -1, -1, -1}},
			{"POST", "/", []int{0, 6, 6, 6, 6, 6, 6, 8, 8, 15, 15, 15, 15, 15, 15, 15, -1}},
			{"PUT", "/", []int{0, 6, 6, 6, 6, 6, 6, 8, 8, 16, 16, 16, 16, 16, 16, 16, 16}},
			{"HEAD", "/", []int{0, 8, 8, 8, 8, 8, 8, 8, 8, -1, -1, -1, -1, -1, -1, -1, -1}},
			{"GET", "/a", []int{0, 1, 3, 3, 8, 8, 8, 8, 8, -1, -1, -1, -1, -1, -1, -1, -1}},
			{"POST", "/a", []int{0, 1, 2, 3, 6, 6, 6, 8, 8, 15, 15, 15, 15, 15, 15, 15, -1}},
			{"PUT", "/a", []int{0, 1, 2, 6, 6, 6, 6, 8, 8, 16, 16, 16, 16, 16, 16, 16, 16}},
			{"HEAD", "/a", []int{0, 1, 8, 8, 8, 8, 8, 8, 8, -1, -1, -1, -1, -1, -1, -1, -1}},
			{"GET", "/b", []int{0, 4, 4, 4, 4, 8, 8, 8, 8, -1, -1, -1, -1, -1, -1, -1, -1}},
			{"POST", "/b", []int{0, 6, 6, 6, 6, 6, 6, 8, 8, 15, 15, 15, 15, 15, 15, 15, -1}},
			{"GET", "/ba", []int{0, 4, 4, 4, 4, 7, 7, 7, 8, -1, -1, -1, -1, -1, -1, -1, -1}},
			{"GET", "/c", []int{0, 8, 8, 8, 8, 8, 8, 8, 8, 14, 14, 14, 14, 14, 14, -1, -1}},
			{"POST", "/c", []int{0, 6, 6, 6, 6, 6, 6, 8, 8, 15, 15, 15, 15, 15, 15, 15, -1}},
			{"GET", "/ab", []int{0, 1, 3, 3, 5, 5, 8, 8, 8, -1, -1, -1, -1, -1, -1, -1, -1}},
			{"POST", "/ab", []int{0, 1, 2, 3, 5, 5, 6, 8, 8, 15, 15, 15, 15, 15, 15, 15, -1}},
			{"GET", "/carl", []int{0, 8, 8, 8, 8, 8, 8, 8, 8, 10, 10, 13, 13, 13, 14, -1, -1}},
			{"POST", "/carl", []int{0, 6, 6, 6, 6, 6, 6, 8, 8, 10, 10, 13, 13, 13, 15, 15, -1}},
			{"HEAD", "/carl", []int{0, 8, 8, 8, 8, 8, 8, 8, 8, 10, 10, 13, 13, 13, -1, -1, -1}},
			{"PUT", "/carl", []int{0, 6, 6, 6, 6, 6, 6, 8, 8, 10, 10, 11, 13, 13, 16, 16, 16}},
			{"GET", "/cake", []int{0, 8, 8, 8, 8, 8, 8, 8, 8, 12, 12, 12, 12, 14, 14, -1, -1}},
			{"PUT", "/cake", []int{0, 6, 6, 6, 6, 6, 6, 8, 8, 12, 12, 12, 12, 16, 16, 16, 16}},
			{"OHAI", "/carl", []int{0, 8, 8, 8, 8, 8, 8, 8, 8, 10, 10, 13, 13, 13, -1, -1, -1}},
		}

		// Run sequence of requests through the router N times, incrementing the
		// mark each time. The net effect is that we can compile the entire set of
		// routes Goji would attempt for every request, ensuring that the router is
		// picking routes in the correct order.
		for i, test := range tests {
			req, err := http.NewRequest(test.method, test.path, nil)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			req = req.WithContext(context.WithValue(context.Background(), pathKey, test.path))
			var out []int
			for *mark = 0; *mark < len(testMatchers); *mark++ {
				req := r.Route(req)
				ctx := req.Context()
				if h := ctx.Value(handlerKey); h != nil {
					out = append(out, int(h.(intHandler)))
				} else {
					out = append(out, -1)
				}
			}
			if !reflect.DeepEqual(out, test.results) {
				t.Errorf("[%d] expected %v got %v", i, test.results, out)
			}
		}
	}
}

func TestRouterContextPropagation(t *testing.T) {
	for _, r := range []Router{&router{}, &simpleRouter{}} {
		r.Handle(contextMatcher{}, intHandler(0))
		_, req := resreq()
		req = req.WithContext(context.WithValue(req.Context(), pathKey, "/"))
		req2 := r.Route(req)
		if hello := req2.Context().Value(nameKey("hello")).(string); hello != "world" {
			t.Fatalf("routed request didn't include correct key from pattern: %q", hello)
		}
	}
}

// simpleRouter is a correct router implementation in its simplest form.
type simpleRouter []route

func (sr *simpleRouter) Handle(matcher Matcher, handler http.Handler) {
	*sr = append(*sr, route{
		matcher: matcher,
		handler: handler,
	})
}

func (sr simpleRouter) Route(req *http.Request) *http.Request {
	for _, r := range sr {
		if req2 := r.matcher.Match(req); req2 != nil {
			return req2.WithContext(&match{
				Context: req2.Context(),
				matcher: r.matcher,
				handler: r.handler,
			})
		}
	}
	return req.WithContext(&match{Context: req.Context()})
}

type boolMatcher bool

func (b boolMatcher) Match(req *http.Request) *http.Request {
	if b {
		return req
	}
	return nil
}

func (b boolMatcher) Prefix() string               { return "" }
func (b boolMatcher) Methods() map[string]struct{} { return nil }

func resreq() (*httptest.ResponseRecorder, *http.Request) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		panic(err)
	}
	return httptest.NewRecorder(), req
}

type testMatcher struct {
	index   int
	mark    *int
	methods []string
	prefix  string
}

func (t testMatcher) Match(r *http.Request) *http.Request {
	ctx := r.Context()
	if t.index < *t.mark {
		return nil
	}
	path := ctx.Value(pathKey).(string)
	if !strings.HasPrefix(path, t.prefix) {
		return nil
	}
	if t.methods != nil {
		if _, ok := t.Methods()[r.Method]; !ok {
			return nil
		}
	}
	return r
}

func (t testMatcher) Prefix() string {
	return t.prefix
}

func (t testMatcher) Methods() map[string]struct{} {
	if t.methods == nil {
		return nil
	}
	m := make(map[string]struct{})
	for _, method := range t.methods {
		m[method] = struct{}{}
	}
	return m
}

type contextMatcher struct{}

func (contextMatcher) Match(req *http.Request) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), nameKey("hello"), "world"))
}

func (contextMatcher) Prefix() string               { return "" }
func (contextMatcher) Methods() map[string]struct{} { return nil }

type intHandler int

func (intHandler) ServeHTTP(http.ResponseWriter, *http.Request) {}
