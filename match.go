package goji

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// Matcher determines whether a given request matches some criteria. Goji users
// looking for a concrete type that implements this interface should consider
// Goji's "pat" sub-package, which implements a small domain specific language
// for HTTP routing.
//
// Matchers typically only examine a small portion of incoming requests, most
// commonly the HTTP method and the URL's RawPath. As an optimization, Goji can
// elide calls to your Matcher for requests it knows cannot match. Matcher
// authors who wish to take advantage of this functionality (and in some cases
// an asymptotic performance improvement) can augment their Matcher
// implementations with any of the matched methods.
//
// The presence or lack of these performance improvements should be viewed as
// an implementation detail and are not part of Goji's API compatibility
// guarantee. It is the responsibility of Matcher authors to ensure that their
// Match function always returns correct results, even if these optimizations
// are not performed.
//
// All operations on Matchers must be safe for concurrent use by multiple
// goroutines.
type Matcher interface {
	// Match examines the input Request to determine if it matches some
	// criteria, and if so returns a non-nil output Request. This returned
	// Request will be passed to the middleware stack and the final Handler.
	//
	// Matchers often extract variables from the Request, for instance from the
	// URL or from HTTP headers. In this case, it is common for the Request
	// returned from the Match function to be derived from the input Request
	// using the WithContext function, with a Context that contains those
	// variable bindings. If no variable bindings are necessary, another common
	// choice is to return the input Request unchanged.
	//
	// Match must not mutate the passed request if it returns nil.
	// Implementers are also strongly discouraged from mutating the input
	// Request even in the event of a match; instead, prefer making a copy.
	Match(*http.Request) *http.Request

	// Methods returns the set of HTTP methods that the Matcher matches, or nil
	// if it's not possible to determine which HTTP methods might be matched.
	// Put another way, requests with HTTP methods not in the returned set are
	// guaranteed to never match this Matcher.
	Methods() map[string]struct{}

	// Prefix returns a string which all RawPaths that the Matcher must
	// have as a prefix. Put another way, requests with RawPaths that do not
	// contain the returned string as a prefix are guaranteed to never match
	// this Matcher.
	Prefix() string
}

// allNames is a standard value which, when passed to
// context.Context.Value, returns all variable bindings present in the context,
// with bindings in newer contexts overriding values deeper in the stack. The
// concrete type
//
// 	map[nameKey]interface{}
//
// is used for this purpose. If no variables are bound, nil should be returned
// instead of an empty map.
var allNames = struct{}{}

type matchContext struct {
	context.Context
	spec    *SpecMatcher
	matches []string
}

func (m matchContext) Value(key interface{}) interface{} {
	switch key {
	case allNames:
		var vs map[nameKey]interface{}
		if vsi := m.Context.Value(key); vsi == nil {
			if len(m.spec.specs) == 0 {
				return nil
			}
			vs = make(map[nameKey]interface{}, len(m.matches))
		} else {
			vs = vsi.(map[nameKey]interface{})
		}

		for _, p := range m.spec.specs {
			vs[p.name] = m.matches[p.idx]
		}
		return vs

	case pathKey:
		if len(m.matches) == len(m.spec.specs)+1 {
			return m.matches[len(m.matches)-1]
		}
		return ""
	}

	if k, ok := key.(nameKey); ok {
		i := sort.Search(len(m.spec.specs), func(i int) bool {
			return m.spec.specs[i].name >= k
		})
		if i < len(m.spec.specs) && m.spec.specs[i].name == k {
			return m.matches[m.spec.specs[i].idx]
		}
	}

	return m.Context.Value(key)
}

type specNames []struct {
	name nameKey
	idx  int
}

func (p specNames) Len() int {
	return len(p)
}
func (p specNames) Less(i, j int) bool {
	return p[i].name < p[j].name
}
func (p specNames) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

