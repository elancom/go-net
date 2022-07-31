package net

import (
	. "github.com/elancom/go-util/lang"
	"github.com/gofiber/fiber/v2"
	"log"
)

type CallParam struct {
	Route string
	Data  map[string]any
}

type ApiServer struct {
	context *Context
	app     *fiber.App
}

func NewApiServer() *ApiServer {
	return &ApiServer{}
}

func (a *ApiServer) Init(context *Context) {
	a.context = context
	a.app = fiber.New()
	a.app.Get("/", func(c *fiber.Ctx) error { return c.JSON(NewOk()) })
	a.app.Get("/online", a.online)
	a.app.Post("/call", a.call)
}

func (a *ApiServer) Start() {
	log.Default().Println("[API]running")
	log.Fatal(a.app.Listen(":3456"))
}

// 在线会员
func (a *ApiServer) online(c *fiber.Ctx) error {
	all := a.context.SessionService.All()
	ssMap := make([]any, 0, len(all))
	for _, v := range all {
		ssMap = append(ssMap, map[string]any{
			"id":    v.ID(),
			"uid":   v.UID(),
			"ip":    v.IP(),
			"ctime": v.CTime(),
		})
	}
	return c.JSON(ssMap)
}

// 接口调用
func (a *ApiServer) call(c *fiber.Ctx) error {
	callParam := CallParam{}
	err := c.BodyParser(&callParam)
	if err != nil {
		return c.JSON(NewErr("args error"))
	}

	route := callParam.Route
	data := callParam.Data
	if route == "" {
		return c.JSON(NewErr("route error"))
	}

	module := a.context.FindModule(GetModuleId(route))
	if module == nil {
		return c.JSON(NewErr("route error(.)"))
	}

	pack := Pack{
		Type:  MsgTypeRequest,
		Route: route,
		Body:  data,
	}
	msgChan := make(chan any)
	go func() {
		module.HandleApiAction(route, &pack, func(m *Msg) { msgChan <- m })
	}()
	return c.JSON(<-msgChan)
}
