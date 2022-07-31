package net

import (
	"encoding/json"
	"fmt"
	. "github.com/elancom/go-util/lang"
	"testing"
)

func TestName0(t *testing.T) {
	context := NewContext()
	context.Reg(NewLoginModule())
	context.Init()
	// todo 把socket集成到context中, context.Socket = NewSocket...
	context.Start()
	server := NewSocketServer(context, &Config{Port: 8888})
	server.Start()

	// wrapper -> game server
}

var LoginExecutor = NewExecutor()

type LoginModule struct {
	*Module
}

func NewLoginModule() IModule {
	return &LoginModule{Module: NewModule("login")}
}

func (mod *LoginModule) OnInit() {
	mod.Exec = LoginExecutor.Exec
	mod.UserAction("login", mod.login)
}

func (mod *LoginModule) login(s ISession, d map[string]any) *Msg {
	marshal, _ := json.Marshal(d)
	fmt.Println("d:", string(marshal))
	return NewOk()
}
