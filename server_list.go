package gentity

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
	"sort"
	"sync"
)

// 服务器信息接口
type ServerInfo interface {
	GetServerId() int32
	GetServerType() string
	GetLastActiveTime() int64
}

// 服务器列表接口
type ServerList interface {
	GetServerInfo(serverId int32) ServerInfo

	// 服务发现: 读取服务器列表信息,并连接这些服务器
	FindAndConnectServers(ctx context.Context)

	// 服务注册:上传本地服务器的信息
	RegisterLocalServerInfo()

	Send(serverId int32, cmd gnet.PacketCommand, message proto.Message) bool
}

type BaseServerList struct {
	// 缓存接口
	cache               KvCache
	// bytes -> ServerInfo
	serverInfoUnmarshal func(bytes []byte) ServerInfo
	// ServerInfo -> bytes
	serverInfoMarshal   func(info ServerInfo) []byte
	// 需要获取信息的服务器类型
	fetchServerTypes []string
	// 需要连接的服务器类型
	connectServerTypes []string
	// 服务器多少毫秒没上传自己的信息,就判断为不活跃了
	activeTimeout int32
	// 缓存的服务器列表信息
	serverInfos      map[int32]ServerInfo // serverId-ServerInfo
	serverInfosMutex sync.RWMutex
	// 按照服务器类型分组的服务器列表信息
	serverInfoTypeMap      map[string][]ServerInfo
	serverInfoTypeMapMutex sync.RWMutex
	// 本地服务器信息
	localServerInfo ServerInfo
	// 已连接的服务器连接
	connectedServerConnectors      map[int32]gnet.Connection // serverId-Connection
	connectedServerConnectorsMutex sync.RWMutex
	// 服务器连接创建函数,供外部扩展
	serverConnectorFunc func(ctx context.Context, info ServerInfo) gnet.Connection
	listUpdateHooks     []func(serverList map[string][]ServerInfo, oldServerList map[string][]ServerInfo)
}

func NewBaseServerList() *BaseServerList {
	return &BaseServerList{
		activeTimeout:             3 * 1000, // 默认3秒
		serverInfos:               make(map[int32]ServerInfo),
		connectedServerConnectors: make(map[int32]gnet.Connection),
		serverInfoTypeMap:         make(map[string][]ServerInfo),
	}
}

func (this *BaseServerList) SetCache(cache KvCache) {
	this.cache = cache
}

// 设置ServerInfo的序列化接口
func (this *BaseServerList) SetServerInfoFunc(serverInfoUnmarshal func(bytes []byte) ServerInfo,
	serverInfoMarshal func(info ServerInfo) []byte) {
	this.serverInfoUnmarshal = serverInfoUnmarshal
	this.serverInfoMarshal = serverInfoMarshal
}

// 设置服务器连接创建函数
func (this *BaseServerList) SetServerConnectorFunc(connectFunc func(ctx context.Context, info ServerInfo) gnet.Connection) {
	this.serverConnectorFunc = connectFunc
}

// 服务发现: 读取服务器列表信息,并连接这些服务器
func (this *BaseServerList) FindAndConnectServers(ctx context.Context) {
	serverInfoMapUpdated := false
	infoMap := make(map[int32]ServerInfo)
	for _, serverType := range this.fetchServerTypes {
		serverInfoDatas := make(map[string]string)
		err := this.cache.GetMap(fmt.Sprintf("servers:%v", serverType), serverInfoDatas)
		if IsRedisError(err) {
			GetLogger().Error("get %v info err:%v", serverType, err)
			continue
		}
		for idStr, serverInfoData := range serverInfoDatas {
			serverInfo := this.serverInfoUnmarshal([]byte(serverInfoData))
			if serverInfo == nil {
				GetLogger().Error("serverInfoCreator err:k:%v v:%v", idStr, serverInfoData)
				continue
			}
			// 目标服务器已经处于"不活跃"状态了
			if util.GetCurrentMS()-serverInfo.GetLastActiveTime() > int64(this.activeTimeout) {
				continue
			}
			// 这里不用加锁,因为其他协程不会修改serverInfos
			if _, ok := this.serverInfos[serverInfo.GetServerId()]; !ok {
				serverInfoMapUpdated = true
			}
			infoMap[serverInfo.GetServerId()] = serverInfo
		}
	}
	if len(this.serverInfos) != len(infoMap) {
		serverInfoMapUpdated = true
	}
	// 服务器列表有更新,才更新服务器列表和类型分组信息
	if serverInfoMapUpdated {
		this.serverInfosMutex.Lock()
		this.serverInfos = infoMap
		this.serverInfosMutex.Unlock()
		serverInfoTypeMap := make(map[string][]ServerInfo)
		for _, info := range infoMap {
			infoSlice, ok := serverInfoTypeMap[info.GetServerType()]
			if !ok {
				infoSlice = make([]ServerInfo, 0)
				serverInfoTypeMap[info.GetServerType()] = infoSlice
			}
			infoSlice = append(infoSlice, info)
			serverInfoTypeMap[info.GetServerType()] = infoSlice
		}
		var oldList map[string][]ServerInfo
		this.serverInfoTypeMapMutex.Lock()
		oldList = this.serverInfoTypeMap
		this.serverInfoTypeMap = serverInfoTypeMap
		this.serverInfoTypeMapMutex.Unlock()
		for _, hookFunc := range this.listUpdateHooks {
			hookFunc(serverInfoTypeMap, oldList)
		}
	}

	for _, info := range infoMap {
		if util.HasString(this.connectServerTypes, info.GetServerType()) {
			if this.localServerInfo.GetServerId() == info.GetServerId() {
				continue
			}
			//// 目标服务器已经处于"不活跃"状态了
			//if util.GetCurrentMS() - info.LastActiveTime > int64(this.activeTimeout) {
			//	continue
			//}
			this.ConnectServer(ctx, info)
		}
	}
}

