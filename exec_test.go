package net

import (
	"fmt"
	"testing"
	"time"
)

var TestExecutor = NewExecutor()

func TestName2(t *testing.T) {
	for i := 0; i < 10; i++ {
		i2 := i
		TestExecutor.Exec(func() {
			fmt.Println("hello", i2)
			time.Sleep(time.Second)
		})
	}
	time.Sleep(time.Hour)
}

func TestName3(t *testing.T) {
	after := TestExecutor.After(time.Second, func() {
		fmt.Println("go 3s!")
	})
	after.Stop()
	fmt.Println(after)
	time.Sleep(time.Hour)
}

func TestName4(t *testing.T) {
	tick := TestExecutor.Ticker(time.Second, func(ticker *time.Ticker, t *time.Time) {
		fmt.Println(t.Second())
	})
	go func() {
		time.Sleep(time.Second * 3)
		TestExecutor.Shutdown()
		tick.Stop()
		fmt.Println(tick)
		go func() {
			time.Sleep(time.Second * 1)
			// TestExecutor.Shutdown()
		}()
	}()
	time.Sleep(time.Hour)
}
