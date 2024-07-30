package util

import (
	"fmt"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// 根据组件名和消息名获取对应的消息号
// packageName是*.proto文件里定义的package名
func GetMessageIdByComponentMessageName(packageName, componentStructName, messageName string) int32 {
	// enum Message的全名举例:gserver.CmdMoney
	enumTypeName := ""
	if packageName != "" {
		enumTypeName = fmt.Sprintf("%v.Cmd%v", packageName, componentStructName)
	} else {
		enumTypeName = fmt.Sprintf("Cmd%v", componentStructName)
	}
	enumType, err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(enumTypeName))
	if err != nil {
		//gentity.Debug("%v err:%v", enumTypeName, err)
		return 0
	}
	// 如: Cmd_CoinReq
	enumIdName := fmt.Sprintf("Cmd_%v", messageName)
	enumValue := enumType.Descriptor().Values().ByName(protoreflect.Name(enumIdName))
	if enumValue == nil {
		return 0
	}
	enumNumber := enumValue.Number()
	//logger.Debug("enum %v:%v", enumIdName, enumNumber)
	return int32(enumNumber)
}

// 根据消息名获取对应的消息号
// packageName是*.proto文件里定义的package名
func GetMessageIdByMessageName(packageName, componentStructName, messageName string) int32 {
	// 先找组件对应的proto文件里定义的消息号
	messageId := GetMessageIdByComponentMessageName(packageName, componentStructName, messageName)
	if messageId == 0 {
		// 组件中没有,就直接找全局的,如: Cmd_CoinReq
		// NOTE: 不要在不同的proto文件里定义同名的消息号,否则可能冲突
		enumIdName := fmt.Sprintf("Cmd_%v", messageName)
		protoregistry.GlobalTypes.RangeEnums(func(enumType protoreflect.EnumType) bool {
			enumValue := enumType.Descriptor().Values().ByName(protoreflect.Name(enumIdName))
			if enumValue == nil {
				return true
			}
			messageId = int32(enumValue.Number())
			return false
		})
	}
	return messageId
}
