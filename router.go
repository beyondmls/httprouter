// httprouter是以前缀树实现的高性能的请求路由
// 从实现原理看只支持简单的请求路径和命名参数，不支持正则表达式
//
//  package main
//
//  import (
//      "fmt"
//      "github.com/julienschmidt/httprouter"
//      "net/http"
//      "log"
//  )
//
//  func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
//      fmt.Fprint(w, "Welcome!\n")
//  }
//
//  func Hello(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
//      fmt.Fprintf(w, "hello, %s!\n", ps.ByName("name"))
//  }
//
//  func main() {
//      router := httprouter.New()
//      router.GET("/", Index)
//      router.GET("/hello/:name", Hello)
//
//      log.Fatal(http.ListenAndServe(":8080", router))
//  }
//
// Router根据请求方法和请求路径匹配处理函数
// GET, POST, PUT, PATCH and DELETE函数是注册路由的快捷方式
// 其他方法使用router.Handle方法注册路由
//
// 支持2种命名参数:
//  :name     named parameter
//  *name     catch-all parameter
//
// Named parameters匹配任何字符直到'/'或者结束:
//  Path: /blog/:category/:post
//
//  Requests:
//   /blog/go/request-routers            match: category="go", post="request-routers"
//   /blog/go/request-routers/           no match, but the router would redirect
//   /blog/go/                           no match
//   /blog/go/request-routers/comments   no match
//
// Catch-all parameters匹配任何字符直到结束
//  Path: /files/*filepath
//
//  Requests:
//   /files/                             match: filepath="/"
//   /files/LICENSE                      match: filepath="/LICENSE"
//   /files/templates/article.html       match: filepath="/templates/article.html"
//   /files                              no match, but the router would redirect
//
// 这些参数的值被存储为Param切片, 作为处理函数的第3个参数
// 可以使用2种方式获取参数的值：
//  user := ps.ByName("user")
//
//  thirdKey   := ps[2].Key
//  thirdValue := ps[2].Value
package httprouter

import (
	"net/http"
)

// Handle函数用于处理请求，类似http.HandlerFunc
type Handle func(http.ResponseWriter, *http.Request, Params)

// Param是URL解析之后的单个变量，由键值对组成
type Param struct {
	Key   string
	Value string
}

type Params []Param

// 根据变量名字获取变量值，如果没有匹配的返回空字符串
func (ps Params) ByName(name string) string {
	for i := range ps {
		if ps[i].Key == name {
			return ps[i].Value
		}
	}
	return ""
}

// Router是http.Handler的实现，通过配置分发请求到不同的处理函数
type Router struct {
	// 前缀树，用于存储路由配置信息
	trees map[string]*node

	// 配置是否自动重定向带斜杠的请求：
	// 如果只存在/foo的路由信息，/foo/重定向到/foo
	RedirectTrailingSlash bool
	// 配置是否自动重定向待修复的请求：
	// 先去除路径的多于元素，例如： ../ //
	// 再转换路径的大小写，例如：/FOO /..//Foo
	RedirectFixedPath bool
	// 配置是否检测存在相同路径但是不同方法的路由
	// 如果存在返回Method Not Allowed，否则返回NotFound
	HandleMethodNotAllowed bool
	// 配置是否自动回复OPTIONS请求
	HandleOPTIONS bool

	// 没有匹配的路由的时候，可以注册1个处理函数
	NotFound http.Handler
	// 存在匹配的路由，但是方法不匹配的时候，可以注册1个处理函数
	MethodNotAllowed http.Handler
	// 请求处理出现异常，可以注册1个处理函数
	PanicHandler func(http.ResponseWriter, *http.Request, interface{})
}

// 保证Router实现了http.Handler接口
var _ http.Handler = New()

// 使用默认配置新建Router
func New() *Router {
	return &Router{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      true,
		HandleMethodNotAllowed: true,
		HandleOPTIONS:          true,
	}
}

func (r *Router) GET(path string, handle Handle) {
	r.Handle("GET", path, handle)
}

func (r *Router) HEAD(path string, handle Handle) {
	r.Handle("HEAD", path, handle)
}

func (r *Router) OPTIONS(path string, handle Handle) {
	r.Handle("OPTIONS", path, handle)
}

func (r *Router) POST(path string, handle Handle) {
	r.Handle("POST", path, handle)
}

func (r *Router) PUT(path string, handle Handle) {
	r.Handle("PUT", path, handle)
}

func (r *Router) PATCH(path string, handle Handle) {
	r.Handle("PATCH", path, handle)
}

func (r *Router) DELETE(path string, handle Handle) {
	r.Handle("DELETE", path, handle)
}

// 根据请求方法和请求路径注册新的路由
func (r *Router) Handle(method, path string, handle Handle) {
	if path[0] != '/' {
		panic("path must begin with '/' in path '" + path + "'")
	}

	if r.trees == nil {
		r.trees = make(map[string]*node)
	}

	// 树的第1级为请求方法，GET->node
	root := r.trees[method]
	if root == nil {
		root = new(node)
		r.trees[method] = root
	}

	root.addRoute(path, handle)
}

