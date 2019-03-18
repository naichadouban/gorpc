package main

import "naichadouban/gorpc/httprpc"

func main() {
	loadConfig()
	rpcServer, err := httprpc.NewRpcServer()
	if err != nil {
		mainLog.Errorf("new rpcserver error:%v", err)
	}
	rpcServer.Start()

}
