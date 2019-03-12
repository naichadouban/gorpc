package rpcjson

import "reflect"
// UsageFlag define flags that specify additional properties about the
// circumstances under which a command can be used.
type UsageFlag uint32
// methodInfo keeps track of information about each registered method such as
// the parameter information.
type methodInfo struct {
	maxParams    int
	numReqParams int
	numOptParams int
	defaults     map[int]reflect.Value
	flags        UsageFlag
	usage        string
}
