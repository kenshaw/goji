package goji

import (
	"context"
	"net/http"
	"reflect"
	"testing"
)

func TestPathSpec(t *testing.T) {
	type pv map[nameKey]interface{}

	tests := []struct {
		spec  string
		req   string
		match bool
		vars  map[nameKey]interface{}
		exp   string
	}{
		{"/", "/", true, nil, ""},
		{"/", "/hello", false, nil, ""},
		{"/hello", "/hello", true, nil, ""},

		{"/:name", "/carl", true, pv{"name": "carl"}, ""},
		{"/:name", "/carl/", false, nil, ""},
		{"/:name", "/", false, nil, ""},
		{"/:name/", "/carl/", true, pv{"name": "carl"}, ""},
		{"/:name/", "/carl/no", false, nil, ""},
		{"/:name/hi", "/carl/hi", true, pv{"name": "carl"}, ""},
		{"/:name/:color", "/carl/red", true, pv{"name": "carl", "color": "red"}, ""},
		{"/:name/:color", "/carl/", false, nil, ""},
		{"/:name/:color", "/carl.red", false, nil, ""},

		{"/:file.:ext", "/data.json", true, pv{"file": "data", "ext": "json"}, ""},
		{"/:file.:ext", "/data.tar.gz", true, pv{"file": "data", "ext": "tar.gz"}, ""},
		{"/:file.:ext", "/data", false, nil, ""},
		{"/:file.:ext", "/data.", false, nil, ""},
		{"/:file.:ext", "/.gitconfig", false, nil, ""},
		{"/:file.:ext", "/data.json/", false, nil, ""},
		{"/:file.:ext", "/data/json", false, nil, ""},
		{"/:file.:ext", "/data;json", false, nil, ""},
		{"/hello.:ext", "/hello.json", true, pv{"ext": "json"}, ""},
		{"/:file.json", "/hello.json", true, pv{"file": "hello"}, ""},
		{"/:file.json", "/hello.world.json", false, nil, ""},
		{"/file;:version", "/file;1", true, pv{"version": "1"}, ""},
		{"/file;:version", "/file,1", false, nil, ""},
		{"/file,:version", "/file,1", true, pv{"version": "1"}, ""},
		{"/file,:version", "/file;1", false, nil, ""},

		{"/*", "/", true, nil, "/"},
		{"/*", "/hello", true, nil, "/hello"},
		{"/users/*", "/", false, nil, ""},
		{"/users/*", "/users", false, nil, ""},
		{"/users/*", "/users/", true, nil, "/"},
		{"/users/*", "/users/carl", true, nil, "/carl"},
		{"/users/*", "/profile/carl", false, nil, ""},
		{"/:name/*", "/carl", false, nil, ""},
		{"/:name/*", "/carl/", true, pv{"name": "carl"}, "/"},
		{"/:name/*", "/carl/photos", true, pv{"name": "carl"}, "/photos"},
		{"/:name/*", "/carl/photos%2f2015", true, pv{"name": "carl"}, "/photos%2f2015"},
	}

	for i, test := range tests {
		p := NewPathSpec(test.spec)

		if str := p.String(); test.spec != str {
			t.Errorf("test %d [%q %q] expected=%q, String()=%q", i, test.spec, test.req, test.exp, str)
		}

		req := p.Match(reqPath("GET", test.req))
		if (req != nil) != test.match {
			t.Errorf("test %d [%q %q] expected=%v, match=%v", i, test.spec, test.req, test.match, req != nil)
		}
		if req == nil {
			continue
		}

		ctx := req.Context()
		if path := Path(ctx); test.exp != path {
			t.Errorf("test %d [%q %q] expected=%q, path=%q", i, test.spec, test.req, test.exp, path)
		}

		vars := ctx.Value(allNames)
		if (vars != nil) != (test.vars != nil) {
			t.Errorf("test %d [%q %q] vars=%#v, expected=%#v", i, test.spec, test.req, vars, test.vars)
		}
		if vars == nil {
			continue
		}
		if tvars := vars.(map[nameKey]interface{}); !reflect.DeepEqual(tvars, test.vars) {
			t.Errorf("test %d [%q %q] vars=%v, expected=%v", i, test.spec, test.req, tvars, test.vars)
		}
	}
}

func TestPathSpecBadPathEncoding(t *testing.T) {
	// This one is hard to fit into the table-driven test above since Go
	// refuses to have anything to do with poorly encoded URLs.
	ctx := WithPath(context.Background(), "/%nope")
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if NewPathSpec("/:name").Match(req.WithContext(ctx)) != nil {
		t.Error("unexpected match")
	}
}

func TestPrefix(t *testing.T) {
	tests := []struct {
		spec   string
		prefix string
	}{
		{"/", "/"},
		{"/hello/:world", "/hello/"},
		{"/users/:name/profile", "/users/"},
		{"/users/*", "/users/"},
	}

	for _, test := range tests {
		p := NewPathSpec(test.spec)
		if prefix := p.Prefix(); prefix != test.prefix {
			t.Errorf("%q.Prefix() = %q, expected %q", test.spec, prefix, test.prefix)
		}
	}
}

