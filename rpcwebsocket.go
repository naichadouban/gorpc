package gorpc

import (
	"encoding/json"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io"
	"sync"
	"time"
)

// semaphore [信号]
type semaphore chan struct{}

func makeSemaphore(n int) semaphore {
	return make(chan struct{}, n)
}
func (s semaphore) acquire() {
	s <- struct{}{}
}
func (s semaphore) release() {
	<-s
}

// wsClient provides an abstraction for handling a websocket client.  The
// overall data flow is split into 3 main goroutines, a possible 4th goroutine
// for long-running operations (only started if request is made), and a
// websocket manager which is used to allow things such as broadcasting
// requested notifications to all connected websocket clients.   Inbound
// messages are read via the inHandler goroutine and generally dispatched to
// their own handler.  However, certain potentially long-running operations such
// as rescans, are sent to the asyncHander goroutine and are limited to one at a
// time.  There are two outbound message types - one for responding to client
// requests and another for async notifications.  Responses to client requests
// use SendMessage which employs a buffered channel thereby limiting the number
// of outstanding requests that can be made.  Notifications are sent via
// QueueNotification which implements a queue via notificationQueueHandler to
// ensure sending notifications from other subsystems can't block.  Ultimately,
// all messages are sent via the outHandler.
type wsClient struct {
	sync.Mutex

	// server is the RPC server that is servicing the client.
	server *RpcServer
	// conn is the underlying websocket connection.
	conn *websocket.Conn
	// disconnected indicated whether or not the websocket client is
	// disconnected.
	disconnected bool
	// addr is the remote address of the client.
	addr string
	// authenticated specifies whether a client has been authenticated
	// and therefore is allowed to communicated over the websocket.
	authenticated bool
	// isAdmin specifies whether a client may change the state of the server;
	// false means its access is only to the limited set of RPC calls.
	isAdmin	bool
	// sessionID is a random ID generated for each client when connected.
	// These IDs may be queried by a client using the session RPC.  A change
	// to the session ID indicates that the client reconnected.
	sessionID	uint64

	// verboseTxUpdates specifies whether a client has requested verbose
	// information about all new transactions.
	verboseTxUpdates bool
	ntfnChan         chan []byte // 发送的消息都是先发送到这里
	// addrRequests is a set of addresses the caller has requested to be
	// notified about.  It is maintained here so all requests can be removed
	// when a wallet disconnects.  Owned by the notification manager.
	addrRequests      map[string]struct{}
	serviceRequestSem semaphore
	quit              chan struct{}
	wg                sync.WaitGroup
}

// wsNotificationManager is a connection and notification manager used for
// websockets.  It allows websocket clients to register for notifications they
// are interested in.  When an event happens elsewhere in the code such as
// transactions being added to the memory pool or block connects/disconnects,
// the notification manager is provided with the relevant details needed to
// figure out which websocket clients need to be notified based on what they
// have registered for and notifies them accordingly.  It is also used to keep
// track of all connected websocket clients.
type wsNotificationManager struct {
	// server is the RPC server the notification manager is associated with.
	server *RpcServer

	// queueNotification queues a notification for handling.
	queueNotification chan interface{}

	// notificationMsgs feeds notificationHandler with notifications
	// and client (un)registeration requests from a queue as well as
	// registeration and unregisteration requests from clients.
	notificationMsgs chan interface{}

	// Access channel for current number of connected clients.
	numClients chan int

	// Shutdown handling
	wg   sync.WaitGroup
	quit chan struct{}
}

// AddClient adds the passed websocket client to the notification manager.
func (m *wsNotificationManager) AddClient(wsc *wsClient) {
	m.queueNotification <- (*notificationRegisterClient)(wsc)
}

// timeZeroVal is simply the zero value for a time.Time and is used to avoid
// creating multiple instances.
var timeZeroVal time.Time
// WebsocketHandler handles a new websocket client by creating a new wsClient,
// starting it, and blocking until the connection closes.  Since it blocks, it
// must be run in a separate goroutine.  It should be invoked from the websocket
// server handler which runs each new connection in a new goroutine thereby
// satisfying the requirement.
func (rs *RpcServer) WebsocketHandler(conn *websocket.Conn, remoteAddr string, authenticated bool, isAdmin bool) {
	conn.SetReadDeadline(timeZeroVal)
	// Limit max number of websocket clients.
	rlog.Infof("New websocket client %s", remoteAddr)
	// TODO  Limit max number of websocket clients.
	// Create a new websocket client to handle the new websocket connection
	// and wait for it to shutdown.  Once it has shutdown (and hence
	// disconnected), remove it and any notifications it registered for.
	client, err := newWebsocketClient(rs, conn, remoteAddr, true, false)
	if err != nil {
		rlog.Errorf("Failed to serve client %s: %v", remoteAddr, err)
		conn.Close()
		return
	}
	//rs.ntfnMgr.AddClient(client)
	client.
}

// newWebsocketClient returns a new websocket client given the notification
// manager, websocket connection, remote address, and whether or not the client
// has already been authenticated (via HTTP Basic access authentication).  The
// returned client is ready to start.  Once started, the client will process
// incoming and outgoing messages in separate goroutines complete with queuing
// and asynchrous handling for long-running operations.
func newWebsocketClient(server *RpcServer, conn *websocket.Conn,
	remoteAddr string, authenticated bool, isAdmin bool) (*wsClient, error) {
	// TODO
	sessionID := uuid.New().ID()
	client := &wsClient{
		conn:      conn,
		addr:      remoteAddr,
		sessionID: uint64(sessionID),
		server:    server,
	}
	return client, nil
}

