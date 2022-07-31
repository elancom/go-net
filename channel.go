package net

type Channel interface {
	Write([]byte) error
	Close()
}
