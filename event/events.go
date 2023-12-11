package event

import (
	"encoding/json"
	"fmt"
	"reflect"
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
// 每种事件在注册时需要指定一个类型名
// 订阅者可以根据类型名来订阅感兴趣的事件
func Register(eventType string, ev any) {
	if _, ok := events[eventType]; ok {
		panic(fmt.Errorf("event %q already registered", eventType))
	}

	valueType := typeOf(ev)
	events[eventType] = valueType
	types[valueType] = eventType
}

type payload struct {
	Type   string `json:"t"`
	Detail []byte `json:"d"`
}

func newPayload(ev any) (p payload, err error) {
	eventType, ok := types[typeOf(ev)]
	if !ok {
		err = fmt.Errorf("unknown event %q", eventType)
		return
	}

	p.Type = eventType
	p.Detail, err = json.Marshal(ev)
	return
}

func typeOf(ev any) reflect.Type {
	t := reflect.TypeOf(ev)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

type unmarshaler func(p payload) (event any, err error)

func newUnmarshaler(want any) (unmarshaler, string, error) {
	wantType := typeOf(want)
	eventType, ok := types[wantType]
	if !ok {
		return nil, "", fmt.Errorf("unknown event %s.%s", wantType.PkgPath(), wantType.Name())
	}

	return func(p payload) (any, error) {
		ev := reflect.New(wantType)
		if err := json.Unmarshal(p.Detail, ev.Interface()); err != nil {
			return nil, fmt.Errorf("unmarshal event, %w", err)
		}

		return ev.Elem().Interface(), nil
	}, eventType, nil
}

// UserConnected 用户连接事件
type UserConnected struct {
	UserID string
}

// UserDisconnected 用户断开连接事件
type UserDisconnected struct {
	UserID string
}
