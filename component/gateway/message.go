package gateway

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/protobuf/proto"
)

const sizeLen = 4

var (
	bbPool = &sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 1024))
		},
	}

	msgPool = &messagePool{
		pool: make(chan *message, 1024),
	}
)

func sendReply(reply *nh.Reply, sender func([]byte) error) error {
	data, err := proto.Marshal(reply)
	if err != nil {
		return fmt.Errorf("marshal reply, %w", err)
	}

	buf := bbPool.Get().(*bytes.Buffer)
	defer bbPool.Put(buf)
	buf.Reset()

	if err := binary.Write(buf, binary.BigEndian, uint32(len(data))); err != nil {
		return fmt.Errorf("write size frame, %w", err)
	} else if err := binary.Write(buf, binary.BigEndian, data); err != nil {
		return fmt.Errorf("write data frame, %w", err)
	}

	return sender(buf.Bytes())
}

type message struct {
	data []byte
	size int
}

func newMessage() *message {
	return &message{
		data: make([]byte, MaxMessageSize),
	}
}

func (msg message) Bytes() []byte {
	return msg.data[:msg.size]
}

func readMessage(r io.Reader, msg *message) error {
	if _, err := io.ReadFull(r, msg.data[:sizeLen]); err != nil {
		return fmt.Errorf("read size frame, %w", err)
	}

	msg.size = int(binary.BigEndian.Uint32(msg.data[:sizeLen]))
	if msg.size == 0 {
		return nil
	} else if msg.size > MaxMessageSize {
		return fmt.Errorf("message size exceeds the limit, %d", msg.size)
	}

	if _, err := io.ReadFull(r, msg.data[:msg.size]); err != nil {
		return fmt.Errorf("read data frame, %w", err)
	}
	return nil
}

type messagePool struct {
	pool chan *message
}

func (p *messagePool) Get() *message {
	select {
	case msg := <-p.pool:
		msg.size = 0
		return msg
	default:
		return newMessage()
	}
}

func (p *messagePool) Put(msg *message) {
	select {
	case p.pool <- msg:
	default:
	}
}
