package net

import (
	"fmt"
	"testing"
)

func TestClient(t *testing.T) {
	// 上层判断是否正在连接中
	// 传递:推送数据通道
	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
	// 启动连接线程
	// 参数是:OnData()
	client := NewClient()
	err := client.Connect()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("[连接]连接成功")

	go func() {
		testLogin(client)
	}()

	for {
		select {
		case data := <-client.OnData():
			fmt.Println("数据:", data)
		case <-client.OnClose():
			goto end
		}
	}
end:
	fmt.Println("已关闭")
}

func testLogin(client *Client) {
	//client.Close()
	msg := <-client.Request("login.login", map[string]any{"username": "张三", "password": "123456"})
	if msg.IsErr() {
		fmt.Println("错误:", msg.Msg)
		return
	}
	fmt.Println("返回:", msg)
}
