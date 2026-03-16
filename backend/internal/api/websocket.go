package api

import (
	"net/http"

	"instantdeploy/backend/internal/websocket"
	"instantdeploy/backend/pkg/utils"
)

func (h *Handler) WebSocket(w http.ResponseWriter, r *http.Request) {
	if h.wsHub == nil {
		utils.WriteError(w, http.StatusServiceUnavailable, "websocket hub unavailable")
		return
	}
	websocket.ServeWS(h.wsHub, w, r)
}
