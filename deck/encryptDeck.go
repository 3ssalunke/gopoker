package deck

import (
	"bytes"
	"encoding/gob"
)

func EncryptCard(key []byte, card Card) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(card); err != nil {
		return nil, err
	}

	payload := buf.Bytes()

	encOutput := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		encOutput[i] = payload[i] ^ key[i%len(key)]
	}

	return Encrypt(key, payload)
}

func DecryptCard(key, encCard []byte) (card Card, err error) {
	card = Card{}

	encBytes, err := Encrypt(key, encCard)
	if err != nil {
		return card, err
	}

	if err := gob.NewDecoder(bytes.NewReader(encBytes)).Decode(&card); err != nil {
		return card, err
	}

	return card, nil
}

func Encrypt(key, payload []byte) ([]byte, error) {
	encOutput := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		encOutput[i] = payload[i] ^ key[i%len(key)]
	}

	return encOutput, nil
}
