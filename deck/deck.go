package deck

import (
	"fmt"
	"math/rand"
	"strconv"
)

type Suit int

func (s Suit) String() string {
	switch s {
	case Spades:
		return "Spades"
	case Hearts:
		return "Hearts"
	case Diamonds:
		return "Diamonds"
	case Clubs:
		return "Clubs"
	default:
		panic("invalid card suit")
	}
}

const (
	Spades Suit = iota
	Hearts
	Diamonds
	Clubs
)

type Card struct {
	Suit  Suit
	Value int
}

func (c Card) String() string {
	value := strconv.Itoa(c.Value)
	switch c.Value {
	case 1:
		value = "ACE"
	case 11:
		value = "JACK"
	case 12:
		value = "QUEEN"
	case 13:
		value = "Knight"
	default:
		break
	}
	return fmt.Sprintf("%s of %s %s", value, c.Suit, suitToUnicode(c.Suit))
}

func NewCard(s Suit, v int) Card {
	if v > 13 {
		panic("the value of the card can not be higher than 13")
	}

	return Card{
		Suit:  s,
		Value: v,
	}
}

type Deck [52]Card

func New() Deck {
	var (
		nSuits = 4
		nCards = 12
		d      = [52]Card{}
	)

	x := 0
	for i := 0; i < nSuits; i++ {
		for j := 0; j <= nCards; j++ {
			d[x] = NewCard(Suit(i), j+1)
			x++
		}
	}
	return shuffle(d)
}

func shuffle(d Deck) Deck {
	for i := 0; i < len(d); i++ {
		r := rand.Intn(i + 1)

		if i != r {
			d[i], d[r] = d[r], d[i]
		}
	}

	return d
}

func suitToUnicode(s Suit) string {
	switch s {
	case Spades:
		return "♠"
	case Hearts:
		return "♥"
	case Diamonds:
		return "♦"
	case Clubs:
		return "♣"
	default:
		panic("invalid card suit")
	}
}
