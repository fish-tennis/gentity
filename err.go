package gentity

import "errors"

var (
	ErrNotSaveableStruct     = errors.New("not saveable struct")
	ErrUnsupportedKeyType    = errors.New("unsupported key type")
	ErrUnsupportedType       = errors.New("unsupported type")
	ErrNotConnected          = errors.New("not connected")
	ErrSliceElemType         = errors.New("slice elem type error")
	ErrArrayLen              = errors.New("array len error")
	ErrNoUniqueColumn        = errors.New("no uniqueId column")
	ErrRouteServerId         = errors.New("route serverId error")
	ErrEntityNotExists       = errors.New("entity not exists")
	ErrConvertRoutineMessage = errors.New("convert routine message error")
	ErrSourceDataType        = errors.New("sourceData type error")
)
