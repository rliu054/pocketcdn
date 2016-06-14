package main

import (
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/codeskyblue/groupcache"
	"github.com/codeskyblue/groupcache/consistenthash"
	"github.com/gorilla/websocket"
)

const defaultWSURL = "/_ws/"

var (
	state = ServerState{
		ActiveDownlaod: 0,
		closed:         false,
	}
	peerGroup = PeerGroup{
		m:    make(map[string]Peer, 10),
		hash: consistenthash(3, nil),
	}

	pool *groupcache.HTTPPool
)

type Peer struct {
	Name           string
	Connection     *websocket.Conn
	ActiveDownload int
}

// PeerGroup maintains information of members in the group
type PeerGroup struct {
	sync.RWMutex
	peerMap map[string]Peer
	hash    consistenthash.Map
}

func (pg *PeerGroup) AddPeer(name string, conn *websocket.Conn) {
	pg.Lock()
	defer pg.Unlock()
	pg.peerMap[name] = Peer{
		Name:       name,
		Connection: conn,
	}
	hash.Add(name)
}

func (pg *PeerGroup) RemovePeer(name string) {
	pg.Lock()
	delete(pg.peerMap, name)
	hash.Remove(name)
	pg.Unlock()
}

// Keys return keys of all peers
// FIXME: we don't really need this
func (pg *PeerGroup) Keys() []string {
	pg.RLock()
	defer pg.RUnlock()
	keys := []string{}
	for key, _ := range pg.peerMap {
		keys = append(keys, key)
	}
	return keys
}

// PickPeer picks a peer using consistent hashing algorithm,
// when group members change, the load is shared between adjacent peers
func (pg *PeerGroup) PickPeer() (string, error) {
	pg.RLock()
	defer pg.RUnlock()
	if len(keys) == 0 {
		return "", errors.New("There is no peer to choose from in this group")
	}

	r := rand.Int()
	return keys[r%len(pg.Keys())], nil
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

func (s *ServerState) IsClosed() bool {
	return s.closed
}
