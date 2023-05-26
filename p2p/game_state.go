package p2p

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/3ssalunke/gopoker/proto"
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

// type PlayerActionRecv struct {
// 	mu          sync.RWMutex
// 	recvActions map[string]MessagePlayerAction
// }

// func NewPlayerActionRecv() *PlayerActionRecv {
// 	return &PlayerActionRecv{
// 		recvActions: map[string]MessagePlayerAction{},
// 	}
// }

// func (pa *PlayerActionRecv) addAction(from string, action MessagePlayerAction) {
// 	pa.mu.Lock()
// 	defer pa.mu.Unlock()

// 	pa.recvActions[from] = action
// }

// func (pa *PlayerActionRecv) clear() {
// 	pa.mu.Lock()
// 	defer pa.mu.Unlock()

// 	pa.recvActions = map[string]MessagePlayerAction{}
// }

// type PlayersReady struct {
// 	mu           sync.RWMutex
// 	recvStatuses map[string]bool
// }

// func NewPlayersReady() *PlayersReady {
// 	return &PlayersReady{
// 		recvStatuses: make(map[string]bool),
// 	}
// }

// func (pr *PlayersReady) addRecvStatus(from string) {
// 	pr.mu.Lock()
// 	defer pr.mu.Unlock()

// 	pr.recvStatuses[from] = true
// }

// func (pr *PlayersReady) len() int {
// 	pr.mu.RLock()
// 	defer pr.mu.RUnlock()

// 	return len(pr.recvStatuses)
// }

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

