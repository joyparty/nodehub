package gateway

import (
	"bytes"
	"encoding/binary"
	"fmt"
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

	bsPool = &sync.Pool{
		New: func() any {
			return make([]byte, MaxPayloadSize)
		},
	}
)

func sendBy(reply *nh.Reply, sender func([]byte) error) error {
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
