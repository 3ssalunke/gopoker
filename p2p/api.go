package p2p

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type apiFunc func(w http.ResponseWriter, r *http.Request) error

func makeHTTPHandlerFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			JSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
	}
}

func JSON(w http.ResponseWriter, status int, v any) error {
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

type APIServer struct {
	listenAddr string
	game       *Game
}

func NewAPIServer(listenAddr string, game *Game) *APIServer {
	return &APIServer{
		listenAddr,
		game,
	}
}

func (s *APIServer) Run() {
	r := mux.NewRouter()

	r.HandleFunc("/ready", makeHTTPHandlerFunc(s.handlePlayerReady))
	r.HandleFunc("/fold", makeHTTPHandlerFunc(s.handlePlayerFold))
	r.HandleFunc("/check", makeHTTPHandlerFunc(s.handlePlayerCheck))
	r.HandleFunc("/bet/{value}", makeHTTPHandlerFunc(s.handlePlayerBet))

	logrus.WithFields(logrus.Fields{"listen addr": s.listenAddr}).Info("api server started")

	http.ListenAndServe(s.listenAddr, r)
}

func (s *APIServer) handlePlayerReady(w http.ResponseWriter, r *http.Request) error {
	s.game.SetReady()
	return JSON(w, http.StatusOK, "Ready")
}

func (s *APIServer) handlePlayerFold(w http.ResponseWriter, r *http.Request) error {
	if err := s.game.TakeAction(PlayerActionFold, 0); err != nil {
		return err
	}
	return JSON(w, http.StatusOK, "Folded")
}

func (s *APIServer) handlePlayerCheck(w http.ResponseWriter, r *http.Request) error {
	if err := s.game.TakeAction(PlayerActionCheck, 0); err != nil {
		return err
	}
	return JSON(w, http.StatusOK, "Checked")
}

func (s *APIServer) handlePlayerBet(w http.ResponseWriter, r *http.Request) error {
	valueStr := mux.Vars(r)["value"]
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return err
	}

	if err := s.game.TakeAction(PlayerActionBet, value); err != nil {
		return err
	}

	return JSON(w, http.StatusOK, fmt.Sprintf("value:%d", value))
}