// SpecMatcher provides a Matcher that matches routes based on named path
// components, storing matched path components in the request context.
//
// Quick Reference
//
// The following table gives an overview of the language this package accepts. See
// the subsequent sections for a more detailed explanation of what each pattern
// does.
//
// 	Path			Matches			Does Not Match
//
// 	/			/			/hello
//
// 	/hello			/hello			/hi
// 							/hello/
//
// 	/user/:name		/user/carl		/user/carl/photos
// 				/user/alice		/user/carl/
// 							/user/
//
// 	/:file.:ext		/data.json		/.json
// 				/info.txt		/data.
// 				/data.tar.gz		/data.json/download
//
// 	/user/*			/user/			/user
// 				/user/carl
// 				/user/carl/photos
//
// Static Paths
//
// Most URL paths may be specified directly: the pattern "/hello" matches URLs with
// precisely that path ("/hello/", for instance, is treated as distinct).
//
// Note that this package operates on raw (i.e., escaped) paths (see the
// documentation for net/url.URL.EscapedPath). In order to match a character that
// can appear escaped in a URL path, use its percent-encoded form.
//
// Named Matches
//
// Named matches allow URL paths to contain any value in a particular path segment.
// Such matches are denoted by a leading ":", for example ":name" in the rule
// "/user/:name", and permit any non-empty value in that position. For instance, in
// the previous "/user/:name" example, the path "/user/carl" is matched, while
// "/user/" or "/user/carl/" (note the trailing slash) are not matched. Pat rules
// can contain any number of named matches.
//
// Named matches set URL variables by comparing pattern names to the segments they
// matched. In our "/user/:name" example, a request for "/user/carl" would bind the
// "name" variable to the value "carl". Use the Param function to extract these
// variables from the request context. Variable names in a single pattern must be
// unique.
//
// Matches are ordinarily delimited by slashes ("/"), but several other characters
// are accepted as delimiters (with slightly different semantics): the period
// ("."), semicolon (";"), and comma (",") characters. For instance, given the
// pattern "/:file.:ext", the request "/data.json" would match, binding "file" to
// "data" and "ext" to "json". Note that these special characters are treated
// slightly differently than slashes: the above pattern also matches the path
// "/data.tar.gz", with "ext" getting set to "tar.gz"; and the pattern "/:file"
// matches names with dots in them (like "data.json").
//
// Prefix Matches
//
// Pat can also match prefixes of routes using wildcards. Prefix wildcard routes
// end with "/*", and match just the path segments preceding the asterisk. For
// instance, the pattern "/user/*" will match "/user/" and "/user/carl/photos" but
// not "/user" (note the lack of a trailing slash).
//
// The unmatched suffix, including the leading slash ("/"), are placed into the
// request context, which allows subsequent routing (e.g., a subrouter) to continue
// from where this pattern left off. For instance, in the "/user/*" pattern from
// above, a request for "/user/carl/photos" will consume the "/user" prefix,
// leaving the path "/carl/photos" for subsequent patterns to handle. A subrouter
// pattern for "/:name/photos" would match this remaining path segment, for
// instance.
type SpecMatcher struct {
	raw     string
	methods map[string]struct{}

	// specs are parallel arrays of each pattern string (sans ":"), the
	// breaks each expect afterwords (used to support e.g., "." dividers), and
	// the string literals in between every pattern. There is always one more
	// literal than pattern, and they are interleaved like this: <literal>
	// <pattern> <literal> <pattern> <literal> etc...
	specs specNames

	breaks   []byte
	literals []string
	wildcard bool
}

// breaksRE is a regexp for "Break characters" that can end patterns. They are
// not allowed to appear in pattern names. "/" was chosen because it is the
// standard path separator, and "." was chosen because it often delimits file
// extensions. ";" and "," were chosen because Section 3.3 of RFC 3986 suggests
// their use.
var breaksRE = regexp.MustCompile(`[/.;,]:([^/.;,]+)`)

// New returns a new SpecMatcher from the given Pat route. See the package
// documentation for more information about what syntax is accepted by this
// function.
func New(pat string, opts ...SpecMatcherOption) *SpecMatcher {
	p := &SpecMatcher{raw: pat}
	for _, o := range opts {
		o(p)
	}

	if strings.HasSuffix(pat, "/*") {
		pat = pat[:len(pat)-1]
		p.wildcard = true
	}

	matches := breaksRE.FindAllStringSubmatchIndex(pat, -1)
	numMatches := len(matches)
	p.specs = make(specNames, numMatches)
	p.breaks = make([]byte, numMatches)
	p.literals = make([]string, numMatches+1)

	n := 0
	for i, match := range matches {
		a, b := match[2], match[3]
		p.literals[i] = pat[n : a-1] // Need to leave off the colon
		p.specs[i].name = nameKey(pat[a:b])
		p.specs[i].idx = i
		if b == len(pat) {
			p.breaks[i] = '/'
		} else {
			p.breaks[i] = pat[b]
		}
		n = b
	}
	p.literals[numMatches] = pat[n:]

	sort.Sort(p.specs)

	return p
}

