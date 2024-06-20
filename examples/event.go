package examples

var (
	_fireEventLoopLimit = int32(3)
)

// 事件示例
type PlayerEntryGame struct {
	IsReconnect    bool
	OfflineSeconds int32 // 离线时长
}

type LoopCheckA struct {
	Num int32
}

type LoopCheckB struct {
	Name string
}
