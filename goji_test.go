package goji

import (
	"context"
	"net/http"
	"testing"
)

func TestWithMatcher(t *testing.T) {
	exp := boolMatcher(true)
	ctx := WithMatcher(context.Background(), exp)
	if m := ctx.Value(matcherKey).(Matcher); m != exp {
		t.Errorf("expected %+v, got: %+v", exp, m)
	}
}

func TestWithHandler(t *testing.T) {
	exp := intHandler(1)
	ctx := WithHandler(context.Background(), exp)
	if h := ctx.Value(handlerKey).(http.Handler); h != exp {
		t.Errorf("expected %+v, got: %+v", exp, h)
	}
}

func TestWithPath(t *testing.T) {
	ctx := WithPath(context.Background(), "hi")
	if path := Path(ctx); path != "hi" {
		t.Errorf("expected hi, got: %q", path)
	}

	if path := Path(context.Background()); path != "" {
		t.Errorf("expected empty path, got: %q", path)
	}
}
