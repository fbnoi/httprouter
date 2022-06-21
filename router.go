package httprouter

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type HandleFunc func(r *http.Request, w http.ResponseWriter, ps Params)

func NewRouteTree(config *Config) *RouteTree {
	return &RouteTree{
		once: &sync.Once{},
		RouteNode: &RouteNode{
			root: true,
			leaf: false,
		},
		config: config,
	}
}

const (
	METHOD_GET     = http.MethodGet
	METHOD_POST    = http.MethodPost
	METHOD_HEAD    = http.MethodHead
	METHOD_PUT     = http.MethodPut
	METHOD_PATCH   = http.MethodPatch
	METHOD_DELETE  = http.MethodDelete
	METHOD_CONNECT = http.MethodConnect
	METHOD_OPTIONS = http.MethodOptions
	METHOD_TRACE   = http.MethodTrace
)

var allowed_methods = []string{
	METHOD_GET, METHOD_POST, METHOD_HEAD,
	METHOD_PUT, METHOD_PATCH, METHOD_DELETE,
}

type Config struct {
	RedirectFixedPath bool
}

// RouteTree keeps a nodeTree where stores the registered path and it's
// handler.
// TODO: add name to route as to generate a url by name
type RouteTree struct {
	*RouteNode
	once            *sync.Once
	paramsPool      sync.Pool
	config          *Config
	notFoundHandler HandleFunc
	mapper          map[string]*RouteNode
}

func (rt *RouteTree) SetNotFoundHandleFunc(fn HandleFunc) {
	rt.notFoundHandler = fn
}

func (rt *RouteTree) GeneratePath(name string, ps Params) string {
	if node, ok := rt.GetRoute(name); ok {
		var (
			s   []string
			rp  = make(map[string]interface{})
			qpb = &strings.Builder{}
		)
		for !node.root {
			if node.wildcard {
				p := ps.ByName(node.name)
				if p == nil {
					panic(fmt.Sprintf("route parameter %s is required", node.name))
				}
				pStr := interfaceToString(p)
				if !node.fit(pStr) {
					panic(fmt.Sprintf("%s not compete with %s for parameter %s", p, node.rawPattern, node.name))
				}
				s = append(s, url.PathEscape(pStr))
				rp[node.name] = nil
			} else {
				s = append(s, node.name)
			}
			node = node.parent
		}
		for _, p := range ps {
			if _, ok := rp[p.key]; !ok {
				qpb.WriteString(url.QueryEscape(p.key))
				qpb.WriteString("=")
				qpb.WriteString(url.QueryEscape(interfaceToString(p.value)))
				qpb.WriteString("&")
			}
		}
		path, query := cleanPath(strings.Join(s, "/")), strings.Trim(qpb.String(), "&")

		if query == "" {
			return path
		}

		return path + "?" + query
	}
	panic(fmt.Sprintf("route \"%s\" don't exist", name))
}

func (rt *RouteTree) GetRoute(name string) (r *RouteNode, ok bool) {
	r, ok = rt.mapper[name]
	return
}

// ServeHTTP serve the http request
func (rt *RouteTree) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	path := r.URL.Path
	fixedPath := cleanPath(path)

	// if path is dirty, redirect
	if path != fixedPath && rt.config.RedirectFixedPath {
		r.URL.Path = fixedPath
		redirect(r.URL.String(), r, w)
		return
	}

	// if path end up with '/', redirect
	if len(fixedPath) > 1 && strings.HasSuffix(fixedPath, "/") {
		r.URL.Path = strings.TrimRight(fixedPath, "/")
		redirect(r.URL.String(), r, w)
		return
	}
	node, params := rt.search(path, rt.getParams)
	defer rt.putParams(params)

	// not found
	if node == nil {
		if rt.notFoundHandler == nil {
			notFound(r, w, *params)
			return
		}
		rt.notFoundHandler(r, w, *params)
		return
	}
	handleFunc, ok := node.getHandler(r.Method)
	if !ok {
		if rt.notFoundHandler == nil {
			notFound(r, w, *params)
			return
		}
		rt.notFoundHandler(r, w, *params)
		return
	}

	handleFunc(r, w, *params)
}

func (rt *RouteTree) Group(path string, fn func(*RouteTree), mds HandleFunc) {
	node := rt.RouteNode.pave(path)
	tree := &RouteTree{
		RouteNode: node,
		once:      &sync.Once{},
	}
	fn(tree)
}

func (rt *RouteTree) POST(name, path string, handler HandleFunc) *RouteTree {
	return rt.addPath(name, METHOD_POST, path, handler)
}

func (rt *RouteTree) GET(name, path string, handler HandleFunc) *RouteTree {
	return rt.addPath(name, METHOD_GET, path, handler)
}

func (rt *RouteTree) HEAD(name, path string, handler HandleFunc) *RouteTree {
	return rt.addPath(name, METHOD_HEAD, path, handler)
}

func (rt *RouteTree) PUT(name, path string, handler HandleFunc) *RouteTree {
	return rt.addPath(name, METHOD_HEAD, path, handler)
}

func (rt *RouteTree) PATCH(name, path string, handler HandleFunc) *RouteTree {
	return rt.addPath(name, METHOD_PATCH, path, handler)
}

func (rt *RouteTree) DELETE(name, path string, handler HandleFunc) *RouteTree {
	return rt.addPath(name, METHOD_DELETE, path, handler)
}

func (rt *RouteTree) Handle(name, method, path string, handler HandleFunc) *RouteTree {
	return rt.addPath(name, method, path, handler)
}

func (rt *RouteTree) All(name, path string, handler HandleFunc) *RouteTree {
	for _, method := range allowed_methods {
		rt.addPath(name, method, path, handler)
	}
	return rt
}

func (rt *RouteTree) addPath(name, method, path string, handler HandleFunc) *RouteTree {
	if !allowMethod(method) {
		methodNotAllowed(method)
	}
	rt.init()
	if _, ok := rt.GetRoute(name); !ok {
		rt.mapper[name] = rt.RouteNode.addPath(name, method, path, handler)

		return rt
	}
	panic(fmt.Sprintf("route name \"%s\" already been used", name))
}

func (rt *RouteTree) init() *RouteTree {
	rt.once.Do(func() {
		rt.paramsPool.New = func() interface{} {
			ps := make(Params, 0)
			return &ps
		}
		rt.mapper = make(map[string]*RouteNode)
	})
	return rt
}

func (rt *RouteTree) getParams() *Params {
	ps, _ := rt.paramsPool.Get().(*Params)
	*ps = (*ps)[0:0] // reset slice
	return ps
}

func (rt *RouteTree) putParams(ps *Params) {
	if ps != nil {
		rt.paramsPool.Put(ps)
	}
}

func allowMethod(method string) bool {
	for _, m := range allowed_methods {
		if method == m {
			return true
		}
	}
	return false
}

func methodNotAllowed(method string) {
	panic(fmt.Sprintf("method [%s] not allowed", method))
}

func notFound(r *http.Request, w http.ResponseWriter, ps Params) {
	w.WriteHeader(http.StatusNotFound)
}

func redirect(location string, r *http.Request, w http.ResponseWriter) {
	http.Redirect(w, r, location, http.StatusTemporaryRedirect)
}
