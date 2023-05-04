package p2p

type GameStatus int32

func (gs GameStatus) String() string {
	switch gs {
	case GameStatusConnected:
		return "Connected"
	case GameStatusPlayerReady:
		return "Player Ready"
	case GameStatusShuffleAndDeal:
		return "Shuffle and Deal"
	case GameStatusReceivingCards:
		return "Receiving Cards"
	case GameStatusDealing:
		return "Dealing"
	case GameStatusPreflop:
		return "Preflop"
	case GameStatusFlop:
		return "Flop"
	case GameStatusTurn:
		return "Turn"
	case GameStatusRiver:
		return "River"
	default:
		return "Unknown"
	}
}

const (
	GameStatusConnected   GameStatus = iota
	GameStatusPlayerReady GameStatus = iota
	GameStatusShuffleAndDeal
	GameStatusReceivingCards
	GameStatusDealing
	GameStatusPreflop
	GameStatusFlop
	GameStatusTurn
	GameStatusRiver
)
