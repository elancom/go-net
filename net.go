package net

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/elancom/go-util/msg"
	"github.com/elancom/go-util/queue"
	"github.com/elancom/go-util/str"
	"io"
	"log"
	"strings"
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
		return nil, nil
	}
	// fmt.Println("receive data:", str(dataBytes))

	p := &Pack{}
	err = json.Unmarshal(dataBytes, &p)
	if err != nil {
		return nil, err
	}
	return p, nil
}

type IBroadcaster interface {
	Broadcast(route string, msg any)
}

type ISingleSender interface {
	Send(route string, msg any, uid ...uint64)
}

type IMsgSender interface {
	IBroadcaster
	ISingleSender
}

type LocalChannelService struct {
	sessionService ISessionService
}

func (c LocalChannelService) Broadcast(route string, msg any) {
	p := Pack{
		Type:  MsgTypePush,
		Id:    "",
		Route: route,
		Body:  msg,
	}
	for _, s := range c.sessionService.All() {
		s.Send(&p)
	}
}

func (c LocalChannelService) Send(route string, msg any, uid ...uint64) {
	p := Pack{
		Type:  MsgTypePush,
		Route: route,
		Body:  msg,
	}
	for _, id := range uid {
		s := c.sessionService.FindByUid(id)
		if s != nil {
			s.Send(&p)
		}
	}
}

type Lifecycle interface {
	OnInit()
	OnStart()
	OnClose()
}

type IEventSubscriber interface {
	OnEvent(event any)
}

type IEventEmitter interface {
	Emit(event any)
}

type EventEmitter struct {
	modules       []IModule
	eventAddQueue *queue.ChanQueue
}

func NewEventEmitter() IEventEmitter {
	return &EventEmitter{
		eventAddQueue: queue.NewChanQueue(),
	}
}

func (e EventEmitter) Start() {
	e.eventAddQueue.Start()
}

func (e EventEmitter) Emit(event any) {
	e.eventAddQueue.Exec(func() {
		// 通知本地
		for _, mod := range e.modules {
			mod.OnEvent(event)
		}
		// 通知远程
	})
}

type RequestDispatcher interface {
	Dispatch(s ISession, pack *Pack)
}

type Dispatcher interface {
	RequestDispatcher
}

type LocalDispatcher struct {
	modules []IModule
}

func (d *LocalDispatcher) Dispatch(s ISession, pack *Pack) {
	marshal, _ := json.Marshal(pack)
	log.Default().Println("[数据]", string(marshal))
	d.dispatch0(s, pack, func(msg *msg.Msg) {
		p := Pack{
			Type:  MsgTypeResponse,
			Id:    pack.Id,
			Route: pack.Route,
			Body:  msg,
		}
		s.Send(&p)
	})
}

func (d *LocalDispatcher) dispatch0(session ISession, pack *Pack, cb callback) {
	// 包ID
	id := strings.Trim(pack.Id, "")
	if len(id) == 0 {
		cb(msg.NewErr("Id error"))
		return
	}

	// 路由
	route := pack.Route
	if len(route) == 0 {
		cb(msg.NewErr("route error"))
		return
	}

	// 提取模块名称
	mid := GetModuleId(route)
	if str.IsBlank(mid) {
		cb(msg.NewErr("route error(.)"))
		return
	}

	// 模块
	mod := d.FindModule(mid)
	if mod == nil {
		cb(msg.NewErr("no find route(" + route + ")"))
		return
	}

	// 转给模块处理
	mod.HandleUserAction(route, session, pack, cb)
}

func (d *LocalDispatcher) FindModule(id string) (module IModule) {
	return FindModule(d.modules, id)
}

type IModule interface {
	Lifecycle
	IEventSubscriber
	ID(id ...string) string
	HandleApiAction(route string, pack *Pack, cb callback)
	HandleUserAction(route string, s ISession, pack *Pack, cb callback)
}

func GetModuleId(route string) string {
	idx := strings.Index(route, ".")
	if idx == -1 {
		return ""
	}
	return route[0:idx]
}

func FindModule(modules []IModule, id string) (module IModule) {
	for i := 0; i < len(modules); i++ {
		if modules[i].ID() == id {
			module = modules[i]
			break
		}
	}
	return module
}

type UserAction func(s ISession, p map[string]any) *msg.Msg
type ApiAction func(p map[string]any) *msg.Msg

type callback func(*msg.Msg)

type userActionReq struct {
	sn     ISession
	action UserAction
	pack   *Pack
	cb     callback
}

func (a userActionReq) call() {
	body := a.pack.Body
	a.cb(a.action(a.sn, body.(map[string]any)))
}

