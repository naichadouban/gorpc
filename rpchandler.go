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

func AddRpcHandler(method string, handler commandHandler) {
	if _,ok := rpcHandlers[method];ok{
		rlog.Errorf("http rpc method %q is already registered", method)
		return
	}
	rpcHandlers[method] = handler
}

func handleGetReadMe(s *RpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	rlog.Debugf("getReadMe was called:%v", cmd)
	readme := GetReadMeReasult{
		"这是从hcd中抽取出来的一个json-rpc，通过http、websocket传输",
	}
	return readme, nil
}
//====================上面是over http=========下面是over websocket===================
// wsCommandHandler describes a callback function used to handle a specific
// command.
type wsCommandHandler func(*wsClient, interface{}) (interface{}, error)

// wsHandlers maps RPC command strings to appropriate websocket handler
// functions.  This is set by init because help references wsHandlers and thus
// causes a dependency loop.

var wsHandlers = map[string]wsCommandHandler{
	"help":                      handleWebsocketHelp,
}
// handleWebsocketHelp implements the help command for websocket connections.
func handleWebsocketHelp(wsc *wsClient, icmd interface{}) (interface{}, error) {
	help:="json-rpc通过websocket，这相当于是个example"
	return help, nil
}
func AddWsRpcHandler(method string,wsHandler wsCommandHandler){
	// method是否已经注册
	if _, ok := wsHandlers[method]; ok {
		rlog.Errorf("websocket method %q is already registered", method)
		return
	}
	wsHandlers[method]=wsHandler
}
//==========================websocket 结束 ============================
func init() {
	rand.Seed(time.Now().UnixNano())
	flags := UsageFlag(0) //
	// (*GetReadMeCmd)(nil) 相当于*GetReadMeCmd类型的指针的初始化
	MustRegisterCmd("getreadme", (*GetReadMeCmd)(nil), flags)
}
