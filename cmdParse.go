package gorpc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Request is a type for raw JSON-RPC 1.0 requests.  The Method field identifies
// the specific command type which in turns leads to different parameters.
// Callers typically will not use this directly since this package provides a
// statically typed command infrastructure which handles creation of these
// requests, however this struct it being exported in case the caller wants to
// construct raw requests for some reason.
type Request struct {
	Jsonrpc string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
	ID      interface{}       `json:"id"`
}

// UnmarshalCmd unmarshals a JSON-RPC request into a suitable concrete command
// so long as the method type contained within the marshalled request is
// registered.

func UnmarshalCmd(r *Request) (interface{}, error) {
	registerLock.Lock()
	rtp, ok := methodToConcreteType[r.Method] // 从这个map中寻找,这里就是注册方法
	info := methodToInfo[r.Method]
	registerLock.Unlock()
	if !ok {
		str := fmt.Sprintf("%q is not register", r.Method)
		return nil, makeError(ErrUnregisteredMethod, str)
	}
	rt := rtp.Elem()
	rvp := reflect.New(rt)
	rv := rvp.Elem()
	// 确保参数个数是正确的
	numParams := len(r.Params)
	if err := checkNumParams(numParams, &info); err != nil {
		return nil, err
	}
	// 遍历每个结构体字段
	for i := 0; i < numParams; i++ {
		rvf := rv.Field(i)
		// Unmarshal参数到结构体字段
		concreteVal := rvf.Addr().Interface()
		if err := json.Unmarshal(r.Params[i], &concreteVal); err != nil { // 参数和命令字段的顺序也应该是一一对应的
			// The most common error is the wrong type, so
			// explicitly detect that error and make it nicer.
			fieldName := strings.ToLower(rt.Field(i).Name)
			if jerr, ok := err.(*json.UnmarshalTypeError); ok {
				str := fmt.Sprintf("parameter #%d '%s' must "+
					"be type %v (got %v)", i+1, fieldName,
					jerr.Type, jerr.Value)
				return nil, makeError(ErrInvalidType, str)
			}

			// Fallback to showing the underlying error.
			str := fmt.Sprintf("parameter #%d '%s' failed to "+
				"unmarshal: %v", i+1, fieldName, err)
			return nil, makeError(ErrInvalidType, str)
		}
	}
	// When there are less supplied parameters than the total number of
	// params, any remaining struct fields must be optional.  Thus, populate
	// them with their associated default value as needed.
	if numParams < info.maxParams {
		populateDefaults(numParams, &info, rv)
	}

	return rvp.Interface(), nil
}

// populateDefaults populates default values into any remaining optional struct
// fields that did not have parameters explicitly provided.  The caller should
// have previously checked that the number of parameters being passed is at
// least the required number of parameters to avoid unnecessary work in this
// function, but since required fields never have default values, it will work
// properly even without the check.
func populateDefaults(numParams int, info *methodInfo, rv reflect.Value) {
	// When there are no more parameters left in the supplied parameters,
	// any remaining struct fields must be optional.  Thus, populate them
	// with their associated default value as needed.
	for i := numParams; i < info.maxParams; i++ {
		rvf := rv.Field(i)
		if defaultVal, ok := info.defaults[i]; ok {
			rvf.Set(defaultVal)
		}
	}
}

func checkNumParams(numParams int, info *methodInfo) error {
	if numParams < info.numReqParams || numParams > info.maxParams {
		if info.numReqParams == info.maxParams {
			str := fmt.Sprintf("wrong numnber of params (expacted %d ,receive %d)", info.numReqParams, numParams)
			return makeError(ErrNumParams, str)
		}
		str := fmt.Sprintf("wrong number of params (expected "+
			"between %d and %d, received %d)", info.numReqParams,
			info.maxParams, numParams)
		return makeError(ErrNumParams, str)
	}
	return nil
}
