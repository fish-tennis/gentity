package gentity

import (
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

var _componentHandlerInfos = make(map[gnet.PacketCommand]*ComponentHandlerInfo)

// 组件回调接口信息
type ComponentHandlerInfo struct {
	componentName string
	method        reflect.Method
	handler       func(c Component, m proto.Message)
}

// 根据proto的命名规则和组件里消息回调的格式,通过反射自动生成消息的注册
// 类似Java的注解功能
// 游戏常见有2类消息
// 1.客户端的请求消息
// 可以在组件里编写函数: OnXxx(cmd PacketCommand, req *pb.Xxx)
// 2.服务器内部的逻辑消息
// 可以在组件里编写函数: HandleXxx(cmd PacketCommand, req *pb.Xxx)
//
// 按照这种格式编写的函数,调用AutoRegisterComponentHandler,可以自动注册
func AutoRegisterComponentHandler(entity Entity, packetHandlerRegister gnet.PacketHandlerRegister, clientHandlerPrefix,otherHandlerPrefix,protoPackageName string) {
	entity.RangeComponent(func(component Component) bool {
		typ := reflect.TypeOf(component)
		// 如: game.Quest -> Quest
		componentStructName := typ.String()[strings.LastIndex(typ.String(), ".")+1:]
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			if method.Type.NumIn() != 3 {
				continue
			}
			isClientMessage := false
			if strings.HasPrefix(method.Name, clientHandlerPrefix) {
				// 客户端消息回调
				isClientMessage = true
			} else if strings.HasPrefix(method.Name, otherHandlerPrefix) {
				// 非客户端的逻辑消息回调
			} else {
				continue
			}
			// 消息回调格式: func (this *Quest) OnFinishQuestReq(cmd PacketCommand, req *pb.FinishQuestReq)
			methonArg1 := method.Type.In(1)
			// 参数1是消息号
			if methonArg1.Name() != "PacketCommand" && methonArg1.Name() != "gnet.PacketCommand" {
				continue
			}
			methonArg2 := method.Type.In(2)
			// 参数2是proto定义的消息体
			if !methonArg2.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
				continue
			}
			// 消息名,如: FinishQuestReq
			// *pb.FinishQuestReq -> FinishQuestReq
			messageName := methonArg2.String()[strings.LastIndex(methonArg2.String(), ".")+1:]
			// 客户端消息回调的函数名规则,如OnFinishQuestReq
			if isClientMessage && method.Name != fmt.Sprintf("%v%v", clientHandlerPrefix, messageName) {
				logger.Debug("client methodName not match:%v", method.Name)
				continue
			}
			// 非客户端消息回调的函数名规则,如HandleFinishQuestReq
			if !isClientMessage && method.Name != fmt.Sprintf("%v%v", otherHandlerPrefix, messageName) {
				logger.Debug("methodName not match:%v", method.Name)
				continue
			}
			messageId := util.GetMessageIdByMessageName(protoPackageName, componentStructName, messageName)
			if messageId == 0 {
				continue
			}
			cmd := gnet.PacketCommand(messageId)
			// 注册消息回调到组件上
			_componentHandlerInfos[cmd] = &ComponentHandlerInfo{
				componentName: component.GetName(),
				method: method,
			}
			// 注册客户端消息
			if isClientMessage && packetHandlerRegister != nil {
				packetHandlerRegister.Register(cmd, nil, reflect.New(methonArg2.Elem()).Interface().(proto.Message))
			}
			logger.Debug("AutoRegister %v.%v %v client:%v", componentStructName, method.Name, messageId, isClientMessage)
		}
		return true
	})
}

// 用于proto_code_gen工具自动生成的消息注册代码
func RegisterProtoCodeGen(packetHandlerRegister gnet.PacketHandlerRegister, componentName string, cmd gnet.PacketCommand, protoMessage proto.Message, handler func(c Component, m proto.Message)) {
	_componentHandlerInfos[cmd] = &ComponentHandlerInfo{
		componentName: componentName,
		handler: handler,
	}
	packetHandlerRegister.Register(cmd, nil, protoMessage)
}

// 执行注册的消息回调接口
// return true表示执行了接口
// return false表示未执行
func ProcessComponentHandler(entity Entity, cmd gnet.PacketCommand, message proto.Message) bool {
	// 先找组件接口
	handlerInfo := _componentHandlerInfos[cmd]
	if handlerInfo != nil {
		component := entity.GetComponentByName(handlerInfo.componentName)
		if component != nil {
			if handlerInfo.handler != nil {
				handlerInfo.handler(component, message)
			} else {
				// 反射调用函数
				handlerInfo.method.Func.Call([]reflect.Value{reflect.ValueOf(component),
					reflect.ValueOf(cmd),
					reflect.ValueOf(message)})
			}
			return true
		}
	}
	return false
}