// 连接其他服务器
func (this *BaseServerList) ConnectServer(ctx context.Context, info ServerInfo) {
	if info == nil || this.serverConnectorFunc == nil {
		return
	}
	this.connectedServerConnectorsMutex.RLock()
	_, ok := this.connectedServerConnectors[info.GetServerId()]
	this.connectedServerConnectorsMutex.RUnlock()
	if ok {
		return
	}
	serverConn := this.serverConnectorFunc(ctx, info)
	if serverConn != nil {
		serverConn.SetTag(info.GetServerId())
		this.connectedServerConnectorsMutex.Lock()
		this.connectedServerConnectors[info.GetServerId()] = serverConn
		this.connectedServerConnectorsMutex.Unlock()
		GetLogger().Info("ConnectServer %v, %v", info.GetServerId(), info.GetServerType())
	} else {
		GetLogger().Info("ConnectServerError %v, %v", info.GetServerId(), info.GetServerType())
	}
}

// 服务注册:上传本地服务器的信息
func (this *BaseServerList) RegisterLocalServerInfo() {
	bytes := this.serverInfoMarshal(this.localServerInfo)
	this.cache.HSet(fmt.Sprintf("servers:%v", this.localServerInfo.GetServerType()),
		util.Itoa(this.localServerInfo.GetServerId()), bytes)
}

// 获取某个服务器的信息
func (this *BaseServerList) GetServerInfo(serverId int32) ServerInfo {
	this.serverInfosMutex.RLock()
	defer this.serverInfosMutex.RUnlock()
	info, _ := this.serverInfos[serverId]
	return info
}

// 自己的服务器信息
func (this *BaseServerList) GetLocalServerInfo() ServerInfo {
	return this.localServerInfo
}

func (this *BaseServerList) SetLocalServerInfo(info ServerInfo) {
	this.localServerInfo = info
}

// 服务器连接断开了
func (this *BaseServerList) OnServerConnectorDisconnect(serverId int32) {
	this.connectedServerConnectorsMutex.Lock()
	delete(this.connectedServerConnectors, serverId)
	this.connectedServerConnectorsMutex.Unlock()
	GetLogger().Debug("DisconnectServer %v", serverId)
}

// 设置要获取的服务器类型
func (this *BaseServerList) SetFetchServerTypes(serverTypes ...string) {
	this.fetchServerTypes = append(this.fetchServerTypes, serverTypes...)
	GetLogger().Debug("fetch:%v", serverTypes)
}

// 设置要获取并连接的服务器类型
func (this *BaseServerList) SetFetchAndConnectServerTypes(serverTypes ...string) {
	this.fetchServerTypes = append(this.fetchServerTypes, serverTypes...)
	this.connectServerTypes = append(this.connectServerTypes, serverTypes...)
	GetLogger().Info("fetch connect:%v", serverTypes)
}

// 获取某类服务器的信息列表
func (this *BaseServerList) GetServersByType(serverType string) []ServerInfo {
	this.serverInfoTypeMapMutex.RLock()
	defer this.serverInfoTypeMapMutex.RUnlock()
	if infoList, ok := this.serverInfoTypeMap[serverType]; ok {
		copyInfoList := make([]ServerInfo, len(infoList), len(infoList))
		for idx, info := range infoList {
			copyInfoList[idx] = info
		}
		sort.Slice(copyInfoList, func(i, j int) bool {
			return copyInfoList[i].GetServerId() < copyInfoList[j].GetServerId()
		})
		return copyInfoList
	}
	return nil
}

// 获取服务器的连接
func (this *BaseServerList) GetServerConnector(serverId int32) gnet.Connection {
	this.connectedServerConnectorsMutex.RLock()
	connection, _ := this.connectedServerConnectors[serverId]
	this.connectedServerConnectorsMutex.RUnlock()
	return connection
}

// 发消息给另一个服务器
func (this *BaseServerList) Send(serverId int32, cmd gnet.PacketCommand, message proto.Message) bool {
	connection := this.GetServerConnector(serverId)
	if connection != nil && connection.IsConnected() {
		return connection.Send(cmd, message)
	}
	return false
}

func (this *BaseServerList) SendPacket(serverId int32, packet gnet.Packet) bool {
	connection := this.GetServerConnector(serverId)
	if connection != nil && connection.IsConnected() {
		return connection.SendPacket(packet)
	}
	return false
}

// 添加服务器列表更新回调
func (this *BaseServerList) AddListUpdateHook(onListUpdateFunc ...func(serverList map[string][]ServerInfo, oldServerList map[string][]ServerInfo)) {
	this.listUpdateHooks = append(this.listUpdateHooks, onListUpdateFunc...)
}
