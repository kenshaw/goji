package goji

import (
	"context"
	"net/http"
	"sort"
	"strings"
)

// Router is the shared router interface.
type Router interface {
	Handle(Matcher, http.Handler)
	Route(*http.Request) *http.Request
}

type route struct {
	matcher Matcher
	handler http.Handler
}

type match struct {
	context.Context
	matcher Matcher
	handler http.Handler
}

func (m match) Value(key interface{}) interface{} {
	switch key {
	case matcherKey:
		return m.matcher
	case handlerKey:
		return m.handler
	default:
		return m.Context.Value(key)
	}
}

type router struct {
	routes   []route
	methods  map[string]*trieNode
	wildcard trieNode
}

func (r *router) Handle(matcher Matcher, handler http.Handler) {
	i := len(r.routes)
	r.routes = append(r.routes, route{matcher: matcher, handler: handler})

	prefix, methods := matcher.Prefix(), matcher.Methods()
	if methods == nil {
		r.wildcard.add(prefix, i)
		for _, sub := range r.methods {
			sub.add(prefix, i)
		}
	} else {
		if r.methods == nil {
			r.methods = make(map[string]*trieNode)
		}

		for method := range methods {
			if _, ok := r.methods[method]; !ok {
				r.methods[method] = r.wildcard.clone()
			}
			r.methods[method].add(prefix, i)
		}
	}
}

func (r *router) Route(req *http.Request) *http.Request {
	tn := &r.wildcard
	if tn2, ok := r.methods[req.Method]; ok {
		tn = tn2
	}

	ctx := req.Context()
	path := ctx.Value(pathKey).(string)
	for path != "" {
		i := sort.Search(len(tn.children), func(i int) bool {
			return path[0] <= tn.children[i].prefix[0]
		})
		if i == len(tn.children) || !strings.HasPrefix(path, tn.children[i].prefix) {
			break
		}

		path = path[len(tn.children[i].prefix):]
		tn = tn.children[i].node
	}

	for _, i := range tn.routes {
		if req2 := r.routes[i].matcher.Match(req); req2 != nil {
			return req2.WithContext(&match{
				Context: req2.Context(),
				matcher: r.routes[i].matcher,
				handler: r.routes[i].handler,
			})
		}
	}

	return req.WithContext(&match{Context: ctx})
}

type child struct {
	prefix string
	node   *trieNode
}

type trieNode struct {
	routes   []int
	children []child
}

func (tn *trieNode) add(prefix string, idx int) {
	if len(prefix) == 0 {
		tn.routes = append(tn.routes, idx)
		for i := range tn.children {
			tn.children[i].node.add(prefix, idx)
		}
		return
	}

	ch := prefix[0]
	i := sort.Search(len(tn.children), func(i int) bool {
		return ch <= tn.children[i].prefix[0]
	})

	if i == len(tn.children) || ch != tn.children[i].prefix[0] {
		routes := append([]int(nil), tn.routes...)
		tn.children = append(tn.children, child{
			prefix: prefix,
			node:   &trieNode{routes: append(routes, idx)},
		})
	} else {
		lp := longestPrefix(prefix, tn.children[i].prefix)

		if tn.children[i].prefix == lp {
			tn.children[i].node.add(prefix[len(lp):], idx)
			return
		}

		split := new(trieNode)
		split.children = []child{
			{tn.children[i].prefix[len(lp):], tn.children[i].node},
		}
		split.routes = append([]int(nil), tn.routes...)
		split.add(prefix[len(lp):], idx)

		tn.children[i].prefix = lp
		tn.children[i].node = split
	}

	sort.Sort(byPrefix(tn.children))
}

func (tn *trieNode) clone() *trieNode {
	clone := new(trieNode)
	clone.routes = append(clone.routes, tn.routes...)
	clone.children = append(clone.children, tn.children...)
	for i := range clone.children {
		clone.children[i].node = tn.children[i].node.clone()
	}
	return clone
}

// We can be a teensy bit more efficient here: we're maintaining a sorted list,
// so we know exactly where to insert the new element. But since that involves
// more bookkeeping and makes the code messier, let's cross that bridge when we
// come to it.
type byPrefix []child

func (b byPrefix) Len() int {
	return len(b)
}
func (b byPrefix) Less(i, j int) bool {
	return b[i].prefix < b[j].prefix
}
func (b byPrefix) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func longestPrefix(a, b string) string {
	mlen := len(a)
	if len(b) < mlen {
		mlen = len(b)
	}
	for i := 0; i < mlen; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:mlen]
}
