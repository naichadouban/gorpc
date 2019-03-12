package gorpc

type commandHandler func(*RpcServer, interface{}, <-chan struct{}) (interface{}, error)

// rpcHandlers maps RPC command strings to appropriate handler functions.
// This is set by init because help references rpcHandlers and thus causes
// a dependency loop.
var rpcHandlers map[string]commandHandler
var rpcHandlersBeforeInit = map[string]commandHandler{
	"addnode":               handleAddNode,

	"version":               handleVersion,
}
// handleAddNode handles addnode commands.
func handleAddNode(s *RpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	llog.Infof("handleAddNode was called")
	return nil,nil
}
// handleAddNode handles addnode commands.
func handleVersion(s *RpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	llog.Infof("handleVersion was called")
	return nil,nil
}