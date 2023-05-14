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

type AtomicInt struct {
	value int32
}

func NewAtomicInt(val int32) *AtomicInt {
	return &AtomicInt{
		value: val,
	}
}

func (a *AtomicInt) Set(value int32) {
	atomic.StoreInt32(&a.value, value)
}

func (a *AtomicInt) Get() int32 {
	return atomic.LoadInt32(&a.value)
}

func (a *AtomicInt) Inc() {
	a.Set(a.Get() + 1)
}

type PlayerActionRecv struct {
	mu          sync.RWMutex
	recvActions map[string]MessagePlayerAction
}

func NewPlayerActionRecv() *PlayerActionRecv {
	return &PlayerActionRecv{
		recvActions: map[string]MessagePlayerAction{},
	}
}

func (pa *PlayerActionRecv) addAction(from string, action MessagePlayerAction) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.recvActions[from] = action
}

func (pa *PlayerActionRecv) clear() {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.recvActions = map[string]MessagePlayerAction{}
}

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

type PlayersList struct {
	lock sync.RWMutex
	list []string
}

func NewPlayersList() *PlayersList {
	return &PlayersList{
		list: []string{},
	}
}

func (p *PlayersList) add(addr string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.list = append(p.list, addr)
	sort.Sort(p)
}

func (p *PlayersList) get(index int) string {
	p.lock.RLock()
	defer p.lock.RUnlock()

	if len(p.list)-1 < index {
		panic("the index is out of range")
	}

	return p.list[index]
}

func (p *PlayersList) len() int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return len(p.list)
}

func (p *PlayersList) List() []string {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.list
}

func (p *PlayersList) String() string {
	var players string
	for _, addr := range p.List() {
		players += " ," + addr
	}
	return players
}

func (p *PlayersList) Len() int { return len(p.list) }
func (p *PlayersList) Swap(i, j int) {
	p.list[i], p.list[j] = p.list[j], p.list[i]
}
func (p *PlayersList) Less(i, j int) bool {
	port1, _ := strconv.Atoi(p.list[i][1:])
	port2, _ := strconv.Atoi(p.list[j][1:])

	return port1 < port2
}

type Game struct {
	listenAddr  string
	broadcastCh chan BroadcastTo

	currentStatus *AtomicInt

	currentPlayerAction *AtomicInt

	currentDealer *AtomicInt

	currentPlayerTurn *AtomicInt

	playersReady      *PlayersReady
	playersList       *PlayersList
	recvPlayerActions *PlayerActionRecv
}

func NewGame(addr string, broadcastCh chan BroadcastTo) *Game {
	g := &Game{
		listenAddr:          addr,
		broadcastCh:         broadcastCh,
		currentStatus:       NewAtomicInt(int32(GameStatusConnected)),
		playersReady:        NewPlayersReady(),
		playersList:         NewPlayersList(),
		currentDealer:       NewAtomicInt(0),
		recvPlayerActions:   NewPlayerActionRecv(),
		currentPlayerTurn:   NewAtomicInt(0),
		currentPlayerAction: NewAtomicInt(0),
	}

	g.playersList.add(addr)

	go g.loop()

	return g
}

func (g *Game) loop() {
	ticker := time.NewTicker(time.Second * 5)

	for {
		<-ticker.C
		currentDealer, _ := g.getCurrentDealerAddr()
		logrus.WithFields(logrus.Fields{
			"addr":                g.listenAddr,
			"playersConnected":    g.playersList,
			"gameStatus":          GameStatus(g.currentStatus.Get()),
			"currentDealer":       currentDealer,
			"currentPlayerTurn":   g.currentPlayerTurn.value,
			"recvActions":         g.recvPlayerActions.recvActions,
			"currentPlayerAction": PlayerAction(g.currentPlayerAction.Get()),
		}).Info()
	}
}

func (g *Game) getNextGameStatus() GameStatus {
	switch GameStatus(g.currentStatus.Get()) {
	case GameStatusPreflop:
		return GameStatusFlop
	case GameStatusFlop:
		return GameStatusTurn
	case GameStatusTurn:
		return GameStatusRiver
	default:
		panic("invalid game status")
	}
}

func (g *Game) canTakeAction(from string) bool {
	currentPlayerAddr := g.playersList.get(int(g.currentPlayerTurn.Get()))

	return currentPlayerAddr == from
}

func (g *Game) isFromCurrentDealer(from string) bool {
	return g.playersList.get(int(g.currentDealer.Get())) == from
}

func (g *Game) handlePlayerAction(from string, action MessagePlayerAction) error {
	if !g.canTakeAction(from) {
		return fmt.Errorf("player (%s) taking action before his turn", from)
	}

	if action.CurrentGameStatus != GameStatus(g.currentStatus.Get()) && !g.isFromCurrentDealer(from) {
		return fmt.Errorf("player (%s) does not have correct game status (%s)", from, action.CurrentGameStatus)
	}

	g.recvPlayerActions.addAction(from, action)

	if g.playersList.get(int(g.currentDealer.Get())) == from {
		g.advanceToNextRound()
	}

	g.incNextPlayer()

	return nil
}

