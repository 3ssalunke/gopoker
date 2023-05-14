package p2p

type PlayerAction byte

func (pa PlayerAction) String() string {
	switch pa {
	case PlayerActionIdle:
		return "Idle"
	case PlayerActionFold:
		return "Fold"
	case PlayerActionBet:
		return "Bet"
	case PlayerActionCheck:
		return "Check"
	default:
		return "Unknown"
	}
}

const (
	PlayerActionIdle = iota
	PlayerActionFold
	PlayerActionCheck
	PlayerActionBet
)

type GameStatus int32

func (gs GameStatus) String() string {
	switch gs {
	case GameStatusConnected:
		return "Connected"
	case GameStatusPlayerReady:
		return "Player Ready"
	case GameStatusDealing:
		return "Dealing"
	case GameStatusFolded:
		return "Folded"
	case GameStatusChecked:
		return "Checked"
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
	GameStatusConnected GameStatus = iota
	GameStatusPlayerReady
	GameStatusDealing
	GameStatusFolded
	GameStatusChecked
	GameStatusPreflop
	GameStatusFlop
	GameStatusTurn
	GameStatusRiver
)
