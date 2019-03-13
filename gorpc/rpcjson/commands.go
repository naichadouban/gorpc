package rpcjson

// GetBlockCmd define the getblock JSON-RPC command
type GetBlockCmd struct {
	Hash string
	Verbose *bool `jsonrpcdefault:"true"`
	VerboseTx *bool `jsonrpcdefault:"false"`
}

func init()  {
	flags := UsageFlag(0)
	// (*GetBlockCmd)(nil) 相当于GetBlockCmd类型的指针的初始化
	MustRegisterCmd("getblock", (*GetBlockCmd)(nil), flags)
}