// 将http.HandlerFunc函数适配成请求处理函数
func (r *Router) HandlerFunc(method, path string, handler http.HandlerFunc) {
	r.Handler(method, path, handler)
}

// 处理文件资源请求，例如：本地目录为/etc，请求路径为/passwd，那么本地文件为/etc/passwd
// 使用 http.FileServer实现，因此不会使用router注册的NotFound函数，直接使用http.NotFound
// 简单的例子，path必须以*filepath结束:
//     router.ServeFiles("/src/*filepath", http.Dir("/var/www"))
func (r *Router) ServeFiles(path string, root http.FileSystem) {
	if len(path) < 10 || path[len(path)-10:] != "/*filepath" {
		panic("path must end with /*filepath in path '" + path + "'")
	}

	// 直接使用http.FileServer作为处理函数
	fileServer := http.FileServer(root)

	r.GET(path, func(w http.ResponseWriter, req *http.Request, ps Params) {
		// filepath是约定的参数名
		req.URL.Path = ps.ByName("filepath")
		fileServer.ServeHTTP(w, req)
	})
}

// 捕获异常进行处理的函数
func (r *Router) recv(w http.ResponseWriter, req *http.Request) {
	if rcv := recover(); rcv != nil {
		r.PanicHandler(w, req, rcv)
	}
}

// 根据请求方法和路径查找对应的处理函数
func (r *Router) Lookup(method, path string) (Handle, Params, bool) {
	if root := r.trees[method]; root != nil {
		return root.getValue(path)
	}
	return nil, nil, false
}

// 判断请求方法和路径是否被允许处理
func (r *Router) allowed(path, reqMethod string) (allow string) {
	// 路径 * 代表都请求方法和路径都被允许
	if path == "*" {
		for method := range r.trees {
			if method == "OPTIONS" {
				continue
			}

			if len(allow) == 0 {
				allow = method
			} else {
				allow += ", " + method
			}
		}
	} else {
		for method := range r.trees {
			if method == reqMethod || method == "OPTIONS" {
				continue
			}

			// 根据请求方法和路径在前缀树查找处理函数
			handle, _, _ := r.trees[method].getValue(path)
			if handle != nil {
				if len(allow) == 0 {
					allow = method
				} else {
					allow += ", " + method
				}
			}
		}
	}
	if len(allow) > 0 {
		allow += ", OPTIONS"
	}
	return
}

// ServeHTTP函数实现了http.Handler接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// 请求处理出现异常，可以注册1个处理函数
	if r.PanicHandler != nil {
		defer r.recv(w, req)
	}

	path := req.URL.Path

	// 如果根据请求方法获取到前缀树，从前缀树查找对应的处理函数
	if root := r.trees[req.Method]; root != nil {
		if handle, ps, tsr := root.getValue(path); handle != nil {
			handle(w, req, ps)
			return
		} else if req.Method != "CONNECT" && path != "/" {
			// Permanent redirect
			code := 301
			if req.Method != "GET" {
				// Temporary redirect
				code = 307
			}

			// 自动重定向带斜杠的请求：
			// 如果只存在/foo的路由信息，/foo/重定向到/foo
			if tsr && r.RedirectTrailingSlash {
				if len(path) > 1 && path[len(path)-1] == '/' {
					req.URL.Path = path[:len(path)-1]
				} else {
					req.URL.Path = path + "/"
				}
				http.Redirect(w, req, req.URL.String(), code)
				return
			}

			// 自动重定向待修复的请求：
			// 先去除路径的多于元素，例如： ../ //
			// 再转换路径的大小写，例如：/FOO /..//Foo
			if r.RedirectFixedPath {
				fixedPath, found := root.findCaseInsensitivePath(
					CleanPath(path),
					r.RedirectTrailingSlash,
				)
				if found {
					req.URL.Path = string(fixedPath)
					http.Redirect(w, req, req.URL.String(), code)
					return
				}
			}
		}
	}

	// 如果配置自动处理OPTIONS请求
	if req.Method == "OPTIONS" && r.HandleOPTIONS {
		if allow := r.allowed(path, req.Method); len(allow) > 0 {
			w.Header().Set("Allow", allow)
			return
		}
	} else {
		// 处理Method Not Allowed（405）
		if r.HandleMethodNotAllowed {
			if allow := r.allowed(path, req.Method); len(allow) > 0 {
				w.Header().Set("Allow", allow)
				if r.MethodNotAllowed != nil {
					r.MethodNotAllowed.ServeHTTP(w, req)
				} else {
					http.Error(w,
						http.StatusText(http.StatusMethodNotAllowed),
						http.StatusMethodNotAllowed,
					)
				}
				return
			}
		}
	}

	// 处理Not Found（404）
	if r.NotFound != nil {
		r.NotFound.ServeHTTP(w, req)
	} else {
		http.NotFound(w, req)
	}
}
