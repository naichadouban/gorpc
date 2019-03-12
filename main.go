package main

import "naichadouban/gorpc/gorpc"

func main() {
	loadConfig()
	rpcServer := new(gorpc.RpcServer)
	rpcServer.Start()

}
