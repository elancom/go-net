package net

import (
	"fmt"
	. "github.com/elancom/go-util/lang"
	"github.com/elancom/go-util/queue"
	"strings"
)

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

type IModule interface {
	Lifecycle
	IEventSubscriber
	ID(id ...string) string
	HandleApiAction(route string, pack *Pack, cb callback)
	HandleUserAction(route string, s ISession, pack *Pack, cb callback)
}

type Module struct {
	Id string

	// 执行器
	Exec func(func())

	// 请求记录
	actions    map[string]Action
	apiActions map[string]ApiAction

	// 模块任务处理队列
	taskChan chan any

	// 事件处理
	eventChan chan any

	// actReqChan    chan *userActionReq
	// apiActReqChan chan *apiActionReq
	exitChan chan any
	closed   bool
	// 事件派发器
	emitter IEventEmitter
}

func NewModule(id string) *Module {
	g := Module{Id: id}
	g.actions = make(map[string]Action)
	g.apiActions = make(map[string]ApiAction)
	g.taskChan = make(chan any, 16)
	return &g
}

// OnInit 子类覆盖
func (g *Module) OnInit() {}

func (g *Module) OnStart() {
	go g.readTask()
}

func (g *Module) readTask() {
	for {
		select {
		case t := <-g.taskChan: // 执行任务
			// 函数任务
			if fn, ok := t.(func()); ok {
				if g.Exec != nil {
					g.Exec(fn)
				} else {
					fn()
				}
			} else { // 事件
				fmt.Println("event:", t)
			}
		case <-g.exitChan:
			goto end
		}
	}
end:
	fmt.Println("module close")
	g.closed = true
	g.exitChan <- 1
}

func (g *Module) OnClose() {}

func (g *Module) OnEvent(event any) {
	if g.closed {
		return
	}
	g.eventChan <- event
}

func (g *Module) HandleApiAction(route string, pack *Pack, cb callback) {
	// 模块已关闭
	if g.closed {
		cb(NewErr("modules is closed"))
		return
	}
	// 处理器
	action := g.apiActions[route]
	if action == nil {
		cb(NewErr("action not found"))
		return
	}
	// 放到任务队列
	req := &apiActionReq{
		action: action,
		pack:   pack,
		cb:     cb,
	}
	g.push(func() { req.call() })
}

func (g *Module) HandleUserAction(route string, sn ISession, pack *Pack, cb callback) {
	// 模块已关闭
	if g.closed {
		cb(NewErr("module was closed"))
		return
	}
	// 处理器
	action := g.actions[route]
	if action == nil {
		cb(NewErr("action not found"))
		return
	}
	// 放到任务队列
	req := &userActionReq{
		session: sn,
		action:  action,
		pack:    pack,
		cb:      cb,
	}
	g.push(func() { req.call() })
}

func (g *Module) push(t any) {
	g.taskChan <- t
	size := len(g.taskChan)
	if size > 0 {
		fmt.Println("taskChan size:", size)
	}
}

func (g *Module) ID(id ...string) string {
	if len(id) > 0 {
		g.Id = id[0]
	}
	return g.Id
}

func (g *Module) UserAction(route string, action ...Action) Action {
	if len(action) > 0 {
		g.actions[g.ID()+"."+route] = action[0]
	}
	return g.actions[route]
}

func (g *Module) ApiAction(route string, action ...ApiAction) ApiAction {
	if len(action) > 0 {
		g.apiActions[g.ID()+"."+route] = action[0]
	}
	return g.apiActions[route]
}

func NewEventEmitter() IEventEmitter {
	return &EventEmitter{
		eventAddQueue: queue.NewChanQueue(),
	}
}

type EventEmitter struct {
	modules       []IModule
	eventAddQueue *queue.ChanQueue
}

func (e *EventEmitter) Start() {
	e.eventAddQueue.Start()
}

func (e *EventEmitter) Emit(event any) {
	e.eventAddQueue.Exec(func() {
		// 通知本地
		for _, mod := range e.modules {
			mod.OnEvent(event)
		}
		// 通知远程
	})
}
