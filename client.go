package net

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/elancom/go-util/lang"
	"github.com/elancom/go-util/str"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type ClientConfig struct{}

type Req struct {
	id      string
	out     chan<- *Msg
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
	conn  net.Conn
	reqId int64
	//reqs       map[string]*Req
	reqs       *sync.Map
	reqMap     *sync.Map
	taskChan   chan any // 响应队列
	onDataChan chan any // 推送队列
	closeChan  chan any // 关闭通道
	echoTime   *time.Time
}

func NewClient() *Client {
	c := Client{}
	//c.reqs = make(map[string]*Req)
	c.reqs = new(sync.Map)
	//c.reqMap = new(sync.Map)
	c.taskChan = make(chan any, 16)
	c.closeChan = make(chan any, 16)
	c.onDataChan = make(chan any, 16)
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
	ioClosedChan := make(chan int)
	go func() {
		for {
			pack, err := c.Read()
			if err != nil {
				break
			}
			c.taskChan <- pack
		}
		ioClosedChan <- 1
	}()
	// handle msg
loop:
	for {
		select {
		case t := <-c.taskChan:
			switch t.(type) {
			case *Pack: // 数据包(响应包、推送包)
				c.handlePack(t.(*Pack))
			case *timeout: // 超时包
				tout := t.(*timeout)
				andDelete, loaded := c.reqs.LoadAndDelete(tout.Id)
				if loaded {
					req := andDelete.(*Req)
					req.out <- NewErr("request timeout")
				}
			}
		case <-ioClosedChan:
			break loop
		}
	}
	c.closeChan <- 1
}

func (c *Client) handlePack(pack *Pack) {
	switch pack.Type {
	case MsgTypeResponse:
		andDelete, loaded := c.reqs.LoadAndDelete(pack.Id)
		if loaded {
			req := andDelete.(*Req)
			req.timeout.Stop()
			m := pack.Body.(map[string]any)
			req.out <- NewFrom(m)
		} else {
			log.Println("not found req handler:" + pack.Id)
		}
	case MsgTypePush:
		c.onDataChan <- pack
	case MsgTypeHeartbeat:
		// 忽略
	}
}

func (c *Client) OnData() <-chan any {
	return c.onDataChan
}

func (c *Client) OnClose() <-chan any {
	return c.closeChan
}

func (c *Client) Read() (*Pack, error) {
	return decodePack(c.conn)
}

func (c *Client) nextId() int64 {
	atomic.AddInt64(&c.reqId, 1)
	return c.reqId
}

func (c *Client) Request(route string, data any) <-chan *Msg {
	out := make(chan *Msg)
	pack := Pack{
		Type:  MsgTypeRequest,
		Id:    str.String(c.nextId()),
		Route: route,
		Body:  data,
	}

	// 请求队列
	c.reqs.Store(pack.Id, &Req{
		id:      pack.Id,
		out:     out,
		timeout: time.AfterFunc(time.Second*10, func() { c.taskChan <- &timeout{Id: pack.Id} }),
	})

	// 发送
	err := c.write(&pack)
	if err != nil {
		go func() {
			out <- NewErr("request error:" + err.Error())
		}()
		return out
	}
	return out
}

func (c *Client) write(pack *Pack) error {
	_, err := c.conn.Write(encodePack(pack))
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() {
	_ = c.conn.Close()
}
