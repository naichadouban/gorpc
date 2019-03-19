package gorpc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

var test_methodToConcreteType = make(map[string]reflect.Type)

var test_concreteTypeToMethod = make(map[reflect.Type]string)

type GetUser struct {
	Name string
	All  *bool `jsonrpcdefault:"true"`
}

func TestRegister(t *testing.T) {
	TesRegisteCmd("GetUser", (*GetUser)(nil))
}
func TesRegisteCmd(method string, cmd interface{}) {
	if _, ok := test_methodToConcreteType[method]; ok {
		fmt.Println("方法已经注册了")
		return
	}
	rtp := reflect.TypeOf(cmd)

	fmt.Printf("rtp type:%#v\n", rtp)
	fmt.Printf("rtp.Kind type=%v\n", rtp.Kind())
	if rtp.Kind() != reflect.Ptr {
		str := fmt.Sprintf("type must be *struct not '%s (%s)'\n", rtp,
			rtp.Kind())
		fmt.Println(str)
		return
	}

	rt := rtp.Elem()
	fmt.Printf("rt |||%#v", rt)
	if rt.Kind() != reflect.Struct {
		str := fmt.Sprintf("type must be *struct not '%s (*%s)'\n",
			rtp, rt.Kind())
		fmt.Println(str)
		return
	}

	numFields := rt.NumField()
	numOptFields := 0
	defaults := make(map[int]reflect.Value)
	fmt.Println("===========")
	for i := 0; i < numFields; i++ {
		rtf := rt.Field(i)
		fmt.Printf("%#v\n", rtf)
		if rtf.Anonymous {
			fmt.Println("不支持匿名字段")
			return
		}
		if rtf.PkgPath != "" {
			fmt.Println("不是可导出字段，可能字段名灭有大写把")
			return
		}
		fmt.Printf("%#v\n", rtf.Type.Kind())
		fmt.Printf("%T%v\n", rtf.Type.Kind(), rtf.Type.Kind())
		var isOptional bool
		switch kind := rtf.Type.Kind(); kind {
		case reflect.Ptr:
			isOptional = true
			kind = rtf.Type.Kind()
			fallthrough
		default:

		}
		if isOptional { // 如果是可选的话
			numOptFields ++
		} else {
			if numOptFields > 0 { // 前面已经有可选字段了
				fmt.Println("前面已经有可选字段了，你就必须可选")
				return
			}
		}

		if tag := rtf.Tag.Get("jsonrpcdefault"); tag != "" {
			if !isOptional {
				fmt.Println("不是可选的，你就不要加这个tag了")
				return
			}
			// 是可选的字段，现在就是要取tag中的默认值了
			rvf := reflect.New(rtf.Type.Elem())
			err := json.Unmarshal([]byte(tag), rvf.Interface())
			if err != nil {
				str := fmt.Sprintf("default value of %q is "+
					"the wrong type (field name %q)", tag,
					rtf.Name)
				fmt.Println(str)
			}
			defaults[i] = rvf
			fmt.Println(defaults)

		}
	}

	// 把注册信息跟新到map中
	test_methodToConcreteType[method] = rtp // rtp已经是修改之后的了
	test_concreteTypeToMethod[rtp] = method
	fmt.Printf("%#v\n", test_methodToConcreteType)
	fmt.Printf("%#v\n", test_concreteTypeToMethod)

	// 下面的是comParse中UnmarshalCmd方法的测试

}
