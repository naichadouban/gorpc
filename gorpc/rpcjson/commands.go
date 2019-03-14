package rpcjson

// 结构体的字段排列是有要求的，第一个可选字段之后的字段都必须是可选的。

// GetBlockCmd define the getblock JSON-RPC command
type GetBlockCmd struct {
	Hash string
	Verbose *bool `jsonrpcdefault:"true"`
	VerboseTx *bool `jsonrpcdefault:"false"`
}

func init()  {
	flags := UsageFlag(0)
	// (*GetBlockCmd)(nil) 相当于*GetBlockCmd类型的指针的初始化
	// note:这里是*GetBlockCmd
	MustRegisterCmd("getblock", (*GetBlockCmd)(nil), flags)
}
