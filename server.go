package net

import (
	"github.com/elancom/go-util/str"
	"log"
	"net"
)

type SocketServer struct {
	config  *Config
	context *Context
}

func NewSocketServer(context *Context, config *Config) *SocketServer {
	return &SocketServer{
		context: context,
		config:  config,
	}
}

func (s *SocketServer) Start() {
	listen, err := net.Listen("tcp", ":"+str.String(s.config.Port))
	if err != nil {
		panic(err)
	}
	defer func(listen net.Listener) { _ = listen.Close() }(listen)
	log.Println("[Socket]running on " + str.String(s.config.Port))
	for {
		accept, err := listen.Accept()
		if err != nil {
			log.Println(err)
		}
		s.accept(accept)
	}
}

func (s *SocketServer) accept(conn net.Conn) {
	go s.accept0(conn)
}

func (s *SocketServer) accept0(conn net.Conn) {
	_ = conn.(*net.TCPConn).SetNoDelay(true)
	_ = conn.(*net.TCPConn).SetKeepAlive(true)
	defer func(conn net.Conn) { _ = conn.Close() }(conn)

	// 握手
	pack := Pack{
		Type: MsgTypeHandshake,
		Body: map[string]any{"v": "100"},
	}
	_, err := conn.Write(encodePack(&pack))
	if err != nil {
		log.Println(err)
		return
	}

	// TODO 握手超时

	// 会话
	ss := newSession(conn.(*net.TCPConn))
	go ss.(ILocalSession).Start()
	s.context.SessionService.Add(ss)

	// 回话事件(创建)
	s.context.EventEmitter.Emit(map[string]any{"event": SessionCreatedEvent})

	// 消息接收
	for {
		p, e := decodePack(conn)
		if e != nil {
			break
		}
		s.context.Dispatcher.Dispatch(ss, p)
	}

	// 会话
	s.context.SessionService.Remove(ss)

	// 回话事件(关闭)
	s.context.EventEmitter.Emit(map[string]any{
		"event": SessionClosedEvent,
		"uid":   ss.UID(),
	})
}