type apiActionReq struct {
	action ApiAction
	pack   *Pack
	cb     callback
}

func (a apiActionReq) call() {
	body := a.pack.Body
	a.cb(a.action(body.(map[string]any)))
}

type GoModule struct {
	Id          string
	userActions map[string]UserAction
	apiActions  map[string]ApiAction
	// 外部传递吧
	eventChan      chan any
	userActReqChan chan *userActionReq
	apiActReqChan  chan *apiActionReq
	exitChan       chan any
	closed         bool
	// 事件派发器
	emitter IEventEmitter
}

func NewGoModule(id string) *GoModule {
	g := GoModule{Id: id}
	g.userActions = make(map[string]UserAction)
	g.apiActions = make(map[string]ApiAction)
	g.eventChan = make(chan any)
	g.userActReqChan = make(chan *userActionReq, 0)
	g.apiActReqChan = make(chan *apiActionReq, 0)
	return &g
}

func (g *GoModule) OnInit() {
}

func (g *GoModule) OnStart() {
	go g.read()
}

func (g *GoModule) read() {
	for {
		// fmt.Println("read...")
		select {
		case userActReq := <-g.userActReqChan:
			userActReq.call()
		case apiActReq := <-g.apiActReqChan:
			apiActReq.call()
		case event := <-g.eventChan:
			fmt.Println("event:", event)
		case <-g.exitChan:
			goto end
		}
	}
end:
	fmt.Println("module close")
	g.closed = true
	g.exitChan <- 1
}

func (g *GoModule) OnClose() {}

func (g *GoModule) OnEvent(event any) {
	if g.closed {
		return
	}
	g.eventChan <- event
}

func (g *GoModule) HandleApiAction(route string, pack *Pack, cb callback) {
	// 模块已关闭
	if g.closed {
		cb(msg.NewErr("modules is closed"))
		return
	}
	// 处理器
	action := g.apiActions[route]
	if action == nil {
		cb(msg.NewErr("action not found"))
		return
	}
	// 转入通道
	g.apiActReqChan <- &apiActionReq{
		action: action,
		pack:   pack,
		cb:     cb,
	}
}

func (g *GoModule) HandleUserAction(route string, sn ISession, pack *Pack, cb callback) {
	// 模块已关闭
	if g.closed {
		cb(msg.NewErr("modules is closed"))
		return
	}
	// 处理器
	action := g.userActions[route]
	if action == nil {
		cb(msg.NewErr("action not found"))
		return
	}
	// 转入通道
	g.userActReqChan <- &userActionReq{
		sn:     sn,
		action: action,
		pack:   pack,
		cb:     cb,
	}
}

func (g *GoModule) ID(id ...string) string {
	if len(id) > 0 {
		g.Id = id[0]
	}
	return g.Id
}

func (g *GoModule) UserAction(route string, action ...UserAction) UserAction {
	if len(action) > 0 {
		g.userActions[g.ID()+"."+route] = action[0]
	}
	return g.userActions[route]
}

func (g *GoModule) ApiAction(route string, action ...ApiAction) ApiAction {
	if len(action) > 0 {
		g.apiActions[g.ID()+"."+route] = action[0]
	}
	return g.apiActions[route]
}

type Config struct {
	Port   int
	WSPort int
}

type Context struct {
	SessionService ISessionService
	Dispatcher     Dispatcher
	MsgSender      IMsgSender
	EventEmitter   IEventEmitter
	modules        []IModule
}

func NewContext() *Context {
	c := Context{}
	c.modules = []IModule{}
	c.SessionService = &SessionManager{}
	c.Dispatcher = &LocalDispatcher{}
	c.MsgSender = &LocalChannelService{
		sessionService: c.SessionService,
	}
	c.EventEmitter = NewEventEmitter()
	return &c
}

func (c *Context) Init() {
	dispatcher, ok := c.Dispatcher.(*LocalDispatcher)
	if ok {
		dispatcher.modules = c.modules
	}
	eventEmitter, ok := c.EventEmitter.(*EventEmitter)
	if ok {
		eventEmitter.modules = c.modules
	}
	sessionManager, ok := c.SessionService.(*SessionManager)
	if ok {
		sessionManager.Init()
	}
	for _, module := range c.modules {
		module.OnInit()
	}
}

func (c *Context) FindModule(id string) (module IModule) {
	return FindModule(c.modules, id)
}

func (c *Context) Start() {
	go func() { c.EventEmitter.(*EventEmitter).Start() }()
	for _, module := range c.modules {
		module.OnStart()
	}
}

func (c *Context) Reg(module IModule) {
	c.modules = append(c.modules, module)
}
