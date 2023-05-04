package p2p

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type PlayersReady struct {
	mu           sync.RWMutex
	recvStatuses map[string]bool
}

func NewPlayersReady() *PlayersReady {
	return &PlayersReady{
		recvStatuses: make(map[string]bool),
	}
}

func (pr *PlayersReady) addRecvStatus(from string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.recvStatuses[from] = true
}

func (pr *PlayersReady) len() int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	return len(pr.recvStatuses)
}

func (pr *PlayersReady) clear() {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.recvStatuses = make(map[string]bool)
}

type PlayersList []string

func (list PlayersList) Len() int { return len(list) }
func (list PlayersList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}
func (list PlayersList) Less(i, j int) bool {
	port1, _ := strconv.Atoi(list[i][1:])
	port2, _ := strconv.Atoi(list[j][1:])

	return port1 < port2
}

type Game struct {
	listenAddr  string
	broadcastCh chan BroadcastTo

	currentStatus GameStatus

	playersReady *PlayersReady
	playersList  PlayersList
}

func NewGame(addr string, broadcastCh chan BroadcastTo) *Game {
	g := &Game{
		listenAddr:    addr,
		broadcastCh:   broadcastCh,
		currentStatus: GameStatusConnected,
		playersReady:  NewPlayersReady(),
		playersList:   PlayersList{},
	}

	go g.loop()

	return g
}

func (g *Game) loop() {
	ticker := time.NewTicker(time.Second * 5)

	for {
		<-ticker.C
		logrus.WithFields(logrus.Fields{
			"addr":             g.listenAddr,
			"player connected": g.playersList,
			"status":           g.currentStatus,
		}).Info()
	}
}

func (g *Game) SetStatus(s GameStatus) {
	if g.currentStatus != s {
		atomic.StoreInt32((*int32)(&g.currentStatus), (int32(s)))
	}
}

func (g *Game) SetReady() {
	g.playersReady.addRecvStatus(g.listenAddr)
	g.SendToPlayer(MessageReady{}, g.getOtherPlayers()...)
	g.SetStatus(GameStatusPlayerReady)
}

func (g *Game) SendToPlayer(payload any, addr ...string) {
	g.broadcastCh <- BroadcastTo{
		To:      addr,
		Payload: payload,
	}

	logrus.WithFields(logrus.Fields{
		"addr":    g.listenAddr,
		"peer":    addr,
		"payload": payload,
	}).Info("sending to player")
}

func (g *Game) AddPlayer(from string) {
	g.playersList = append(g.playersList, from)
	sort.Sort(g.playersList)
	g.playersReady.addRecvStatus(from)
}

func (g *Game) getOtherPlayers() []string {
	players := []string{}

	for _, addr := range g.playersList {
		if addr == g.listenAddr {
			continue
		}

		players = append(players, addr)
	}

	return players
}

type Player struct {
	ListenAddr string
	Status     GameStatus
}

func (p *Player) String() string {
	return fmt.Sprintf("%s:%s", p.ListenAddr, p.Status)
}

// func (list PlayersList) Len() int { return len(list) }
// func (list PlayersList) Swap(i, j int) {
// 	list[i], list[j] = list[j], list[i]
// }
// func (list PlayersList) Less(i, j int) bool {
// 	port1, _ := strconv.Atoi(list[i].ListenAddr[1:])
// 	port2, _ := strconv.Atoi(list[j].ListenAddr[1:])

// 	return port1 < port2
// }
// func (p *Player) String() string {
// 	return fmt.Sprintf("%s:%s", p.ListenAddr, p.Status)
// }

// type GameState struct {
// 	listenAddr  string
// 	broadcastCh chan BroadcastTo
// 	isDealer    bool
// 	gameStatus  GameStatus

// 	playersLock sync.RWMutex
// 	players     map[string]*Player
// 	playerList  PlayersList
// }

// func NewGameState(addr string, broadcastCh chan BroadcastTo) *GameState {
// 	g := &GameState{
// 		listenAddr:  addr,
// 		broadcastCh: broadcastCh,
// 		isDealer:    false,
// 		gameStatus:  GameStatusWaitingForCards,
// 		players:     make(map[string]*Player),
// 	}

// 	g.AddPlayer(addr, GameStatusWaitingForCards)

// 	go g.loop()

// 	return g
// }

// func (g *GameState) SetStatus(s GameStatus) {
// 	if g.gameStatus != s {
// 		atomic.StoreInt32((*int32)(&g.gameStatus), (int32(s)))
// 		g.players[g.listenAddr].Status = s
// 	}
// }

// func (g *GameState) loop() {
// 	ticker := time.NewTicker(time.Second * 5)

// 	for {
// 		<-ticker.C
// 		logrus.WithFields(logrus.Fields{
// 			"addr":             g.listenAddr,
// 			"player connected": g.playerList,
// 			"status":           g.gameStatus,
// 		}).Info()
// 	}
// }

