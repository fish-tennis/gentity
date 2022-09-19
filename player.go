package gentity

type Player interface {
	Entity

	// 玩家名
	GetName() string

	// 账号id
	GetAccountId() int64

	// 区服id
	GetRegionId() int32
}

type PlayerMgr interface {
	GetPlayer(playerId int64) *Player
	AddPlayer(player *Player)
	RemovePlayer(player *Player)
}