package main

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/codeskyblue/groupcache"
	"github.com/gorilla/websocket"
)

const defaultWSURL = "/_ws/"

var (
	state = ServerState{
		ActiveDownlaod: 0,
		closed:         false,
	}
	peerGroup = PeerGroup{
		m: make(map[string]Peer, 10),
	}

	pool *groupcache.HTTPPool
)

type Peer struct {
	Name           string
	Connection     *websocket.Conn
	ActiveDownload int
}

type PeerGroup struct {
	sync.RWMutex
	peerMap map[string]Peer
}

func (pg *PeerGroup) AddPeer(name string, conn *websocket.Conn) {
	pg.Lock()
	defer pg.Unlock()
	pg.peerMap[name] = Peer{
		Name:       name,
		Connection: conn,
	}
}

func (pg *PeerGroup) RemovePeer(name string) {
	pg.Lock()
	delete(pg.peerMap, name)
	pg.Unlock()
}

func (pg *PeerGroup) Keys() []string {
	pg.RLock()
	defer pg.RUnlock()
	keys := []string{}
	for key, _ := range pg.peerMap {
		keys = append(keys, key)
	}
	return keys
}

func (pg *PeerGroup) PickPeer() (string, error) {
	//FIXME: this picking strategy is stupid, need a fix
	// Hopefully there's a way to validate the peers
	pg.RLock()
	defer pg.RUnlock()
	r := rand.Int()
	keys := []string{}
	for key, _ := range pg.peerMap {
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return "", errors.New("No peers to pick from")
	}
	return keys[r%len(keys)], nil
}

// BroadcastJSON sends a message to all peers in the group
func (pg *PeerGroup) BroadcastJSON(value interface{}) error {
	var err error
	for _, peer := range pg.peerMap {
		if err = peer.Connection.WriteJSON(value); err != nil {
			return err
		}
	}
}

type ServerState struct {
	sync.Mutex
	ActiveDownload int
	closed         bool
}

func (s *ServerState) addActiveDownload(n int) {
	s.Lock()
	defer s.Unlock()
	s.ActiveDownload += n
}

func (s *ServerState) Close() error {
	s.closed = true
	if wsclient != nil {
		wsclient.Close()
	}
	time.Sleep(time.Second * 1)
	for {
		if s.ActiveDownload == 0 {
			break
		}
		time.Sleep(time.Second * 1)
	}
	return nil
}

// FIXME: this is stupid, fix it
func (s *ServerState) isClosed() bool {
	return s.closed
}
