package gorpc

import (
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcd/btcjson"
	"io"
	"io/ioutil"
	"log"
	"naichadouban/gorpc/gorpc/rpcjson"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"sync"
)

type RpcConfig struct {
	RPCQuirks bool
}
type RpcServer struct {
	started     int32
	config      RpcConfig
	statusLock  sync.RWMutex
	statusLines map[int]string
}
func NewRpcServer()(*RpcServer,error){
	rpc := &RpcServer{
		statusLines:make(map[int]string),
	}
	return rpc,nil
}

func (rs *RpcServer) Start() {
	rpcServeMux := http.NewServeMux()

	rpcServeMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "close")
		w.Header().Set("Connect-Type", "application/json")
		r.Close = true
		// Read and respond to the request.
		rs.jsonRPCRead(w, r)
	})
	httpServer := http.Server{
		Handler: rpcServeMux,
	}
	listen, err := net.Listen("tcp", ":8009")
	if err != nil {
		log.Panicf("net listen error:%v", err)
	}
	rlog.Infof("rpc server listen :%v", listen.Addr())
	httpServer.Serve(listen)

}
func (rs *RpcServer) jsonRPCRead(w http.ResponseWriter, r *http.Request) {
	// 打印出请求信息
	byteReq, err := httputil.DumpRequest(r, true)
	if err != nil {
		fmt.Println(err)
	}
	rlog.Infof("receive request:%v", string(byteReq))
	// 读取body信息
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		errCode := http.StatusBadRequest
		http.Error(w, fmt.Sprintf("%d error reading json message:%v", errCode, err), errCode)
	}
	//获得底层的 TCP 连接，这样才能转发数据，
	// 所以下面会有 Hijacker 类型转换和 Hijack() 调用，
	// 它们最终的目的是拿到客户端的 TCP 连接（net.TCPConn）
	hj, ok := w.(http.Hijacker)
	if !ok {
		errMsg := "webserver does not support hijacking"
		rlog.Error(errMsg)
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+" "+errMsg, errCode)
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		rlog.Errorf("Failed to hijack HTTP connection: %v", err)
		errCode := http.StatusInternalServerError
		http.Error(w, strconv.Itoa(errCode)+""+err.Error(), errCode)
	}
	defer conn.Close()
	defer buf.Flush()
	// TODO conn.SetReadDeadline(timeZeroVal)
	// 把body信息解析成JOSN-RPC requests
	var responseID interface{}
	var jsonErr error
	var result interface{} // 处理后的结果
	var request rpcjson.Request
	if err := json.Unmarshal(body, &request); err != nil {
		jsonErr = rpcjson.RPCError{
			Code:    rpcjson.ErrRPCParse.Code,
			Message: "Failed to parse request: " + err.Error(),
		}
	}
	rlog.Debugf("After Unmarshal get request:%v", request)
	if jsonErr == nil {
		// json-rpc 1.0规范：通知必须将字段id设置为null。通知是不需要response的
		// json-rpc 2.0规范：通知的request必须有`json-rpc`字段，并且没有id字段。
		// 2.0 规定通知的话一定不要回复。
		// 2.0规范容许id值设置为null，因此即使id为null，也不是一个通知。（通知直接就没有id，上面刚说了）

		// Bitcoin Core 中，如果请求的id为null，或者没有id字段，也会有回应，
		// response中id字段的值也为null

		// Btcd中，任何请求如果没有id字段或者id值为null，都不会回应，而不管json-rpc协议版本。除非
		// rpc quirks是允许的。RPCQuirks 就是一个字段。
		// 	RPCQuirks            bool          `long:"rpcquirks" description:"Mirror some JSON-RPC quirks of Bitcoin Core -- NOTE: Discouraged unless interoperability issues need to be worked around"`
		// 如果RPC quirks允许，这样的请求也会回应，如果请求没有指定json-rpc版本

		if request.ID == nil && (rs.config.RPCQuirks && request.Jsonrpc == "") {
			return
		}
		// 到这里解析至少是成功的，设置response的ID
		responseID = request.ID
		// 设置close通知。因为这个连接已经被Hijacked，在ResponseWriter上的关闭是无效的
		closeChan := make(chan struct{})
		go func() {
			_, err := conn.Read(make([]byte, 1))
			if err != nil {
				close(closeChan)
			}
		}()
		// TODO 检查用户是否有限制
		if jsonErr == nil {
			// 把json-rpc请求request解析成一个具体的command
			parsedCmd := parseCmd(&request)
			if parsedCmd.Err != nil {
				jsonErr = parsedCmd.Err
			} else {
				result, jsonErr = rs.standardCmdResult(parsedCmd, closeChan)
			}
		}
	}
	// Marshal the response.
	msg, err := createMarshalledReply(responseID, result, jsonErr)
	if err != nil {
		rlog.Errorf("Failed to marshal reply: %v", err)
		return
	}

	// Write the response.
	err = rs.writeHTTPResponseHeaders(r, w.Header(), http.StatusOK, buf)
	if err != nil {
		rlog.Error(err)
		return
	}
	if _, err := buf.Write(msg); err != nil {
		rlog.Errorf("Failed to write marshalled reply: %v", err)
	}

	// Terminate with newline to maintain compatibility with Bitcoin Core.
	if err := buf.WriteByte('\n'); err != nil {
		rlog.Errorf("Failed to append terminating newline to reply: %v", err)
	}

}