// Match runs the Pat pattern on the given request, returning a non-nil output
// request if the input request matches the pattern.
func (p *SpecMatcher) Match(req *http.Request) *http.Request {
	if p.methods != nil {
		if _, ok := p.methods[req.Method]; !ok {
			return nil
		}
	}

	// Check Path
	ctx := req.Context()
	path := Path(ctx)
	var scratch []string
	if p.wildcard {
		scratch = make([]string, len(p.specs)+1)
	} else if len(p.specs) > 0 {
		scratch = make([]string, len(p.specs))
	}

	for i := range p.specs {
		sli := p.literals[i]
		if !strings.HasPrefix(path, sli) {
			return nil
		}
		path = path[len(sli):]

		m := 0
		bc := p.breaks[i]
		for ; m < len(path); m++ {
			if path[m] == bc || path[m] == '/' {
				break
			}
		}

		if m == 0 {
			// Empty strings are not matches, otherwise routes like "/:foo"
			// would match the path "/"
			return nil
		}

		scratch[i], path = path[:m], path[m:]
	}

	// There's exactly one more literal than pat.
	tail := p.literals[len(p.specs)]
	if p.wildcard {
		if !strings.HasPrefix(path, tail) {
			return nil
		}
		scratch[len(p.specs)] = path[len(tail)-1:]
	} else if path != tail {
		return nil
	}

	for i := range p.specs {
		var err error
		scratch[i], err = unescape(scratch[i])
		if err != nil {
			// If we encounter an encoding error here, there's really not much
			// we can do about it with our current API, and I'm not really
			// interested in supporting clients that misencode URLs anyways.
			return nil
		}
	}

	return req.WithContext(&matchContext{ctx, p, scratch})
}

// Methods returns a set of HTTP methods that all requests that this
// SpecMatcher matches must be in, or nil if it's not possible to determine
// which HTTP methods might be matched.
func (p *SpecMatcher) Methods() map[string]struct{} {
	return p.methods
}

// Prefix returns a string prefix that the Paths of all requests that this
// SpecMatcher accepts must contain.
//
// This function satisfies goji's Prefix SpecMatcher optimization.
func (p *SpecMatcher) Prefix() string {
	return p.literals[0]
}

// String satisfies the fmt.Stringer interface.
func (p *SpecMatcher) String() string {
	return p.raw
}

// Delete returns a Pat route that only matches the DELETE HTTP method.
func Delete(pat string) *SpecMatcher {
	return New(pat, WithMethod("DELETE"))
}

// Get returns a Pat route that only matches the GET and HEAD HTTP method. HEAD
// requests are handled transparently by net/http.
func Get(pat string) *SpecMatcher {
	return New(pat, WithMethod("GET", "HEAD"))
}

// Head returns a Pat route that only matches the HEAD HTTP method.
func Head(pat string) *SpecMatcher {
	return New(pat, WithMethod("HEAD"))
}

// Options returns a Pat route that only matches the OPTIONS HTTP method.
func Options(pat string) *SpecMatcher {
	return New(pat, WithMethod("OPTIONS"))
}

// Patch returns a Pat route that only matches the PATCH HTTP method.
func Patch(pat string) *SpecMatcher {
	return New(pat, WithMethod("PATCH"))
}

// Post returns a Pat route that only matches the POST HTTP method.
func Post(pat string) *SpecMatcher {
	return New(pat, WithMethod("POST"))
}

// Put returns a Pat route that only matches the PUT HTTP method.
func Put(pat string) *SpecMatcher {
	return New(pat, WithMethod("PUT"))
}

// SpecMatcherOption is a pattern option.
type SpecMatcherOption func(*SpecMatcher)

// WithMethod is a pattern option to set the matching HTTP methods.
func WithMethod(methods ...string) SpecMatcherOption {
	return func(p *SpecMatcher) {
		methodSet := make(map[string]struct{}, len(methods))
		for _, method := range methods {
			methodSet[method] = struct{}{}
		}
		p.methods = methodSet
	}
}
