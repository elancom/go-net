package net

import (
	"encoding/json"
	. "github.com/elancom/go-util/lang"
	"github.com/elancom/go-util/str"
	"log"
	"strings"
)

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
	d.dispatch(s, pack, func(msg *Msg) {
		p := Pack{
			Type:  MsgTypeResponse,
			Id:    pack.Id,
			Route: pack.Route,
			Body:  msg,
		}
		s.Send(&p)
	})
}

func (d *LocalDispatcher) dispatch(session ISession, pack *Pack, cb callback) {
	// 包ID
	id := strings.Trim(pack.Id, "")
	if len(id) == 0 {
		cb(NewErr("Id error"))
		return
	}

	// 路由
	route := pack.Route
	if len(route) == 0 {
		cb(NewErr("route error"))
		return
	}

	// 提取模块名称
	mid := GetModuleId(route)
	if str.IsBlank(mid) {
		cb(NewErr("route error(.)"))
		return
	}

	// 模块
	mod := d.FindModule(mid)
	if mod == nil {
		cb(NewErr("no find route(" + route + ")"))
		return
	}

	// 转给模块处理
	mod.HandleUserAction(route, session, pack, cb)
}

func (d *LocalDispatcher) FindModule(id string) (module IModule) {
	return FindModule(d.modules, id)
}
