package p2p

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/3ssalunke/gopoker/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Node struct {
	ServerConfig

	gameState *GameState

	peerLock sync.RWMutex
	peers    map[string]proto.GossipClient

	broadcastch chan BroadcastTo

	proto.UnimplementedGossipServer
}

func NewNode(cfg ServerConfig) *Node {
	broadcastCh := make(chan BroadcastTo, 1024)

	node := &Node{
		ServerConfig: cfg,
		peers:        make(map[string]proto.GossipClient),
		broadcastch:  broadcastCh,
		gameState:    NewGameState(cfg.ListenAddr, broadcastCh),
	}

	go func(n *Node) {
		apiServer := NewAPIServer(cfg.ApiListenAddr, n.gameState)
		apiServer.Run()
	}(node)

	return node
}

func (n *Node) Handshake(ctx context.Context, version *proto.Version) (*proto.Version, error) {
	client, err := makeGrpcClientConn(version.ListenAddr)
	if err != nil {
		return nil, err
	}

	n.addPeer(client, version)

	return n.getVersion(), nil
}

func (n *Node) addPeer(c proto.GossipClient, v *proto.Version) {
	n.peerLock.Lock()
	defer n.peerLock.Unlock()

	n.peers[v.ListenAddr] = c
	n.gameState.AddPlayer(v.ListenAddr)

	go func() {
		for _, addr := range v.PeerList {
			if err := n.Connect(addr); err != nil {
				fmt.Println("failed to connect: ", err)
				continue
			}
		}
	}()

	logrus.WithFields(logrus.Fields{
		"we":     n.ListenAddr,
		"remote": v.ListenAddr,
	}).Info("new player connected")
}

func (n *Node) HandleTakeSeat(ctx context.Context, v *proto.TakeSeat) (*proto.Ack, error) {
	n.gameState.SetPlayerAtTable(v.Addr)
	return &proto.Ack{}, nil
}

func (n *Node) broadcastTo(bct BroadcastTo) {
	for _, addr := range bct.To {
		go func(addr string) {
			client, ok := n.peers[addr]
			if !ok {
				return
			}

			switch v := bct.Payload.(type) {
			case *proto.TakeSeat:
				_, err := client.HandleTakeSeat(context.TODO(), v)
				if err != nil {
					fmt.Printf("encDeck broadcase error: %s\n", err)
				}

			case *proto.EncDeck:
				_, err := client.HandleEncDeck(context.TODO(), v)
				if err != nil {
					fmt.Printf("encDeck broadcase error: %s\n", err)
				}
			}
		}(addr)
	}
}

func (n *Node) loop() {
	for bt := range n.broadcastch {
		n.broadcastTo(bt)
	}
}

func (n *Node) getVersion() *proto.Version {
	return &proto.Version{
		Version:    n.Version,
		ListenAddr: n.ListenAddr,
		PeerList:   n.getPeerList(),
	}
}

func (n *Node) getPeerList() []string {
	n.peerLock.RLock()
	defer n.peerLock.RUnlock()

	var (
		peers = make([]string, len(n.peers))
		i     = 0
	)

	for addr := range n.peers {
		peers[i] = addr
		i++
	}

	return peers
}

func (n *Node) canConnect(addr string) bool {
	if n.ListenAddr == addr {
		return false
	}
	for peerAddr := range n.peers {
		if peerAddr == addr {
			return false
		}
	}
	return true
}

func (n *Node) Connect(addr string) error {
	if !n.canConnect(addr) {
		return nil
	}
	client, err := makeGrpcClientConn(addr)
	if err != nil {
		return err
	}

	hs, err := client.Handshake(context.TODO(), n.getVersion())
	if err != nil {
		return err
	}

	n.addPeer(client, hs)

	return nil
}

func (n *Node) Start() error {
	grpcServer := grpc.NewServer()
	proto.RegisterGossipServer(grpcServer, n)

	ln, err := net.Listen("tcp", n.ListenAddr)
	if err != nil {
		return err
	}

	go n.loop()

	logrus.WithFields(logrus.Fields{
		"port":       n.ListenAddr,
		"variant":    n.GameVariant,
		"maxPlayers": n.MaxPlayers,
	}).Info("p2p server started running")

	return grpcServer.Serve(ln)
}

func makeGrpcClientConn(addr string) (proto.GossipClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := proto.NewGossipClient(conn)
	return client, err
}
