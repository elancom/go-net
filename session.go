package net

import (
	"github.com/elancom/go-util/str"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	SessionCreatedEvent = "session_created"
	SessionLoginEvent   = "session_login"
	SessionClosedEvent  = "session_closed"
)

type ISession interface {
	ID() string
	UID() uint64
	IP() string
	CTime() *time.Time
	Send(pack *Pack)
}

type ILocalSession interface {
	IsLogged() bool
	Login(uid uint64)
	Logout()
	Start()
	Close() <-chan any
	IsClosed() bool
}

type LocalSession struct {
	id         string
	uid        uint64
	ip         string
	ctime      time.Time
	logged     bool
	readChan   chan any
	writeChan  chan *Pack // todo -> []byte
	closeChan  chan any
	conn       *net.TCPConn
	closed     bool
	bindNotify func()
}

func newSession(conn *net.TCPConn) ISession {
	s := LocalSession{}
	s.id = strconv.FormatInt(time.Now().UnixNano(), 10)
	s.ip = conn.RemoteAddr().Network()
	s.ctime = time.Time{}
	s.readChan = make(chan any, 32)
	s.writeChan = make(chan *Pack, 32)
	s.closeChan = make(chan any)
	s.conn = conn
	return &s
}

func (s *LocalSession) ID() string {
	return s.id
}

func (s *LocalSession) UID() uint64 {
	return s.uid
}

func (s *LocalSession) IP() string {
	return s.ip
}

func (s *LocalSession) CTime() *time.Time {
	return &s.ctime
}

func (s *LocalSession) Login(uid uint64) {
	if s.logged {
		return
	}
	if uid <= 0 {
		panic("zero uid")
	}
	s.uid = uid
	s.logged = true
	if s.bindNotify != nil {
		s.bindNotify()
	}
}

func (s *LocalSession) Logout() {
	s.logged = false
}

func (s LocalSession) IsLogged() bool {
	return s.logged
}

func (s *LocalSession) Start() {
	for {
		select {
		case pack := <-s.writeChan:
			_, err := s.conn.Write(encodePack(pack))
			if err != nil {
				log.Default().Println(err)
				break
			}
		case <-s.closeChan:
			goto close
		}
	}
close:
	s.closed = true
	s.closeChan <- 2
	close(s.closeChan)
}

func (s *LocalSession) Send(msg *Pack) {
	s.writeChan <- msg
}

func (s *LocalSession) Close() <-chan any {
	s.closeChan <- 1
	return s.closeChan
}

func (s *LocalSession) IsClosed() bool {
	return s.closed
}

type ISessionService interface {
	Add(s ISession)
	RemoveIfExist(uid uint64) bool
	Remove(s ISession)
	FindByUid(uid uint64) ISession
	All() map[string]ISession
}

type SessionManager struct {
	lock   sync.RWMutex
	sidMap map[string]ISession
	uidMap map[string]ISession
}

func (s2 *SessionManager) Init() {
	s2.sidMap = make(map[string]ISession)
	s2.uidMap = make(map[string]ISession)
}

func (s2 *SessionManager) Add(s ISession) {
	s2.lock.Lock()
	defer s2.lock.Unlock()
	if s2.sidMap[s.ID()] == nil {
		s2.sidMap[s.ID()] = s
	}
	if s.UID() != 0 {
		s2.Bind(s)
	} else {
		session := s.(*LocalSession)
		session.bindNotify = func() { s2.Bind(s) }
	}
}

func (s2 *SessionManager) RemoveIfExist(uid uint64) bool {
	s2.lock.RLock()
	defer s2.lock.RUnlock()
	s := s2.uidMap[str.ToString(uid)]
	if s == nil {
		return false
	}
	s2.Remove(s)
	return true
}

func (s2 *SessionManager) Bind(s ISession) {
	uid := s.UID()
	if uid == 0 {
		return
	}
	s2.lock.Lock()
	defer s2.lock.Unlock()
	if s2.sidMap[s.ID()] != nil {
		s2.uidMap[str.ToString(uid)] = s
	} else {
		delete(s2.uidMap, str.ToString(uid))
	}
}

func (s2 *SessionManager) Remove(s ISession) {
	s2.lock.Lock()
	defer s2.lock.Unlock()
	if s2.sidMap[s.ID()] == nil {
		return
	}
	delete(s2.sidMap, s.ID())
	uid := s.UID()
	if uid != 0 {
		if s == s2.uidMap[str.ToString(uid)] {
			delete(s2.uidMap, str.ToString(s.UID()))
		}
	}
}

func (s2 *SessionManager) FindByUid(uid uint64) ISession {
	return s2.uidMap[str.ToString(uid)]
}

func (s2 *SessionManager) All() map[string]ISession {
	return s2.sidMap
}
