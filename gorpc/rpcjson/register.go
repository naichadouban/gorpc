package rpcjson

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// UsageFlag define flags that specify additional properties about the
// circumstances under which a command can be used.
type UsageFlag uint32

const (
	// UFWalletOnly indicates that the command can only be used with an RPC
	// server that supports wallet commands.
	UFWalletOnly UsageFlag = 1 << iota

	// UFWebsocketOnly indicates that the command can only be used when
	// communicating with an RPC server over websockets.  This typically
	// applies to notifications and notification registration functions
	// since neiher makes since when using a single-shot HTTP-POST request.
	UFWebsocketOnly

	// UFNotification indicates that the command is actually a notification.
	// This means when it is marshalled, the ID must be nil.
	UFNotification

	// highestUsageFlagBit is the maximum usage flag bit and is used in the
	// stringer and tests to ensure all of the above constants have been
	// tested.
	highestUsageFlagBit
)

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

var registerLock sync.RWMutex
var methodToConcreteType = make(map[string]reflect.Type)
var methodToInfo = make(map[string]methodInfo)
var concreteTypeToMethod = make(map[reflect.Type]string)
// MustRegisterCmd performs the same function as RegisterCmd except it panics
// if there is an error.  This should only be called from package init
// functions.
func MustRegisterCmd(method string, cmd interface{}, flags UsageFlag) {
	if err := RegisterCmd(method, cmd, flags); err != nil {
		panic(fmt.Sprintf("failed to register type %q:%v\n", method, err))
	}
}

