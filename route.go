package httprouter

import (
	"fmt"
	"regexp"
	"strings"
)

type RouteNode struct {
	parent     *RouteNode
	children   []*RouteNode
	root       bool
	leaf       bool
	path       string
	fullPath   string
	routeName  string
	pattern    *regexp.Regexp
	rawPattern string
	wildcard   bool
	handleFunc map[string]HandleFunc
}

func (routeNode *RouteNode) RouteName() string {
	return routeNode.routeName
}

func (routeNode *RouteNode) FullPath() string {
	return routeNode.fullPath
}

// search the given path, if find, return matched node and a params,
// if not, return nil, nil.
func (routeNode *RouteNode) search(path string, getParams func() *Params) (*RouteNode, *Params) {
	//split given path to a slice and make a slice of collection.Iterator with the capacity
	//of the len of path slice. each path item will go through a list of entry, if match then go to
	//find in this entry's children list. if there is no match in current list, go upper to
	//find another entry and so on. if no match at all, not found.
	var (
		l        = strings.Split(path, "/")
		ptrStore = make([]int, len(l))
		k        = routeNode
		at, ptr  = 1, 0
		ps       = getParams()
	)
	// beautyPath always start with '/' and the begin of the path must be "".
	// search from root entry's children and skip the begin of path slice.
	// it means that root must match with the begin of path slice.
	for at >= 1 && at < len(l) {
		find := false
		for ptr < len(k.children) {
			child := k.children[ptr]
			// if find fit node
			if child.fit(l[at]) {
				// push Iterator to same position
				ptrStore[at] = ptr
				find = true
				k = child
				//save the params if necessary
				if child.wildcard {
					*ps = append(*ps, Param{key: child.path, value: l[at]})
				}
				// then we need to go further
				ptr = 0
				at++
				// but if end, we can not
				if at >= len(l) {
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
func (routeNode *RouteNode) addPath(name, method, path string, handler HandleFunc) *RouteNode {
	node := routeNode.pave(path)
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
				case sibling.wildcard && sibling.pattern != nil && sibling.pattern.MatchString(node.path):
					panic(conflict)
				case node.wildcard && node.pattern != nil && node.pattern.MatchString(sibling.path):
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
func (routeNode *RouteNode) pave(path string) *RouteNode {
	path = strings.TrimLeft(path, "/")
	if path == "" && !routeNode.root {
		return routeNode
	}
	var (
		l           = strings.Split(path, "/")
		currentNode = routeNode
		lol         = len(l)
		i           int
	)
	for i = 0; i < lol; i++ {
		matched := false
		pathNode, ptr := resolvePathSplit2Node(l[i]), 0
		for ptr < len(currentNode.children) {
			node := currentNode.children[ptr]
			if pathNode.path == node.path && pathNode.wildcard == node.wildcard {
				matched = true
				if i >= lol-1 {
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
	for ; i < lol; i++ {
		currentNode = currentNode.addChildNode(l[i])
	}

	return currentNode
}

// addChildNode add a node to child list
// if th path is wildcard, add to end, else add to beginning.
func (routeNode *RouteNode) addChildNode(path string) *RouteNode {
	node := resolvePathSplit2Node(path)
	node.root, node.parent = false, routeNode

	node.fullPath = fmt.Sprintf("%s/%s", routeNode.fullPath, path)
	if node.wildcard {
		routeNode.children = append(routeNode.children, node)

		return node
	}
	routeNode.children = append([]*RouteNode{node}, routeNode.children...)

	return node
}

// fit compare the given path with the node path, if match then return true,
// else return false
func (routeNode *RouteNode) fit(path string) bool {
	if routeNode.wildcard {
		if routeNode.pattern != nil {
			return routeNode.pattern.MatchString(path)
		}

		return true
	}

	return routeNode.path == path
}

// getHandlers return the handler stored in the node.
// if handler exist, return (handler, true), else return (nil, false)
func (routeNode *RouteNode) getHandler(method string) (handleFunc HandleFunc, ok bool) {
	if routeNode.handleFunc == nil {
		return nil, false
	}
	handleFunc, ok = routeNode.handleFunc[method]

	return
}

// setHandlers store a handler with the given method, if the handler in given method already exist,
// panic
func (routeNode *RouteNode) setHandler(method string, handleFunc HandleFunc) *RouteNode {
	routeNode.leaf = true
	if routeNode.handleFunc == nil {
		routeNode.handleFunc = make(map[string]HandleFunc)
	}
	if _, ok := routeNode.handleFunc[method]; ok {
		panic(fmt.Sprintf("method [%s] on route [%s] already exist", method, routeNode.fullPath))
	}
	routeNode.handleFunc[method] = handleFunc

	return routeNode
}

// resolvePathNode resolve the given path string, if it is a static path,
// return (name, nil, false), if it is a wildcard, return (name, nil, true),
// if it is a wildcard with a pattern, return (name, pattern, true), if the pattern
// compiled failed, panic
func resolvePathSplit2Node(pathSplit string) *RouteNode {
	node := &RouteNode{}
	if !strings.HasPrefix(pathSplit, ":") {
		node.path, node.rawPattern, node.wildcard = pathSplit, "", false

		return node
	}
	pathSplit = strings.TrimPrefix(pathSplit, ":")
	if !strings.Contains(pathSplit, "(") {
		node.path, node.rawPattern, node.wildcard = pathSplit, "", true

		return node
	}
	path := pathSplit[:strings.Index(pathSplit, "(")]
	regStr := pathSplit[strings.Index(pathSplit, "(")+1 : len(pathSplit)-1]
	reg, err := regexp.Compile(regStr)
	if err != nil {
		panic(fmt.Sprintf("resolve path node [%s] failed, error: %s", pathSplit, err))
	}
	node.path, node.rawPattern, node.wildcard, node.pattern = path, regStr, true, reg

	return node
}
