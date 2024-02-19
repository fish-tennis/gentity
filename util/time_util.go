package util

import (
	"time"
)

// 获取当前时间戳(秒)
func GetCurrentTimeStamp() uint32 {
	return uint32(time.Now().Unix())
}

// 获取当前毫秒数(毫秒,0.001秒)
func GetCurrentMS() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// 去除Time中的时分秒,只保留日期
func ToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// 转换成20240219格式
func ToDateInt(t time.Time) int32 {
	y, m, d := t.Date()
	return int32(y*10000 + int(m)*100 + d)
}

// 2个日期的相隔天数
func DayCount(a time.Time, b time.Time) int {
	y, m, d := a.Date()
	bY, bM, bD := b.Date()
	aDate := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	bDate := time.Date(bY, bM, bD, 0, 0, 0, 0, time.Local)
	days := aDate.Sub(bDate) / (time.Hour * 24)
	if days < 0 {
		return int(-days)
	}
	return int(days)
}
