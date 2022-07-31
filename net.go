package net

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

func (c *LocalChannelService) Broadcast(route string, msg any) {
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

func (c *LocalChannelService) Send(route string, msg any, uid ...uint64) {
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
