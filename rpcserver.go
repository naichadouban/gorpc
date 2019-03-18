package main

import (
	"naichadouban/gorpc/httprpc"
)

// 结构体的字段排列是有要求的，第一个可选字段之后的字段都必须是可选的。

// GetBlockCmd define the getblock JSON-RPC command
type GetBlockCmd struct {
	Hash      string
	Verbose   *bool `jsonrpcdefault:"true"`
	VerboseTx *bool `jsonrpcdefault:"false"`
}

// hex-encoded string.
type GetBlockResult struct {
	Hash   string `json:"hash"`
	Height int    `json:"height"`
}
// handleGetBlock implements the getblock command.
func handleGetBlock(s *httprpc.RpcServer, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	getBlockReply := GetBlockResult{
		Hash:   "fasdfsadfsdfasdf1654654654fasdf",
		Height: 10010,
	}
	mainLog.Infof("handleGetBlock was called")
	return getBlockReply, nil
}

func init() {
	flags := httprpc.UsageFlag(0) //
	// (*GetBlockCmd)(nil) 相当于*GetBlockCmd类型的指针的初始化
	// note:这里是*GetBlockCmd
	httprpc.MustRegisterCmd("getblock", (*GetBlockCmd)(nil), flags)
	httprpc.AddRpcHandler("getblock",handleGetBlock)
}
