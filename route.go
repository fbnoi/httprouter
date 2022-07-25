package httprouter

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

type HandleFunc func(r *http.Request, w http.ResponseWriter, ps Params)

type routeNode struct {
	parent     *routeNode
	children   []*routeNode
	root       bool
	leaf       bool
	part       string
	fullPath   string
	routeName  string
	pattern    *regexp.Regexp
	rawPattern string
	wildcard   bool
	handleFunc map[string]HandleFunc
}

func (r *routeNode) RouteName() string {
	return r.routeName
}

func (r *routeNode) FullPath() string {
	return r.fullPath
}

// search the given path, if find, return matched node and a params,
// if not, return nil, nil.
func (r *routeNode) search(path string, getParams func() *Params) (*routeNode, *Params) {
	//split given path to a slice and make a slice of collection.Iterator with the capacity
	//of the len of path slice. each path item will go through a list of entry, if match then go to
	//find in this entry's children list. if there is no match in current list, go upper to
	//find another entry and so on. if no match at all, not found.
	var (
		parts    = strings.Split(path, "/")
		ptrStore = make([]int, len(parts))
		k        = r
		at, ptr  = 1, 0
		ps       = getParams()
	)
	// beautyPath always start with '/' and the begin of the path must be "".
	// search from root entry's children and skip the begin of path slice.
	// it means that root must match with the begin of path slice.
	for at >= 1 && at < len(parts) {
		find := false
		for ptr < len(k.children) {
			child := k.children[ptr]
			// if find fit node
			if child.fit(parts[at]) {
				// push Iterator to same position
				ptrStore[at] = ptr
				find = true
				k = child
				//save the params if necessary
				if child.wildcard {
					*ps = append(*ps, Param{key: child.part, value: parts[at]})
				}
				// then we need to go further
				ptr = 0
				at++
				// but if end, we can not
				if at >= len(parts) {
					break
				}
				continue
			}

			ptr++
		}
		// this is a blind alley, we need go back to find way out in another entry.
		if !find {
			// if the upper entry is a wildcard, the last param should be removed
			if ps != nil && len(*ps) > 0 {
				*ps = (*ps)[:len(*ps)-1]
			}
			at--
			ptr = ptrStore[at]
		}
	}

	// if it return to the root of the tree, or the matched node is not a leaf,
	// there is no matched node.
	if at == 0 || !k.leaf {
		return nil, ps
	}

	return k, ps
}

// addPath add a new path to this node's children, then return the leaf node.
func (r *routeNode) addPath(name, method, path string, handler HandleFunc) *routeNode {
	node := r.pave(path)
	var (
		pc  = node.parent.children
		ptr = 0
	)
	for ptr < len(pc) {
		sibling := pc[ptr]
		if node != sibling && sibling.leaf {
			conflict := fmt.Sprintf("route [%s] conflict with [%s]", path, sibling.fullPath)
			if sibling.wildcard != node.wildcard {
				switch {
				case sibling.wildcard && sibling.pattern != nil && sibling.pattern.MatchString(node.part):
					panic(conflict)
				case node.wildcard && node.pattern != nil && node.pattern.MatchString(sibling.part):
					panic(conflict)
				}
			} else if sibling.wildcard {
				panic(conflict)
			}
		}
		ptr++
	}
	node.setHandler(method, handler)
	node.routeName = name

	return node
}

// pave search the matched node in the tree and then return the matched node.
// if no matched node exist, find the last matched node and add a new path, then return
// the last added node.
func (r *routeNode) pave(path string) *routeNode {
	path = strings.TrimLeft(path, "/")
	if path == "" && !r.root {
		return r
	}
	var (
		parts       = strings.Split(path, "/")
		currentNode = r
		lOPart      = len(parts)
		i           int
	)
	for i = 0; i < lOPart; i++ {
		matched := false
		pathNode, ptr := resolvePathPart2Node(parts[i]), 0
		for ptr < len(currentNode.children) {
			node := currentNode.children[ptr]
			if pathNode.part == node.part && pathNode.wildcard == node.wildcard {
				matched = true
				if i >= lOPart-1 {
					return node
				}
				currentNode = node
				break
			}
			ptr++
		}
		if !matched {
			break
		}
	}
	for ; i < lOPart; i++ {
		currentNode = currentNode.addChildNode(parts[i])
	}

	return currentNode
}

// addChildNode add a node to child list
// if th path is wildcard, add to end, else add to beginning.
func (r *routeNode) addChildNode(path string) *routeNode {
	node := resolvePathPart2Node(path)
	node.root, node.parent = false, r

	node.fullPath = fmt.Sprintf("%s/%s", r.fullPath, path)
	if node.wildcard {
		r.children = append(r.children, node)

		return node
	}
	r.children = append([]*routeNode{node}, r.children...)

	return node
}

// fit compare the given path with the node path, if match then return true,
// else return false
func (r *routeNode) fit(part string) bool {
	if r.wildcard {
		if r.pattern != nil {
			return r.pattern.MatchString(part)
		}

		return true
	}

	return r.part == part
}

// getHandlers return the handler stored in the node.
// if handler exist, return (handler, true), else return (nil, false)
func (r *routeNode) getHandler(method string) (handleFunc HandleFunc, ok bool) {
	if r.handleFunc == nil {
		return nil, false
	}
	handleFunc, ok = r.handleFunc[method]

	return
}

// setHandlers store a handler with the given method, if the handler in given method already exist,
// panic
func (r *routeNode) setHandler(method string, handleFunc HandleFunc) *routeNode {
	r.leaf = true
	if r.handleFunc == nil {
		r.handleFunc = make(map[string]HandleFunc)
	}
	if _, ok := r.handleFunc[method]; ok {
		panic(fmt.Sprintf("method [%s] on route [%s] already exist", method, r.fullPath))
	}
	r.handleFunc[method] = handleFunc

	return r
}

// resolvePathNode resolve the given path string, if it is a static path,
// return (name, nil, false), if it is a wildcard, return (name, nil, true),
// if it is a wildcard with a pattern, return (name, pattern, true), if the pattern
// compiled failed, panic
func resolvePathPart2Node(pathPart string) *routeNode {
	node := &routeNode{}
	if !strings.HasPrefix(pathPart, ":") {
		node.part, node.rawPattern, node.wildcard = pathPart, "", false

		return node
	}
	pathPart = strings.TrimPrefix(pathPart, ":")
	if !strings.Contains(pathPart, "(") {
		node.part, node.rawPattern, node.wildcard = pathPart, "", true

		return node
	}
	path := pathPart[:strings.Index(pathPart, "(")]
	regStr := pathPart[strings.Index(pathPart, "(")+1 : len(pathPart)-1]
	reg, err := regexp.Compile(regStr)
	if err != nil {
		panic(fmt.Sprintf("resolve path node [%s] failed, error: %s", pathPart, err))
	}
	node.part, node.rawPattern, node.wildcard, node.pattern = path, regStr, true, reg

	return node
}
