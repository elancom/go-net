package net

import (
	"encoding/json"
	"fmt"
	"github.com/elancom/go-util/msg"
	"testing"
)

func TestName0(t *testing.T) {
	// wrap to app
	// a:new
	// a:reg(login)
	// a:start("tcp", ":8888")
	context := NewContext()
	context.Reg(NewLoginModule())
	server := NewSocketServer(context, &Config{Port: 8888})
	server.Start()
}

var LoginExecutor = NewExecutor()

type LoginModule struct {
	*Module
}

func NewLoginModule() IModule {
	return &LoginModule{
		Module: NewModule("login"),
	}
}

func (l *LoginModule) OnInit() {
	l.Exec = LoginExecutor.Exec
	l.UserAction("login", l.login)
}

func (l *LoginModule) login(s ISession, p map[string]any) *msg.Msg {
	marshal, _ := json.Marshal(p)
	fmt.Println("p:", string(marshal))
	return msg.NewOk()
}
