package main

import "naichadouban/gorpc/gorpc"

func main() {
	loadConfig()
	rpcServer,err  := gorpc.NewRpcServer()
	if err != nil {
		mainLog.Errorf("new rpcserver error:%v",err)
	}
	rpcServer.Start()

}
