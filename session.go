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
	logged     bool // 登录标记
	readChan   chan any
	writeChan  chan *Pack // todo -> []byte
	closeChan  chan any
	conn       *net.TCPConn // todo 抽象
	closed     bool
	bindNotify func()
}

func newSession(conn *net.TCPConn) ISession {
	s := LocalSession{}
	s.id = strconv.FormatInt(time.Now().UnixNano(), 10)
	s.ip = conn.RemoteAddr().Network()
	s.ctime = time.Now()
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

func (sm *SessionManager) Init() {
	sm.sidMap = make(map[string]ISession)
	sm.uidMap = make(map[string]ISession)
}

func (sm *SessionManager) Add(s ISession) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if sm.sidMap[s.ID()] == nil {
		sm.sidMap[s.ID()] = s
	}
	if s.UID() != 0 {
		sm.Bind(s)
	} else {
		session := s.(*LocalSession)
		session.bindNotify = func() { sm.Bind(s) }
	}
}

func (sm *SessionManager) RemoveIfExist(uid uint64) bool {
	sm.lock.RLock()
	defer sm.lock.RUnlock()
	s := sm.uidMap[str.String(uid)]
	if s == nil {
		return false
	}
	sm.Remove(s)
	return true
}

func (sm *SessionManager) Bind(s ISession) {
	uid := s.UID()
	if uid == 0 {
		return
	}
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if sm.sidMap[s.ID()] != nil {
		sm.uidMap[str.String(uid)] = s
	} else {
		delete(sm.uidMap, str.String(uid))
	}
}

func (sm *SessionManager) Remove(s ISession) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if sm.sidMap[s.ID()] == nil {
		return
	}
	delete(sm.sidMap, s.ID())
	uid := s.UID()
	if uid != 0 {
		if s == sm.uidMap[str.String(uid)] {
			delete(sm.uidMap, str.String(s.UID()))
		}
	}
}

func (sm *SessionManager) FindByUid(uid uint64) ISession {
	return sm.uidMap[str.String(uid)]
}

func (sm *SessionManager) All() map[string]ISession {
	return sm.sidMap
}
