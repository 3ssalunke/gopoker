package p2p

type Message struct {
	Payload any
	From    string
}

type BroadcastTo struct {
	Payload any
	To      []string
}

func NewMessage(from string, payload any) *Message {
	return &Message{
		From:    from,
		Payload: payload,
	}
}

type Handshake struct {
	Version     string
	GameVariant GameVariant
	GameStatus  GameStatus
	ListenAddr  string
}

type MessagePeerList struct {
	Peers []string
}

type MessageEncDeck struct {
	Deck [][]byte
}

type MessageReady struct{}

func (msg MessageReady) String() string {
	return "Ready"
}
