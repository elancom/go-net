package net

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elancom/go-util/msg"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

type ClientConfig struct{}

type Req struct {
	id      string
	out     chan<- *msg.Msg
	timeout *time.Timer
}

type IOCat struct {
	packChan  chan *Pack
	closeChan chan any
	read      func() (*Pack, error)
}

func (i *IOCat) cat() {
	for {
		pack, err := i.read()
		if err != nil {
			break
		}
		i.packChan <- pack
	}
	i.closeChan <- 1
}

type Client struct {
	conn      net.Conn
	reqId     int64
	reqs      map[string]*Req
	packChan  chan *Pack // 响应包通道
	closeChan chan any   // 关闭通道
	dataChan  chan any   // 业务推送通道
	echoTime  *time.Time
}

func NewClient() *Client {
	c := Client{}
	c.reqs = make(map[string]*Req)
	c.packChan = make(chan *Pack)
	c.closeChan = make(chan any)
	c.dataChan = make(chan any)
	return &c
}

func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", "127.0.0.1:8888")
	if err != nil {
		return err
	}
	c.conn = conn
	// handshake
	pack, err := c.Read()
	if pack.Type != MsgTypeHandshake {
		_ = c.conn.Close()
		c.closeChan <- 1
		return errors.New("handshake error")
	}
	b, _ := json.Marshal(pack)
	fmt.Println("handshake:", string(b))
	go c.ready()
	return nil
}

func (c *Client) ready() {
	// read data
	ioPackChan := make(chan *Pack)
	ioClosedChan := make(chan int)
	go func() {
		for {
			pack, err := c.Read()
			if err != nil {
				break
			}
			ioPackChan <- pack
		}
		ioClosedChan <- 1
	}()
	// handle msg
loop:
	for {
		select {
		case pack := <-c.packChan:
			c.handlePack(pack)
		case pack := <-ioPackChan:
			c.handlePack(pack)
		case <-ioClosedChan:
			break loop
		}
	}
	c.closeChan <- 1
}

func (c Client) handlePack(pack *Pack) {
	if pack == nil {
		fmt.Println("非法数据包")
		return
	}
	switch pack.Type {
	case MsgTypeResponse:
		req := c.reqs[pack.Id]
		if req != nil {
			delete(c.reqs, pack.Id)
			req.timeout.Stop()
			// fmt.Printf("response:%#v\n", pack)
			m := pack.Body.(map[string]any)
			req.out <- msg.NewFrom(m)
		}
	case MsgTypePush:
		c.dataChan <- pack
	case MsgTypeHeartbeat:
	}
}

func (c Client) OnData() <-chan any {
	return c.dataChan
}

func (c Client) OnClose() <-chan any {
	return c.closeChan
}

func (c *Client) Read() (*Pack, error) {
	return decodePack(c.conn)
}

func (c Client) nextId() int64 {
	atomic.AddInt64(&c.reqId, 1)
	return c.reqId
}

func (c Client) Request(route string, data any) <-chan *msg.Msg {
	out := make(chan *msg.Msg)
	pack := Pack{
		Type:  MsgTypeRequest,
		Id:    strconv.FormatInt(c.nextId(), 10),
		Route: route,
		Body:  data,
	}
	err := c.write(&pack)
	if err != nil {
		go func() { out <- msg.NewErr("request error:" + err.Error()) }()
		return out
	}
	timeout := time.AfterFunc(time.Second*10, func() {
		timeoutPack := &Pack{
			Type:  MsgTypeResponse,
			Id:    pack.Id,
			Route: pack.Route,
			Body:  map[string]any{"code": 400, "msg": "request timeout"},
		}
		c.packChan <- timeoutPack
	})
	c.reqs[pack.Id] = &Req{id: pack.Id, out: out, timeout: timeout}
	return out
}

func (c Client) write(pack *Pack) error {
	_, err := c.conn.Write(encodePack(pack))
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() {
	_ = c.conn.Close()
}
