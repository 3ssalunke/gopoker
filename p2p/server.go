package p2p

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"sync"

	"github.com/sirupsen/logrus"
)

type GameVariant uint

const (
	TexasHoldem GameVariant = iota
	Stud
	Draw
)

func (gv GameVariant) String() string {
	switch gv {
	case TexasHoldem:
		return "Texas Holdem"
	case Stud:
		return "Stud"
	case Draw:
		return "Draw"
	default:
		return "Unknown"
	}
}

type ServerConfig struct {
	Version       string
	ListenAddr    string
	ApiListenAddr string
	GameVariant   GameVariant
}

type Server struct {
	ServerConfig

	transport   *TCPTransport
	peers       map[string]*Peer
	peerLock    sync.RWMutex
	addPeer     chan *Peer
	delPeer     chan *Peer
	msgCh       chan *Message
	gameState   *Game
	broadcastCh chan BroadcastTo
}

func NewServer(cfg ServerConfig) *Server {
	s := &Server{
		ServerConfig: cfg,
		peers:        make(map[string]*Peer),
		addPeer:      make(chan *Peer, 10),
		delPeer:      make(chan *Peer, 10),
		msgCh:        make(chan *Message, 10),
		broadcastCh:  make(chan BroadcastTo, 10),
	}

	// s.gameState = NewGameState(s.ListenAddr, s.broadcastCh)
	s.gameState = NewGame(s.ListenAddr, s.broadcastCh)

	// if s.ListenAddr == ":3000" {
	// 	s.gameState.isDealer = true
	// }

	transport := NewTCPTransport(s.ListenAddr)
	s.transport = transport

	transport.AddPeer = s.addPeer
	transport.DelPeer = s.delPeer

	go func(s *Server) {
		apiServer := NewAPIServer(cfg.ApiListenAddr, s.gameState)
		apiServer.Run()
	}(s)

	return s
}

func (s *Server) Start() {
	go s.loop()

	logrus.WithFields(logrus.Fields{
		"port":    s.ListenAddr,
		"variant": s.GameVariant,
		// "gameStatus": s.gameState.gameStatus,
	}).Info("p2p server started running")

	s.transport.ListenAndAccept()
}

func (s *Server) loop() {
	for {
		select {
		case peer := <-s.addPeer:
			if err := s.handleNewPeer(peer); err != nil {
				logrus.Errorf("handle new peer error: %s", err)
			}

		case peer := <-s.delPeer:
			logrus.WithFields(logrus.Fields{
				"addr": peer.conn.RemoteAddr(),
			}).Info("player disconnected")

			delete(s.peers, peer.listenAddr)

		case msg := <-s.msgCh:
			if err := s.handleMessage(msg); err != nil {
				panic(err)
			}

		case msg := <-s.broadcastCh:
			if err := s.Broadcast(msg); err != nil {
				logrus.Errorf("broadcast error: %s", err)
			}
		}
	}
}

func (s *Server) handleNewPeer(peer *Peer) error {
	hs, err := s.handshake(peer)

	if err != nil {
		peer.conn.Close()
		delete(s.peers, peer.listenAddr)
		return fmt.Errorf("handshake with incoming player failed - %s", err)
	}

	go peer.ReadLoop(s.msgCh)

	if !peer.outbound {
		if err := s.SendHandshake(peer); err != nil {
			peer.conn.Close()
			delete(s.peers, peer.listenAddr)
			return fmt.Errorf("failed to complete the handshake - %s", err)
		}

		go func() {
			if err := s.sendPeerList(peer); err != nil {
				logrus.Errorf("failed to send peerlist - %s", err)
			}
		}()
	}

	logrus.WithFields(logrus.Fields{
		"addr":           s.ListenAddr,
		"peer":           peer.conn.RemoteAddr(),
		"version":        hs.Version,
		"variant":        hs.GameVariant,
		"gameStatus":     hs.GameStatus,
		"peerListenAddr": hs.ListenAddr,
	}).Info("handshake completed, new player connected")

	s.AddPeer(peer)

	// s.gameState.AddPlayer(peer.listenAddr, hs.GameStatus)
	s.gameState.AddPlayer(peer.listenAddr)

	return nil
}

func (s *Server) SendHandshake(p *Peer) error {
	hs := &Handshake{
		Version:     s.Version,
		GameVariant: s.GameVariant,
		GameStatus:  s.gameState.currentStatus,
		ListenAddr:  s.ListenAddr,
	}

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(hs); err != nil {
		return err
	}

	return p.Send(buf.Bytes())
}

func (s *Server) handshake(p *Peer) (*Handshake, error) {
	hs := &Handshake{}
	if err := gob.NewDecoder(p.conn).Decode(hs); err != nil {
		return nil, err
	}

	if s.Version != hs.Version {
		return nil, fmt.Errorf("invalid game version %s", hs.Version)
	}
	if s.GameVariant != hs.GameVariant {
		return nil, fmt.Errorf("invalid game variant %s", hs.GameVariant)
	}

	p.listenAddr = hs.ListenAddr

	return hs, nil
}

func (s *Server) sendPeerList(p *Peer) error {
	peerList := MessagePeerList{
		Peers: s.Peers(),
	}

	msg := NewMessage(s.ListenAddr, peerList)

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	return p.Send(buf.Bytes())
}

func (s *Server) Peers() []string {
	s.peerLock.RLock()
	defer s.peerLock.RUnlock()

	peers := make([]string, len(s.peers))
	it := 0
	for _, peer := range s.peers {
		peers[it] = peer.listenAddr
		it++
	}

	return peers
}

func (s *Server) AddPeer(p *Peer) {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()

	s.peers[p.listenAddr] = p
}

func (s *Server) handleMessage(msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessagePeerList:
		return s.handlePeerList(v)
	case MessageReady:
		return nil
	default:
		return fmt.Errorf("invalid payload type")
	}
}

func (s *Server) handleEncDeck(from string, msg MessageEncDeck) error {
	logrus.WithFields(logrus.Fields{
		"addr":    s.ListenAddr,
		"from":    from,
		"payload": msg,
	}).Info("received enc deck message")
	// return s.gameState.ShuffleAndEncrypt(from, msg.Deck)
	return nil
}

func (s *Server) handlePeerList(l MessagePeerList) error {
	for i := 0; i < len(l.Peers); i++ {
		if err := s.Connect(l.Peers[i]); err != nil {
			logrus.Errorf("failed to dial peer: %s", err)
			continue
		}
	}

	return nil
}

func (s *Server) Connect(addr string) error {
	if s.isInPeerList(addr) {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"dialTo": addr,
		"from":   s.ListenAddr,
	}).Info("peer dial")

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	peer := &Peer{
		conn:     conn,
		outbound: true,
	}

	s.addPeer <- peer

	return s.SendHandshake(peer)
}

func (s *Server) isInPeerList(addr string) bool {
	peers := s.Peers()
	if s.ListenAddr == addr {
		return true
	}
	for _, peerAddr := range peers {
		if peerAddr == addr {
			return true
		}
	}

	return false
}

func (s *Server) Broadcast(broadcastTo BroadcastTo) error {
	msg := NewMessage(s.ListenAddr, broadcastTo.Payload)

	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, addr := range broadcastTo.To {
		peer, ok := s.peers[addr]
		if ok {
			go func(peer *Peer) {
				if err := peer.Send(buf.Bytes()); err != nil {
					logrus.Errorf("broadcast to peer error: %s", err)
				}
			}(peer)
		}
	}

	return nil
}

func init() {
	gob.Register(MessagePeerList{})
	gob.Register(MessageEncDeck{})
	gob.Register(MessageReady{})
}
