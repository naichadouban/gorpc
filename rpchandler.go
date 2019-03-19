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
	rpcHandlers[method] = handler
}

func handleGetReadMe(s *RpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	rlog.Debugf("getReadMe was called:%v", cmd)
	readme := GetReadMeReasult{
		"这是从hcd中抽取出来的一个json-rpc，通过http、websocket传输",
	}
	return readme, nil
}
func init() {
	rand.Seed(time.Now().UnixNano())
	flags := UsageFlag(0) //
	// (*GetReadMeCmd)(nil) 相当于*GetReadMeCmd类型的指针的初始化
	MustRegisterCmd("getreadme", (*GetReadMeCmd)(nil), flags)
}
