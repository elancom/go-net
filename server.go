package net

import (
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
	listen, err := net.Listen("tcp", ":8888")
	if err != nil {
		panic(err)
	}
	defer func(listen net.Listener) { _ = listen.Close() }(listen)
	log.Default().Println("[Socket]running")
	for {
		accept, err := listen.Accept()
		if err != nil {
			log.Default().Println(err)
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
		log.Default().Println(err)
		return
	}

	// 会话
	ss := newSession(conn.(*net.TCPConn))
	go ss.(ILocalSession).Start()
	s.context.SessionService.Add(ss)

	s.context.EventEmitter.Emit(map[string]any{"event": SessionCreatedEvent})

	// 消息接收
	for {
		pack, err := decodePack(conn)
		if err != nil {
			break
		}
		s.context.Dispatcher.Dispatch(ss, pack)
	}

	// 会话
	s.context.SessionService.Remove(ss)

	// 通知
	s.context.EventEmitter.Emit(map[string]any{
		"event": SessionClosedEvent,
		"uid":   ss.UID(),
	})
}
