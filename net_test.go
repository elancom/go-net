package net

import (
	"bytes"
	"io"
	"log"
	"testing"
)

func TestName(t *testing.T) {
	b2 := []byte{1, 2, 3}
	reader := bytes.NewReader(b2)
	b := make([]byte, 4)

	bl, err := io.ReadFull(reader, b)
	if err != nil {
		b3 := make([]byte, 3)
		bl2, e := io.ReadFull(reader, b3)
		log.Println(bl2, e)
		panic(err)
	}
	log.Println(bl)
}
