package ws

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	notificaciones "axis-flow-back/internal/notificaciones"
	"axis-flow-back/internal/service"
)

// Handler wraps the Hub and exposes ServeWS as an http.HandlerFunc.
type Handler struct {
	hub     *Hub
	authSvc *service.AuthService
	cfg     notificaciones.Config
}

// NewHandler allocates a Handler.
func NewHandler(hub *Hub, authSvc *service.AuthService, cfg notificaciones.Config) *Handler {
	return &Handler{hub: hub, authSvc: authSvc, cfg: cfg}
}

// ServeWS handles the HTTP → WebSocket upgrade for the Notificaciones module.
//
// Security contract:
//   - JWT is extracted from the ?token= query parameter only (browser WS API
//     limitation — the Upgrade header cannot carry arbitrary headers).
//   - The token value is NEVER logged.
//   - Missing or invalid tokens result in HTTP 401 before the upgrade.
func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := h.authSvc.ParseAccessToken(tokenStr)
	if err != nil {
		// Token is present but invalid/expired: upgrade first, then close with
		// code 4001 so browser clients can distinguish auth failures from
		// network errors. NEVER log the token value.
		slog.Info("ws.Handler: token validation failed", "remote_addr", r.RemoteAddr)
		upgrader := websocket.Upgrader{
			HandshakeTimeout: h.cfg.WSHandshakeTimeout,
			CheckOrigin:      func(r *http.Request) bool { return true },
		}
		conn, upgradeErr := upgrader.Upgrade(w, r, nil)
		if upgradeErr != nil {
			return
		}
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "unauthorized"))
		conn.Close()
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		slog.Error("ws.Handler: invalid user_id in token claims", "error", err)
		upgrader := websocket.Upgrader{
			HandshakeTimeout: h.cfg.WSHandshakeTimeout,
			CheckOrigin:      func(r *http.Request) bool { return true },
		}
		conn, upgradeErr := upgrader.Upgrade(w, r, nil)
		if upgradeErr != nil {
			return
		}
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(4001, "unauthorized"))
		conn.Close()
		return
	}

	upgrader := websocket.Upgrader{
		HandshakeTimeout: h.cfg.WSHandshakeTimeout,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrader already wrote an error response.
		slog.Error("ws.Handler: upgrade error", "error", err)
		return
	}

	client := NewClient(h.hub, conn, userID, h.cfg)
	h.hub.Subscribe(client)

	go client.WritePump(h.cfg)
	client.ReadPump(h.cfg) // blocks until disconnect

	// ReadPump's defer calls hub.Unsubscribe, so nothing to do here.
}