func (g *Game) TakeAction(action PlayerAction, value int) error {
	if !g.canTakeAction(g.listenAddr) {
		return fmt.Errorf("taking action before turn %s", g.listenAddr)
	}

	g.currentPlayerAction.Set((int32)(action))

	g.incNextPlayer()

	if g.listenAddr == g.playersList.get(int(g.currentDealer.Get())) {
		g.advanceToNextRound()
	}

	g.SendToPlayer(MessagePlayerAction{
		Action:            action,
		CurrentGameStatus: GameStatus(g.currentStatus.Get()),
		Value:             value,
	}, g.getOtherPlayers()...)

	return nil
}

// func (g *Game) bet(value int) error {
// 	return nil
// }

// func (g *Game) check() error {
// 	g.SetStatus(GameStatusChecked)

// 	g.SendToPlayer(MessagePlayerAction{
// 		Action:            PlayerActionCheck,
// 		CurrentGameStatus: g.currentStatus,
// 	}, g.getOtherPlayers()...)

// 	return nil
// }

// func (g *Game) fold() error {
// 	g.SetStatus(GameStatusFolded)

// 	g.SendToPlayer(MessagePlayerAction{
// 		Action:            PlayerActionFold,
// 		CurrentGameStatus: g.currentStatus,
// 	}, g.getOtherPlayers()...)

// 	return nil
// }

func (g *Game) advanceToNextRound() {
	g.recvPlayerActions.clear()
	g.currentPlayerAction.Set(int32(PlayerActionIdle))
	g.currentStatus.Set(int32(g.getNextGameStatus()))
}

func (g *Game) incNextPlayer() {
	if g.playersList.len()-1 == int(g.currentPlayerTurn.Get()) {
		g.currentPlayerTurn.Set(0)
		return
	}

	g.currentPlayerTurn.Inc()
}

func (g *Game) SetStatus(s GameStatus) {
	g.setStatus(s)
}

func (g *Game) setStatus(s GameStatus) {
	if s == GameStatusPreflop {
		g.incNextPlayer()
	}

	if GameStatus(g.currentStatus.Get()) != s {
		g.currentStatus.Set(int32(s))
	}
}

func (g *Game) getCurrentDealerAddr() (string, bool) {
	currentDealer := g.playersList.get(int(g.currentDealer.Get()))

	return currentDealer, g.listenAddr == currentDealer
}

func (g *Game) InitiateShuffleAndDeal() {
	dealToPlayer := g.playersList.get(g.getNextPositionOnTable())

	g.SendToPlayer(MessageEncDeck{Deck: [][]byte{}}, dealToPlayer)
}

func (g *Game) ShuffleAndEncrypt(from string, deck [][]byte) error {
	prevPlayer := g.playersList.get(g.getPrevPositionOnTable())

	if from != prevPlayer {
		return fmt.Errorf("received encrypted deck from the wrong player (%s) should be (%s)", from, prevPlayer)
	}

	_, isDealer := g.getCurrentDealerAddr()

	if isDealer && from == prevPlayer {
		g.setStatus(GameStatusPreflop)
		g.SendToPlayer(MessagePreFlop{}, g.getOtherPlayers()...)
		return nil
	}

	dealToPlayer := g.playersList.get(g.getNextPositionOnTable())

	g.SendToPlayer(MessageEncDeck{Deck: [][]byte{}}, dealToPlayer)

	g.setStatus(GameStatusDealing)

	return nil
}

func (g *Game) SetReady() {
	g.playersReady.addRecvStatus(g.listenAddr)
	g.SendToPlayer(MessageReady{}, g.getOtherPlayers()...)
	g.setStatus(GameStatusPlayerReady)
}

func (g *Game) SetPlayerReady(from string) {
	logrus.WithFields(logrus.Fields{
		"addr": g.listenAddr,
		"peer": from,
	}).Info("setting a player ready")

	g.playersReady.addRecvStatus(from)

	if g.playersReady.len() < g.playersList.len() {
		return
	}

	// g.playersReady.clear()

	if _, ok := g.getCurrentDealerAddr(); ok {
		fmt.Println("we are dealer")
		g.InitiateShuffleAndDeal()
	}
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
	g.playersList.add(from)
	sort.Sort(g.playersList)
	// g.playersReady.addRecvStatus(from)
}

func (g *Game) getOtherPlayers() []string {
	players := []string{}

	for _, addr := range g.playersList.List() {
		if addr == g.listenAddr {
			continue
		}

		players = append(players, addr)
	}

	return players
}

func (g *Game) getPositionOnTable() int {
	for i, player := range g.playersList.List() {
		if g.listenAddr == player {
			return i
		}
	}

	panic("player does not exist in players list")
}

func (g *Game) getPrevPositionOnTable() int {
	ownPosition := g.getPositionOnTable()
	if ownPosition == 0 {
		return g.playersList.len() - 1
	}
	return ownPosition - 1
}

func (g *Game) getNextPositionOnTable() int {
	ownPosition := g.getPositionOnTable()
	if ownPosition == g.playersList.len()-1 {
		return 0
	}
	return ownPosition + 1
}

// func (g *Game) getNextPlayerReady() string {
// 	nextPos := g.getNextPositionOnTable()
// 	nextPlayerAddr := g.playersList[nextPos]

// }

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