func TestMethods(t *testing.T) {
	p := NewPathSpec("/foo")
	if methods := p.Methods(); methods != nil {
		t.Errorf("expected nil with no methods, got %v", methods)
	}

	p = Get("/boo")
	expect := map[string]struct{}{"GET": {}, "HEAD": {}}
	if methods := p.Methods(); !reflect.DeepEqual(expect, methods) {
		t.Errorf("methods=%v, expected %v", methods, expect)
	}
}

func TestParam(t *testing.T) {
	p := NewPathSpec("/hello/:name")
	req := p.Match(reqPath("GET", "/hello/carl"))
	if req == nil {
		t.Fatal("expected a match")
	}
	if name := Param(req, "name"); name != "carl" {
		t.Errorf("name=%q, expected %q", name, "carl")
	}
}

func TestNewWithMethod(t *testing.T) {
	p := NewPathSpec("/", WithMethod("LOCK", "UNLOCK"))
	if p.Match(reqPath("POST", "/")) != nil {
		t.Errorf("pattern was LOCK/UNLOCK, but matched POST")
	}
	if p.Match(reqPath("LOCK", "/")) == nil {
		t.Errorf("pattern didn't match LOCK")
	}
	if p.Match(reqPath("UNLOCK", "/")) == nil {
		t.Errorf("pattern didn't match UNLOCK")
	}
}

func TestDelete(t *testing.T) {
	p := Delete("/")
	if p.Match(reqPath("GET", "/")) != nil {
		t.Errorf("pattern was DELETE, but matched GET")
	}
	if p.Match(reqPath("DELETE", "/")) == nil {
		t.Errorf("pattern didn't match DELETE")
	}
}

func TestGet(t *testing.T) {
	p := Get("/")
	if p.Match(reqPath("POST", "/")) != nil {
		t.Errorf("pattern was GET, but matched POST")
	}
	if p.Match(reqPath("GET", "/")) == nil {
		t.Errorf("pattern didn't match GET")
	}
	if p.Match(reqPath("HEAD", "/")) == nil {
		t.Errorf("pattern didn't match HEAD")
	}
}

func TestHead(t *testing.T) {
	p := Head("/")
	if p.Match(reqPath("GET", "/")) != nil {
		t.Errorf("pattern was HEAD, but matched GET")
	}
	if p.Match(reqPath("HEAD", "/")) == nil {
		t.Errorf("pattern didn't match HEAD")
	}
}

func TestOptions(t *testing.T) {
	p := Options("/")
	if p.Match(reqPath("GET", "/")) != nil {
		t.Errorf("pattern was OPTIONS, but matched GET")
	}
	if p.Match(reqPath("OPTIONS", "/")) == nil {
		t.Errorf("pattern didn't match OPTIONS")
	}
}

func TestPatch(t *testing.T) {
	p := Patch("/")
	if p.Match(reqPath("GET", "/")) != nil {
		t.Errorf("pattern was PATCH, but matched GET")
	}
	if p.Match(reqPath("PATCH", "/")) == nil {
		t.Errorf("pattern didn't match PATCH")
	}
}

func TestPost(t *testing.T) {
	p := Post("/")
	if p.Match(reqPath("GET", "/")) != nil {
		t.Errorf("pattern was POST, but matched GET")
	}
	if p.Match(reqPath("POST", "/")) == nil {
		t.Errorf("pattern didn't match POST")
	}
}

func TestPut(t *testing.T) {
	p := Put("/")
	if p.Match(reqPath("GET", "/")) != nil {
		t.Errorf("pattern was PUT, but matched GET")
	}
	if p.Match(reqPath("PUT", "/")) == nil {
		t.Errorf("pattern didn't match PUT")
	}
}

func TestExistingContext(t *testing.T) {
	p := NewPathSpec("/hi/:c/:a/:r/:l")
	req, err := http.NewRequest("GET", "/hi/foo/bar/baz/quux", nil)
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	ctx = WithPath(ctx, req.URL.EscapedPath())
	ctx = context.WithValue(ctx, allNames, map[nameKey]interface{}{
		"hello": "world",
		"c":     "nope",
	})
	ctx = context.WithValue(ctx, nameKey("user"), "carl")

	req = req.WithContext(ctx)
	req = p.Match(req)
	if req == nil {
		t.Fatalf("expected pattern to match")
	}
	ctx = req.Context()

	exp := map[nameKey]interface{}{
		"c": "foo",
		"a": "bar",
		"r": "baz",
		"l": "quux",
	}
	for k, v := range exp {
		if p := Param(req, string(k)); p != v {
			t.Errorf("expected %s=%q, got %q", k, v, p)
		}
	}

	exp["hello"] = "world"
	all := ctx.Value(allNames).(map[nameKey]interface{})
	if !reflect.DeepEqual(all, exp) {
		t.Errorf("expected %v, got %v", exp, all)
	}

	if path := Path(ctx); path != "" {
		t.Errorf("expected path=%q, got %q", "", path)
	}

	if user := ctx.Value(nameKey("user")); user != "carl" {
		t.Errorf("expected user=%q, got %q", "carl", user)
	}
}

func reqPath(method, path string) *http.Request {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		panic(err)
	}
	return req.WithContext(WithPath(context.Background(), req.URL.EscapedPath()))
}
