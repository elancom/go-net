package net

import . "github.com/elancom/go-util/lang"

type Action func(s ISession, p map[string]any) *Msg
type ApiAction func(p map[string]any) *Msg

type callback func(msg *Msg)

type userActionReq struct {
	sn     ISession
	action Action
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

func (a *apiActionReq) call() {
	body := a.pack.Body
	a.cb(a.action(body.(map[string]any)))
}
