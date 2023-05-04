package p2p

import (
	"encoding/json"
	"net/http"

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

	logrus.WithFields(logrus.Fields{"listen addr": s.listenAddr}).Info("api server started")

	http.ListenAndServe(s.listenAddr, r)
}

func (s *APIServer) handlePlayerReady(w http.ResponseWriter, r *http.Request) error {
	s.game.SetReady()
	return JSON(w, http.StatusOK, []byte("Ready"))
}
