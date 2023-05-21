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

	err := node2.Connect(node1.ListenAddr)

	if err != nil {
		log.Fatal(err)
	}

	// playerA := makeServerAndStart(":3000", ":3001")
	// playerB := makeServerAndStart(":4000", ":4001")
	// playerC := makeServerAndStart(":5000", ":5001")
	// playerD := makeServerAndStart(":6000")
	// playerE := makeServerAndStart(":7000")

	// if err := playerB.Connect(playerA.ListenAddr); err != nil {
	// 	log.Fatal(err)
	// }
	// time.Sleep(time.Second * 1)
	// if err := playerC.Connect(playerB.ListenAddr); err != nil {
	// 	log.Fatal(err)
	// }
	// time.Sleep(time.Second * 1)
	// if err := playerD.Connect(playerA.ListenAddr); err != nil {
	// 	log.Fatal(err)
	// }
	// time.Sleep(time.Second * 2)
	// if err := playerE.Connect(playerA.ListenAddr); err != nil {
	// 	log.Fatal(err)
	// }

	select {}
}
