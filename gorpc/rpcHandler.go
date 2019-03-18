package gorpc

import (
	"math/rand"
	"time"
)

// getReadMe指令，此指令不需要注册
type GetReadMeCmd struct{}

// getReadMe指令的返回值
type GetReadMeReasult struct {
	Info string `json:"info"`
}

// 需要chan，处理完成后通知断开Hijack()之后的链接
type commandHandler func(*RpcServer, interface{}, <-chan struct{}) (interface{}, error)

var rpcHandlers = map[string]commandHandler{
	"getreadme": handleGetReadMe,
}
func AddRpcHandler(method string,handler commandHandler){
	rpcHandlers[method]=handler
}

func handleGetReadMe(s *RpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	rlog.Debugf("getReadMe was called:%v", cmd)
	readme := GetReadMeReasult{
		Info:"這是我自己仿照btcd實現的json-rpc，readme只是一個測試方法",
	}
	return readme, nil
}
func init() {
	rand.Seed(time.Now().UnixNano())
}