// parsedRPCCmd represents a JSON-RPC request object that has been parsed into
// a known concrete command along with any error that might have happened while
// parsing it.
type ParsedRPCCmd struct {
	Id     interface{}       `json:"id"`
	Method string            `json:"method"`
	Cmd    interface{}       `json:"cmd"`
	Err    *rpcjson.RPCError `json:"err"`
}

func parseCmd(request *rpcjson.Request) *ParsedRPCCmd {
	var parsedCmd ParsedRPCCmd
	parsedCmd.Id = request.ID
	parsedCmd.Method = request.Method
	cmd, err := rpcjson.UnmarshalCmd(request)
	if err != nil {
		rlog.Infof("rpcjson.UnmarshalCmd error:%v", err)
	}
	parsedCmd.Cmd = cmd
	return &parsedCmd

}

// standardCmdResult checks that a parsed command is a standard Bitcoin JSON-RPC
// command and runs the appropriate handler to reply to the command.  Any
// commands which are not recognized or not implemented will return an error
// suitable for use in replies.
func (s *RpcServer) standardCmdResult(cmd *ParsedRPCCmd, closeChan <-chan struct{}) (interface{}, error) {
	handler, ok := rpcHandlers[cmd.Method]
	if ok {
		goto handled
	}
	// TODO
	//_, ok = rpcUnimplemented[cmd.method]
	//if ok {
	//	handler = handleUnimplemented
	//	goto handled
	//}
	//return nil, btcjson.ErrRPCMethodNotFound
handled:

	return handler(s, cmd.Cmd, closeChan)
}

// writeHTTPResponseHeaders writes the necessary response headers prior to
// writing an HTTP body given a request to use for protocol negotiation, headers
// to write, a status code, and a writer.
func (s *RpcServer) writeHTTPResponseHeaders(req *http.Request, headers http.Header, code int, w io.Writer) error {
	_, err := io.WriteString(w, s.httpStatusLine(req, code))
	if err != nil {
		return err
	}

	err = headers.Write(w)
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, "\r\n")
	return err
}

// httpStatusLine returns a response Status-Line (RFC 2616 Section 6.1)
// for the given request and response status code.  This function was lifted and
// adapted from the standard library HTTP server code since it's not exported.
func (rs *RpcServer) httpStatusLine(req *http.Request, code int) string {
	// Fast path:
	key := code
	proto11 := req.ProtoAtLeast(1, 1)
	if !proto11 {
		key = -key
	}
	rs.statusLock.RLock()
	line, ok := rs.statusLines[key]
	rs.statusLock.RUnlock()
	if ok {
		return line
	}

	// Slow path:
	proto := "HTTP/1.0"
	if proto11 {
		proto = "HTTP/1.1"
	}
	codeStr := strconv.Itoa(code)
	text := http.StatusText(code)
	if text != "" {
		line = proto + " " + codeStr + " " + text + "\r\n"
		rs.statusLock.Lock()
		rs.statusLines[key] = line
		rs.statusLock.Unlock()
	} else {
		text = "status code " + codeStr
		line = proto + " " + codeStr + " " + text + "\r\n"
	}

	return line
}

// createMarshalledReply returns a new marshalled JSON-RPC response given the
// passed parameters.  It will automatically convert errors that are not of
// the type *btcjson.RPCError to the appropriate type as needed.
func createMarshalledReply(id, result interface{}, replyErr error) ([]byte, error) {
	var jsonErr *btcjson.RPCError
	if replyErr != nil {
		if jErr, ok := replyErr.(*btcjson.RPCError); ok {
			jsonErr = jErr
		} else {
			jsonErr = internalRPCError(replyErr.Error(), "")
		}
	}

	return btcjson.MarshalResponse(id, result, jsonErr)
}

// internalRPCError is a convenience function to convert an internal error to
// an RPC error with the appropriate code set.  It also logs the error to the
// RPC server subsystem since internal errors really should not occur.  The
// context parameter is only used in the log message and may be empty if it's
// not needed.
func internalRPCError(errStr, context string) *btcjson.RPCError {
	logStr := errStr
	if context != "" {
		logStr = context + ": " + errStr
	}
	rlog.Error(logStr)
	return btcjson.NewRPCError(btcjson.ErrRPCInternal.Code, errStr)
}
