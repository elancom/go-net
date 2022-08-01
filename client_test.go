package net

import (
	"fmt"
	"github.com/elancom/go-util/json"
	"github.com/elancom/go-util/str"
	"log"
	"sync"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	go TestClient0(t)
	time.Sleep(time.Hour)
}
func TestClient0(t *testing.T) {
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
		wg := sync.WaitGroup{}
		for j := 0; j < 20; j++ {
			wg.Add(1)
			go func() {
				for i := 0; i < 100; i++ {
					testLogin(client, i)
				}
				wg.Done()
			}()
		}
		wg.Wait()
		log.Println("OK")
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

func testLogin(client *Client, i int) {
	msg := <-client.Request("login.login", map[string]any{"username": "张三" + str.String(i), "password": "123456"})
	if msg.IsErr() {
		fmt.Println("错误:", msg.Msg)
		return
	}
	log.Println("返回:", json.ToJsonStr(msg))
}
