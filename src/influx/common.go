package influx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"influx/protocol"
	"code.google.com/p/goprotobuf/proto"
)

func (self *Client) ReadGreeting(message *protocol.Greeting) error {
	return self.readMessage(message)
}

func (self *Client) ReadCommand(message *protocol.Command) error {
	return self.readMessage(message)
}

func (self *Client) readMessage(message interface{}) error {
	var err error
	if err = self.ReadRaw(); err != nil {
		return err
	}

	if command, ok := message.(*protocol.Command); ok {
		err = proto.Unmarshal(self.Buffer.Bytes(), command)
	} else if greeting, ok := message.(*protocol.Greeting); ok {
		err = proto.Unmarshal(self.Buffer.Bytes(), greeting)
	}

	return err
}

func (self *Client) ReadRaw() error {
	var messageSizeU uint32
	var err error

	self.Buffer.Reset()

	err = binary.Read(self.Conn, binary.LittleEndian, &messageSizeU)
	if err != nil {
		return err
	}
	size := int64(messageSizeU)
	reader := io.LimitReader(self.Conn, size)
	_, err = io.Copy(self.Buffer, reader)
	if err != nil {
		return err
	}

	return nil
}

func (self *Client) WriteRequest(request interface{}) error {
	var messageSizeU uint32
	var d []byte

	if req, ok := request.(*protocol.Greeting); ok {
		d, _ = proto.Marshal(req)
	} else if req, ok := request.(*protocol.Command); ok {
		d, _ = proto.Marshal(req)
	} else {
		return errors.New(fmt.Sprintf("unsupported type %v", request))
	}

	self.Buffer.Reset()
	messageSizeU = uint32(len(d))
	binary.Write(self.Buffer, binary.LittleEndian, messageSizeU)
	_, err := io.Copy(self.Buffer, bytes.NewReader(d))
	if err != nil {
		return err
	}
	_, err = self.Conn.Write(self.Buffer.Bytes())
	if err != nil {
		return err
	}

	return nil
}
