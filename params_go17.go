// +build go1.7

package httprouter

import (
	"context"
	"net/http"
)

type paramsKey struct{}

// go 1.7+版本才能使用
var ParamsKey = paramsKey{}

// 将http.HandlerFunc函数适配成请求处理函数，不使用Params参数，利用context包传输变量
// go 1.7+版本才能使用
func (r *Router) Handler(method, path string, handler http.Handler) {
	r.Handle(method, path,
		func(w http.ResponseWriter, req *http.Request, p Params) {
			ctx := req.Context()
			ctx = context.WithValue(ctx, ParamsKey, p)
			req = req.WithContext(ctx)
			handler.ServeHTTP(w, req)
		},
	)
}

// 从请求上下文获取变量值，并且封装成Params
// go 1.7+版本添加context包，这些框架已经不需要Params参数
// go 1.7+版本才能使用
func ParamsFromContext(ctx context.Context) Params {
	p, _ := ctx.Value(ParamsKey).(Params)
	return p
}
