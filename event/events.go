package event

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

var (
	events = map[string]reflect.Type{}
	types  = map[reflect.Type]string{}
)

func init() {
	Register("user:connected", UserConnected{})
	Register("user:disconnected", UserDisconnected{})
}

// Register 注册事件
//
// 每种事件在注册时需要指定一个类型名，类型名只需要保证唯一即可
func Register(eventType string, ev any) {
	if _, ok := events[eventType]; ok {
		panic(fmt.Errorf("event %q already registered", eventType))
	}

	valueType := deref(reflect.TypeOf(ev))
	events[eventType] = valueType
	types[valueType] = eventType
}

type payload struct {
	Time   int64  `json:"t"`
	Type   string `json:"ty"`
	Detail []byte `json:"d"`
}

func (p payload) GetTime() time.Time {
	return time.UnixMicro(p.Time)
}

func newPayload(ev any) (p payload, err error) {
	eventType, ok := types[deref(reflect.TypeOf(ev))]
	if !ok {
		err = fmt.Errorf("unknown event %q", eventType)
		return
	}

	p.Time = time.Now().UnixMicro()
	p.Type = eventType
	p.Detail, err = json.Marshal(ev)
	return
}

func deref(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

// UserConnected 用户连接事件
type UserConnected struct {
	GatewayID  string `json:"gatewayID"`
	UserID     string `json:"userID"`
	RemoteAddr string `json:"remoteAddr"`
}

// UserDisconnected 用户断开连接事件
type UserDisconnected struct {
	GatewayID  string `json:"gatewayID"`
	UserID     string `json:"userID"`
	RemoteAddr string `json:"remoteAddr"`
}
