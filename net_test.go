package net

import (
	"fmt"
	"github.com/elancom/go-util/collection"
	"testing"
)

func TestName(t *testing.T) {
	var aa any = make(map[string]any)
	mp := collection.Params(aa.(map[string]any))
	fmt.Println(mp)
}