// func (g *GameState) AddPlayer(addr string, status GameStatus) {
// 	g.playersLock.Lock()
// 	defer g.playersLock.Unlock()

// 	player := &Player{
// 		ListenAddr: addr,
// 	}

// 	g.players[addr] = player

// 	g.playerList = append(g.playerList, addr)
// 	sort.Sort(g.playerList)

// 	g.SetPlayerStatus(addr, status)

// 	logrus.WithFields(logrus.Fields{
// 		"addr":   g.listenAddr,
// 		"peer":   addr,
// 		"status": status,
// 	}).Info("new player joined")
// }

// func (g *GameState) playersWaitingForCards() int {
// 	totalPlayers := 0

// 	for i := 0; i < len(g.playerList); i++ {
// 		if g.playerList[i].Status == GameStatusWaitingForCards {
// 			totalPlayers++
// 		}
// 	}

// 	return totalPlayers
// }

// func (g *GameState) SetPlayerStatus(addr string, status GameStatus) {
// 	player, ok := g.players[addr]

// 	if !ok {
// 		panic("player could not be found, altough it should exist")
// 	}

// 	player.Status = status

// 	g.CheckNeedForDealCards()
// }

// func (g *GameState) CheckNeedForDealCards() {
// 	// playersWaiting := atomic.LoadInt32(&g.playersWaitingForCards)
// 	playersWaiting := g.playersWaitingForCards()

// 	if playersWaiting == len(g.players) && g.isDealer && g.gameStatus == GameStatusWaitingForCards {
// 		logrus.WithFields(logrus.Fields{
// 			"addr": g.listenAddr,
// 		}).Info("need to deal cards")

// 		g.InitiateShuffleAndDeal()
// 	}
// }

// func (g *GameState) ShuffleAndEncrypt(from string, deck [][]byte) error {
// 	g.SetPlayerStatus(from, GameStatusShuffleAndDeal)

// 	prevPlayer := g.playerList[g.getPrevPositionOnTable()]
// 	if g.isDealer && from == prevPlayer.ListenAddr {
// 		logrus.Info("round trip completed")
// 		return nil
// 	}
// 	dealToPlayer := g.playerList[g.getNextPositionOnTable()]

// 	g.SendToPlayer(dealToPlayer.ListenAddr, MessageEncDeck{Deck: [][]byte{}})

// 	g.SetStatus(GameStatusShuffleAndDeal)

// 	return nil
// }

// func (g *GameState) getPositionOnTable() int {
// 	for i, player := range g.playerList {
// 		if g.listenAddr == player.ListenAddr {
// 			return i
// 		}
// 	}

// 	panic("player does not exist in players list")
// }

// func (g *GameState) getPrevPositionOnTable() int {
// 	ownPosition := g.getPositionOnTable()
// 	if ownPosition == 0 {
// 		return len(g.playerList) - 1
// 	}
// 	return ownPosition - 1
// }

// func (g *GameState) getNextPositionOnTable() int {
// 	ownPosition := g.getPositionOnTable()
// 	if ownPosition == len(g.playerList)-1 {
// 		return 0
// 	}
// 	return ownPosition + 1
// }

// func (g *GameState) InitiateShuffleAndDeal() {
// 	dealToPlayer := g.playerList[g.getNextPositionOnTable()]

// 	g.SendToPlayer(dealToPlayer.ListenAddr, MessageEncDeck{Deck: [][]byte{}})

// 	g.SetStatus(GameStatusShuffleAndDeal)
// }

// // func (g *GameState) ShuffleAndDeal() error {
// // 	dealToPlayer := g.playerList[g.getNextPositionOnTable()]

// // 	g.SendToPlayer(dealToPlayer.ListenAddr, MessageEncDeck{Deck: [][]byte{}})

// // 	g.SetStatus(GameStatusShuffleAndDeal)

// // 	return nil
// // }

// func (g *GameState) SendToPlayer(addr string, payload any) {
// 	g.broadcastCh <- BroadcastTo{
// 		To:      []string{addr},
// 		Payload: payload,
// 	}

// 	logrus.WithFields(logrus.Fields{
// 		"addr":    g.listenAddr,
// 		"peer":    addr,
// 		"payload": payload,
// 	}).Info("sending to player")
// }

// func (g *GameState) SendToPlayerWithStatus(payload any, s GameStatus) {
// 	players := g.GetPlayersWithStatus(s)

// 	g.broadcastCh <- BroadcastTo{
// 		To:      players,
// 		Payload: payload,
// 	}

// 	logrus.WithFields(logrus.Fields{
// 		"addr":    g.listenAddr,
// 		"players": players,
// 		"payload": payload,
// 	}).Info("sending to players")
// }

// func (g *GameState) GetPlayersWithStatus(s GameStatus) []string {
// 	players := []string{}

// 	for addr, player := range g.players {
// 		if player.Status == s {
// 			players = append(players, addr)
// 		}
// 	}

// 	return players
// }
