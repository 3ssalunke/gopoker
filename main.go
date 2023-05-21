package main

import (
	"log"
	"time"

	"github.com/3ssalunke/gopoker/p2p"
)

func makeServerAndStart(addr string, apiAddr string) *p2p.Node {
	cfg := p2p.ServerConfig{
		Version:       "GPOKER V0.1-alpha",
		ListenAddr:    addr,
		ApiListenAddr: apiAddr,
		GameVariant:   p2p.TexasHoldem,
	}
	server := p2p.NewNode(cfg)
	go server.Start()

	time.Sleep(time.Second * 1)

	return server
}

func main() {
	node1 := makeServerAndStart(":3000", ":3001")
	node2 := makeServerAndStart(":4000", ":4001")
	node3 := makeServerAndStart(":5000", ":5001")
	node4 := makeServerAndStart(":6000", ":6001")

	err := node2.Connect(node1.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	err = node3.Connect(node2.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	err = node4.Connect(node1.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}

	select {}
}
