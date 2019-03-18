package rpcjson

import (
	"github.com/naichadouban/mylog/mylog"
)

var llog mylog.Logger
// UseLogger函数在项目根目录下的log.go文件中调用
func UseLogger(logger mylog.Logger) {
	llog = logger
}