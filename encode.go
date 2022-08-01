package net

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	MsgTypeRequest   MsgType = 1
	MsgTypeResponse          = 2
	MsgTypePush              = 4
	MsgTypeHandshake         = 8
	MsgTypeHeartbeat         = 16
)

type MsgType int8

type Pack struct {
	Type  MsgType
	Id    string
	Route string
	Body  any
}

type timeout struct {
	Id string
}

func encodePack(pack *Pack) []byte {
	dataBytes, _ := json.Marshal(pack)
	headBytes := []byte{0, 0, 0, 0}
	binary.BigEndian.PutUint32(headBytes, uint32(len(dataBytes)))
	return bytes.Join([][]byte{headBytes, dataBytes}, nil)
}

func decodePack(reader io.Reader) (*Pack, error) {
	buf := []byte{0, 0, 0, 0}
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	hLen := binary.BigEndian.Uint32(buf)
	if hLen > 10*1024*1024 {
		fmt.Println("data too big")
		return nil, nil
	}

	dataBytes := make([]byte, hLen)
	_, err = io.ReadFull(reader, dataBytes)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	// fmt.Println("receive data:", str(dataBytes))

	p := &Pack{}
	err = json.Unmarshal(dataBytes, &p)
	if err != nil {
		return nil, err
	}
	return p, nil
}
