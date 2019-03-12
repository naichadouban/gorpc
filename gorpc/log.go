package gorpc

import (
	"github.com/naichadouban/mylog/mylog"
)

var Llog mylog.Logger // 这个是给子包使用的
var llog mylog.Logger
// UseLogger是在项目根目录下b的log.go文件中调用
func UseLogger(logger mylog.Logger) {
	llog = logger
	Llog = logger
}