package codec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/joyparty/gokit"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/protobuf/proto"
)

// SizeLen 数据包长度帧的长度
const SizeLen = 4

var (
	// MaxMessageSize 客户端消息最大长度，默认64KB
	MaxMessageSize = 64 * 1024

	bufPool = gokit.NewPoolOf(func() *bytes.Buffer {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	})

	msgPool = &messagePool{
		pool: make(chan *Message, 1024),
	}
)

// SendReply 发送响应
func SendReply(reply *nh.Reply, sender func([]byte) error) error {
	data, err := proto.Marshal(reply)
	if err != nil {
		return fmt.Errorf("marshal reply, %w", err)
	}

	return SendBytes(data, sender)
}

// SendBytes 发送字节流
func SendBytes(data []byte, sender func([]byte) error) error {
	buf := bufPool.Get()
	defer bufPool.Put(buf)
	buf.Reset()

	if err := binary.Write(buf, binary.BigEndian, uint32(len(data))); err != nil {
		return fmt.Errorf("write size frame, %w", err)
	}

	if len(data) > 0 {
		if err := binary.Write(buf, binary.BigEndian, data); err != nil {
			return fmt.Errorf("write data frame, %w", err)
		}
	}
	return sender(buf.Bytes())
}

// Message 网络消息
type Message struct {
	data []byte
	size int
}

// Bytes 消息数据
func (msg Message) Bytes() []byte {
	return msg.data[:msg.size]
}

// Len 数据长度
func (msg Message) Len() int {
	return msg.size
}

// Reset 重置
func (msg *Message) Reset() {
	msg.size = 0
}

// ReadMessage 读消息
func ReadMessage(r io.Reader, msg *Message) error {
	if _, err := io.ReadFull(r, msg.data[:SizeLen]); err != nil {
		return fmt.Errorf("read size frame, %w", err)
	}

	msg.size = int(binary.BigEndian.Uint32(msg.data[:SizeLen]))
	if msg.size == 0 {
		return nil
	} else if msg.size > MaxMessageSize {
		return fmt.Errorf("message size exceeds the limit, %d", msg.size)
	}

	// grow bytes
	if c, s := cap(msg.data), msg.size; s > c {
		msg.data = append(msg.data[:c], make([]byte, s-c)...)
	}
	if _, err := io.ReadFull(r, msg.data[:msg.size]); err != nil {
		return fmt.Errorf("read data frame, %w", err)
	}
	return nil
}

// GetMessage 从对象池内获取Message实例
func GetMessage() *Message {
	return msgPool.Get()
}

// PutMessage 把Message实例放回对象池
func PutMessage(msg *Message) {
	msgPool.Put(msg)
}

type messagePool struct {
	pool chan *Message
}

func (p *messagePool) Get() *Message {
	select {
	case msg := <-p.pool:
		msg.Reset()
		return msg
	default:
		return &Message{
			data: make([]byte, 4*1024), // default 4k
		}
	}
}

func (p *messagePool) Put(msg *Message) {
	select {
	case p.pool <- msg:
	default:
	}
}