func (p *PlayersList) getIndex(addr string) int {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for i := 0; i < p.len(); i++ {
		if p.list[i] == addr {
			return i
		}
	}

	return -1
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

type GameState struct {
	listenAddr  string
	broadcastCh chan BroadcastTo

	currentStatus *AtomicInt

	currentPlayerAction *AtomicInt

	currentDealer *AtomicInt

	currentPlayerTurn *AtomicInt

	// playersReady      *PlayersReady
	playersList      *PlayersList
	playersReadyList *PlayersList
	// recvPlayerActions *PlayerActionRecv

	table *Table
}

func NewGameState(addr string, broadcastCh chan BroadcastTo) *GameState {
	g := &GameState{
		listenAddr:    addr,
		broadcastCh:   broadcastCh,
		currentStatus: NewAtomicInt(int32(GameStatusConnected)),
		// playersReady:        NewPlayersReady(),
		playersList:      NewPlayersList(),
		playersReadyList: NewPlayersList(),
		currentDealer:    NewAtomicInt(0),
		// recvPlayerActions:   NewPlayerActionRecv(),
		currentPlayerTurn:   NewAtomicInt(0),
		currentPlayerAction: NewAtomicInt(0),
		table:               NewTable(defaultMaxPlayers),
	}

	g.playersList.add(addr)

	go g.loop()

	return g
}

func (g *GameState) loop() {
	ticker := time.NewTicker(time.Second * 5)

	for {
		<-ticker.C
		currentDealer, _ := g.getCurrentDealerAddr()
		logrus.WithFields(logrus.Fields{
			"addr":              g.listenAddr,
			"currentGameStatus": GameStatus(g.currentStatus.Get()),
			"currentDealer":     currentDealer,
			"currentPlayerTurn": g.currentPlayerTurn.Get(),
			"table":             g.table,
			"players":           g.playersList,
		}).Info()
	}
}

func (g *GameState) getNextGameStatus() GameStatus {
	switch GameStatus(g.currentStatus.Get()) {
	case GameStatusPreflop:
		return GameStatusFlop
	case GameStatusFlop:
		return GameStatusTurn
	case GameStatusTurn:
		return GameStatusRiver
	case GameStatusRiver:
		return GameStatusPlayerReady
	default:
		panic("invalid game status")
	}
}

func (g *GameState) canTakeAction(from string) bool {
	currentPlayerAddr := g.playersList.get(int(g.currentPlayerTurn.Get()))

	return currentPlayerAddr == from
}

func (g *GameState) isFromCurrentDealer(from string) bool {
	return g.playersList.get(int(g.currentDealer.Get())) == from
}

func (g *GameState) handlePlayerAction(from string, action MessagePlayerAction) error {
	if !g.canTakeAction(from) {
		return fmt.Errorf("player (%s) taking action before his turn", from)
	}

	if action.CurrentGameStatus != GameStatus(g.currentStatus.Get()) && !g.isFromCurrentDealer(from) {
		return fmt.Errorf("player (%s) does not have correct game status (%s)", from, action.CurrentGameStatus)
	}

	// g.recvPlayerActions.addAction(from, action)

	if g.playersList.get(int(g.currentDealer.Get())) == from {
		g.advanceToNextRound()
	}

	g.incNextPlayer()

	return nil
}

func (g *GameState) TakeAction(action PlayerAction, value int) error {
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

func (g *GameState) advanceToNextRound() {
	g.currentPlayerAction.Set(int32(PlayerActionNone))
	if GameStatus(g.currentStatus.Get()) == GameStatusRiver {
		g.TakeSeatAtTable()
		return
	}
	g.currentStatus.Set(int32(g.getNextGameStatus()))
}

func (g *GameState) incNextPlayer() {
	if g.playersList.len()-1 == int(g.currentPlayerTurn.Get()) {
		g.currentPlayerTurn.Set(0)
		return
	}

	g.currentPlayerTurn.Inc()
}

func (g *GameState) SetStatus(s GameStatus) {
	g.setStatus(s)
}

func (g *GameState) setStatus(s GameStatus) {
	if s == GameStatusPreflop {
		g.incNextPlayer()
	}

	if GameStatus(g.currentStatus.Get()) != s {
		g.table.SetPlayerStatus(g.listenAddr, s)
		g.currentStatus.Set(int32(s))
	}
}

func (g *GameState) getCurrentDealerAddr() (string, bool) {
	currentDealer := g.playersList.get(int(g.currentDealer.Get()))

	return currentDealer, g.listenAddr == currentDealer
}

func (g *GameState) InitiateShuffleAndDeal() {
	// dealToPlayer := g.playersList.get(g.getNextPositionOnTable())
	dealToPlayer, err := g.table.GetPlayerNextTo(g.listenAddr)
	if err != nil {
		panic(err)
	}

	g.SendToPlayer(MessageEncDeck{Deck: [][]byte{}}, dealToPlayer.addr)
	g.setStatus(GameStatusDealing)
}

func (g *GameState) ShuffleAndEncrypt(from string, deck [][]byte) error {
	// prevPlayer := g.playersList.get(g.getPrevPositionOnTable())
	prevPlayer, err := g.table.GetPlayerPrevTo(g.listenAddr)
	if err != nil {
		panic(err)
	}

	if from != prevPlayer.addr {
		return fmt.Errorf("received encrypted deck from the wrong player (%s) should be (%s)", from, prevPlayer.addr)
	}

	_, isDealer := g.getCurrentDealerAddr()

	if isDealer && from == prevPlayer.addr {
		g.setStatus(GameStatusPreflop)
		g.SendToPlayer(MessagePreFlop{}, g.getOtherPlayers()...)
		return nil
	}

	dealToPlayer, err := g.table.GetPlayerNextTo(g.listenAddr)
	if err != nil {
		panic(err)
	}

	g.SendToPlayer(MessageEncDeck{Deck: [][]byte{}}, dealToPlayer.addr)
	g.setStatus(GameStatusDealing)

	return nil
}

func (g *GameState) TakeSeatAtTable() {
	pos := g.playersList.getIndex(g.listenAddr)
	g.table.AddPlayerInPosition(g.listenAddr, pos)

	// g.playersReady.addRecvStatus(g.listenAddr)
	g.playersReadyList.add(g.listenAddr)
	g.SendToPlayer(&proto.TakeSeat{
		Addr: g.listenAddr,
	}, g.getOtherPlayers()...)
	g.setStatus(GameStatusPlayerReady)
}

func (g *GameState) mayBeDeal() {
	if GameStatus(g.currentStatus.Get()) == GameStatusPlayerReady {
		g.InitiateShuffleAndDeal()
	}
}

func (g *GameState) SetPlayerAtTable(addr string) {
	pos := g.playersList.getIndex(addr)
	g.table.AddPlayerInPosition(addr, pos)

	// g.playersReady.addRecvStatus(addr)
	g.playersReadyList.add(addr)

	if g.table.LenPlayers() < 2 {
		return
	}

	if _, ok := g.getCurrentDealerAddr(); ok {
		go func() {
			time.Sleep(time.Second * 5)
			g.mayBeDeal()
		}()
	}
}

func (g *GameState) SendToPlayer(payload any, addr ...string) {
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

func (g *GameState) AddPlayer(from string) {
	g.playersList.add(from)
	sort.Sort(g.playersList)
}

func (g *GameState) getOtherPlayers() []string {
	players := []string{}

	for _, addr := range g.playersList.List() {
		if addr == g.listenAddr {
			continue
		}

		players = append(players, addr)
	}

	return players
}