// TODO 这里用反射的目的到底是什么？是要预防什么情况吗？
// 这个方法会设置map，methodToInfo和methodToConcreteType和concreteTypeToMethod
func RegisterCmd(method string, cmd interface{}, flags UsageFlag) error {
	registerLock.Lock()
	defer registerLock.Unlock()
	// method是否已经注册
	if _, ok := methodToConcreteType[method]; ok {
		str := fmt.Sprintf("method %q is already registered", method)
		return makeError(ErrDuplicateMethod, str)
	}

	// Ensure that no unrecognized flag bits were specified.
	// TODO 下面这个运算还没有看懂
	if ^(highestUsageFlagBit - 1)&flags != 0 {
		str := fmt.Sprintf("invalid usage flags specified for method "+
			"%s: %v", method, flags)
		return makeError(ErrInvalidUsageFlags, str)
	}

	rtp := reflect.TypeOf(cmd)
	// Type和Value都有一个Kind方法可以返回一个常量用于指示一个项到底是
	// 以什么形式(也就是底层类型underlying type)存储的（what sort of item is stored)
	// 所以我们注册命令的时候必须传入结构体的指针
	if rtp.Kind() != reflect.Ptr {
		str := fmt.Sprintf("type must be *struct not '%s (%s)'", rtp,
			rtp.Kind())
		return makeError(ErrInvalidType, str)
	}
	// 我们要的不是rtp，而是（从效果上来说）*p。
	// 为了得到rtp指向的东西，我们调用rtp的Elem()方法。
	// 这个判断和上面那个判断就是就是结构体指针和结构体，相互对应。
	rt := rtp.Elem()
	if rt.Kind() != reflect.Struct {
		str := fmt.Sprintf("type must be *struct not '%s (*%s)'",
			rtp, rt.Kind())
		return makeError(ErrInvalidType, str)
	}

	// Enumerate the struct fields to validate them and gather parameter
	// information.
	// 枚举结构体字段，验证他们、收集参数信息
	numFields := rt.NumField()
	numOptFields := 0
	defaults := make(map[int]reflect.Value)
	for i := 0; i < numFields; i++ {
		rtf := rt.Field(i)
		if rtf.Anonymous { // 匿名字段不支持
			str := fmt.Sprintf("embedded fields are not supported "+
				"(field name: %q)", rtf.Name)
			return makeError(ErrEmbeddedType, str)
		}
		// PkgPath 返回指定类型的import package path，如果是在main包中定义的结构体，返回就是main
		// 也就是说，如果代码中有import encoding/base64这样的语句，
		// 那么通过PkgPath()就会返回encoding/base64，而不是base64package所在的实际路径。
		// 反言之，PkgPath()返回的是import package path。
		// 对于Go内置的类型string,error等，或者未定义名称的类型struct{}等，则返回空字符串。
		// ============= 上面都是对结构体来说的，现在的场景是结构体字段，reflect.StructField 
		// 对结构体字段，PkgPath是非导出字段的包路径，对导出字段该字段为""
		if rtf.PkgPath != "" {  // 不是可导出字段
			str := fmt.Sprintf("unexported fields are not supported "+
				"(field name: %q)", rtf.Name)
			return makeError(ErrUnexportedField, str)
		}

		// Disallow types that can't be JSON encoded.  Also, determine
		// if the field is optional based on it being a pointer.
		// 决定一个字段是否可选依赖他是否是一个指针
		var isOptional bool // 是否可选
		switch kind := rtf.Type.Kind(); kind {
		case reflect.Ptr: // 指针
			isOptional = true
			kind = rtf.Type.Elem().Kind()
			fallthrough // 一般是break，不会忘下执行了。fallthrough强制忘下执行
		default:
			if !isAcceptableKind(kind) {
				str := fmt.Sprintf("unsupported field type "+
					"'%s (%s)' (field name %q)", rtf.Type,
					baseKindString(rtf.Type), rtf.Name)
				return makeError(ErrUnsupportedFieldType, str)
			}
		}

		// Count the optional fields and ensure all fields after the
		// first optional field are also optional.
		// 统计可选字段数，确保第一个可选字段之后都是可选的

		if isOptional {
			numOptFields++
		} else {
			if numOptFields > 0 {  // 前面已经有可选的字段，你这个尽然还不可以选，不行
				str := fmt.Sprintf("all fields after the first "+
					"optional field must also be optional "+
					"(field name %q)", rtf.Name)
				return makeError(ErrNonOptionalField, str)
			}
		}

		// Ensure the default value can be unsmarshalled into the type
		// and that defaults are only specified for optional fields.
		if tag := rtf.Tag.Get("jsonrpcdefault"); tag != "" {
			// 字段不是可选的话，不能有jsonrpcdefault这个tag
			if !isOptional {
				str := fmt.Sprintf("required fields must not "+
					"have a default specified (field name "+
					"%q)", rtf.Name)
				return makeError(ErrNonOptionalDefault, str)
			}

			rvf := reflect.New(rtf.Type.Elem())
			err := json.Unmarshal([]byte(tag), rvf.Interface())
			if err != nil {
				str := fmt.Sprintf("default value of %q is "+
					"the wrong type (field name %q)", tag,
					rtf.Name)
				return makeError(ErrMismatchedDefault, str)
			}
			defaults[i] = rvf
		}
	}

	// Update the registration maps.
	methodToConcreteType[method] = rtp
	methodToInfo[method] = methodInfo{
		maxParams:    numFields,
		numReqParams: numFields - numOptFields,
		numOptParams: numOptFields,
		defaults:     defaults,
		flags:        flags,
	}
	concreteTypeToMethod[rtp] = method
	return nil
}

// baseKindString returns the base kind for a given reflect.Type after
// indirecting through all pointers.
func baseKindString(rt reflect.Type) string {
	numIndirects := 0
	for rt.Kind() == reflect.Ptr {
		numIndirects++
		rt = rt.Elem()
	}

	return fmt.Sprintf("%s%s", strings.Repeat("*", numIndirects), rt.Kind())
}

// isAcceptableKind returns whether or not the passed field type is a supported
// type.  It is called after the first pointer indirection, so further pointers
// are not supported.
func isAcceptableKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Chan:
		fallthrough
	case reflect.Complex64:
		fallthrough
	case reflect.Complex128:
		fallthrough
	case reflect.Func:
		fallthrough
	case reflect.Ptr:
		fallthrough
	case reflect.Interface:
		return false  // 接口不可以
	}

	return true
}
