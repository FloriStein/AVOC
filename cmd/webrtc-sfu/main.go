package main

import (
	"encoding/json"
	"net/http"
	"os"

	"avoc/internal/webrtcsfu"
	"avoc/pkg/logger"
)

var log = logger.New("webrtc-sfu")

func main() {
	port := os.Getenv("SFU_PORT")
	if port == "" {
		port = "8084"
	}

	sfu := webrtcsfu.New()
	mux := newSFUMux(sfu)

	log.Info("WebRTC SFU starting", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal("WebRTC SFU failed", "error", err)
	}
}

func newSFUMux(sfu *webrtcsfu.SFU) *http.ServeMux {
	mux := http.NewServeMux()
	// Session Event Consumer — receives SESSION_* events from Control Server (ADR-015).
	mux.HandleFunc("POST /session/event", handleSessionEvent(sfu))
	mux.HandleFunc("POST /offer/{sessionId}/{peerId}", handleVehicleOffer(sfu))
	mux.HandleFunc("POST /subscribe/{sessionId}/{operatorId}", handleOperatorSubscribe(sfu))
	mux.HandleFunc("GET /health", handleHealth)
	return mux
}

func handleSessionEvent(sfu *webrtcsfu.SFU) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var event webrtcsfu.SessionEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		sfu.HandleSessionEvent(event)
		w.WriteHeader(http.StatusAccepted)
	}
}

func handleVehicleOffer(sfu *webrtcsfu.SFU) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("sessionId")
		peerID := r.PathValue("peerId")

		var req struct {
			SDP string `json:"sdp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		answer, err := sfu.CreateVehicleOffer(sessionID, peerID, req.SDP)
		if err != nil {
			log.Error("SFU offer error", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"sdp": answer})
	}
}

func handleOperatorSubscribe(sfu *webrtcsfu.SFU) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.PathValue("sessionId")
		operatorID := r.PathValue("operatorId")

		var req struct {
			SDP string `json:"sdp"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		answer, err := sfu.SubscribeOperator(sessionID, operatorID, req.SDP)
		if err != nil {
			log.Error("SFU subscribe error", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"sdp": answer})
	}
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "webrtc-sfu"})
}