func (c *wsClient) Start() {
	rlog.Tracef("Starting websocket client %s", c.addr)
	c.wg.Add(3)
	go c.inHandler()
	go c.notificationQueueHandler()
	go c.outHandler()
	// note: 这里并没有c.wg.Wait方法，所以才需要下一个方法
}
func (c *wsClient) WaitForShutdown() {
	c.wg.Wait()
}
func (c *wsClient) inHandler() {
out:
	for {
		// 一旦quit channel关闭就结束循环
		// 使用非阻塞（non-blocking）select
		select {
		case <-c.quit:
			break out
		default:

		}
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			// 如果不是由于断开连接的话，就打印错误
			if err != io.EOF {
				rlog.Errorf("WebSocket receive error from %s:%v", c.addr, err)
			}
			break out
		}
		var request Request // 这里和json-rpc over http 的request是相同的
		err = json.Unmarshal(msg, &request)
		if err != nil {
			if !c.authenticated {
				break out
			}

			jsonErr := &btcjson.RPCError{
				Code:    btcjson.ErrRPCParse.Code,
				Message: "Failed to parse request: " + err.Error(),
			}
			reply, err := createMarshalledReply(nil, nil, jsonErr)
			if err != nil {
				rlog.Errorf("Failed to marshal parse failure "+
					"reply: %v", err)
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}
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
		if request.ID == nil && !(c.server.Config.RPCQuirks && request.Jsonrpc == "") {
			if !c.authenticated {
				break out
			}
			continue
		}
		cmd := parseCmd(&request)
		if cmd.Err != nil {
			if !c.authenticated {
				break out
			}

			reply, err := createMarshalledReply(cmd.Id, nil, cmd.Err)
			if err != nil {
				rlog.Errorf("Failed to marshal parse failure "+
					"reply: %v", err)
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}
		rlog.Debugf("Received command <%s> from %s", cmd.Method, c.addr)
		// 检查身份验证
		// client将会立即断开，如果没有认证websocket client的第一个请求不是一个验证的请求,
		// 或者在请求中没有提供身份验证凭据
		// 只有当client已经验证通过，验证的请求的才会被接受
		switch authCmd, ok := cmd.Cmd.(*AuthenticateCmd); {
		// 这个client已经认证过了，又来认证
		case c.authenticated && ok:
			rlog.Warnf("websocket client %s is already authenticated", c.addr)
			break out
			// 没有认证过，这次还不认证
		case !c.authenticated && !ok:
			rlog.Warnf("Unauthenticated websocket message received")
			break out
			// 没有认证过
		case !c.authenticated:
			// TODO 这里还是认证
			c.authenticated = true
			c.isAdmin = true
			// marshal and send response
			reply, err := createMarshalledReply(cmd.Id, nil, nil)
			if err != nil {
				rlog.Errorf("Failed to marshal authenticate reply: "+
					"%v", err.Error())
				continue
			}
			c.SendMessage(reply, nil)
			continue
		}
		// TODO Check if the client is using limited RPC credentials and
		// error when not authorized to call this RPC.

		// TODO serviceRequestSem是限制连接的请求数的吗？
		c.serviceRequestSem.acquire()
		go func() {
			c.serviceRequest(cmd)
			c.serviceRequestSem.release()
		}()
	}
	// Ensure the connection is closed.
	c.Disconnect()
	c.wg.Done()
	rlog.Tracef("Websocket client input handler done for %s", c.addr)
}

// serviceRequest services a parsed RPC request by looking up and executing the
// appropriate RPC handler.  The response is marshalled and sent to the
// websocket client.
func (c *wsClient) serviceRequest(r *ParsedRPCCmd) {
	var result interface{}
	var err error
	// Lookup the websocket extension for the command and if it doesn't
	// exist fallback to handling the command as a standard command.
	wsHandler, ok := wsHandlers[r.Method]
	if ok {
		result, err = wsHandler(c, r.Cmd)
	} else {
		result, err = c.server.standardCmdResult(r, nil)
	}
	reply, err := createMarshalledReply(r.Id, result, err)
	if err != nil {
		rlog.Errorf("Failed to marshal reply for <%s> "+
			"command: %v", r.Method, err)
		return
	}
	c.SendMessage(reply, nil)
}

// SendMessage sends the passed json to the websocket client.  It is backed
// by a buffered channel, so it will not block until the send channel is full.
// Note however that QueueNotification must be used for sending async
// notifications instead of the this function.  This approach allows a limit to
// the number of outstanding requests a client can make without preventing or
// blocking on async notifications.
func (c *wsClient) SendMessage(marshalledJSON []byte, doneChan chan bool) error {
	if c.Disconnected() {
		return ErrClientQuit
	}
	c.ntfnChan <- marshalledJSON
	return nil
}

// Disconnected returns whether or not the websocket client is disconnected.
func (c *wsClient) Disconnected() bool {
	c.Lock()
	isDisconnected := c.disconnected
	c.Unlock()

	return isDisconnected
}

// Disconnect disconnects the websocket client.
func (c *wsClient) Disconnect() {
	c.Lock()
	defer c.Unlock()

	// Nothing to do if already disconnected.
	if c.disconnected {
		return
	}

	rlog.Tracef("Disconnecting websocket client %s", c.addr)
	close(c.quit)
	c.conn.Close()
	c.disconnected = true
}
