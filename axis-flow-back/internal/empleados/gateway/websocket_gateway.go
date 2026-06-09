// Package gateway provides realtime delivery for empleados events.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	empleados "axis-flow-back/internal/empleados"
	"axis-flow-back/internal/empleados/service"

	"github.com/gorilla/websocket"
)

const (
	verificationSendBuffer = 16
	companyDashboardFormat = "company:%d:dashboard"
)

// VerificationGateway accepts dashboard WebSocket clients and publishes biometric
// attendance verification events scoped by empresa.
type VerificationGateway struct {
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[int64]map[*verificationClient]struct{}
}

type verificationClient struct {
	empresaID int64
	conn      *websocket.Conn
	send      chan []byte
}

type verificationMessage struct {
	Type               string    `json:"type"`
	Channel            string    `json:"channel"`
	AsistenciaID       string    `json:"asistencia_id"`
	EmpleadoID         int64     `json:"empleado_id"`
	EmpresaID          int64     `json:"empresa_id"`
	SimilitudFacial    float64   `json:"similitud_facial"`
	EstatusObservacion int       `json:"estatus_observacion"`
	Timestamp          time.Time `json:"timestamp"`
}

// NewVerificationGateway creates a WebSocket gateway for asistencia verification events.
func NewVerificationGateway() *VerificationGateway {
	return &VerificationGateway{
		upgrader: websocket.Upgrader{CheckOrigin: sameHostOrNoOrigin},
		clients:  make(map[int64]map[*verificationClient]struct{}),
	}
}

// ServeHTTP upgrades requests that include a positive empresa_id query parameter.
func (g *VerificationGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	empresaID, err := parseEmpresaID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &verificationClient{empresaID: empresaID, conn: conn, send: make(chan []byte, verificationSendBuffer)}
	g.register(client)
	go client.writePump()
	client.readUntilClosed(g.unregister)
}

// PublishAsistenciaVerificada publishes one biometric verification result to the
// empresa dashboard channel. It implements service.BiometricEventPublisher.
func (g *VerificationGateway) PublishAsistenciaVerificada(ctx context.Context, event service.AsistenciaVerificadaEvent) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("verificationGateway publish cancelled: %w", ctx.Err())
	default:
	}

	payload, err := json.Marshal(verificationMessage{
		Type:               eventType(event.EstatusObservacion),
		Channel:            fmt.Sprintf(companyDashboardFormat, event.EmpresaID),
		AsistenciaID:       event.AsistenciaID.String(),
		EmpleadoID:         event.EmpleadoID,
		EmpresaID:          event.EmpresaID,
		SimilitudFacial:    event.SimilitudFacial,
		EstatusObservacion: event.EstatusObservacion,
		Timestamp:          event.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("verificationGateway encode event: %w", err)
	}

	g.mu.RLock()
	clients := make([]*verificationClient, 0, len(g.clients[event.EmpresaID]))
	for client := range g.clients[event.EmpresaID] {
		clients = append(clients, client)
	}
	g.mu.RUnlock()

	for _, client := range clients {
		select {
		case <-ctx.Done():
			return fmt.Errorf("verificationGateway publish cancelled: %w", ctx.Err())
		case client.send <- payload:
		default:
			g.unregister(client)
		}
	}
	return nil
}

func (g *VerificationGateway) register(client *verificationClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.clients[client.empresaID] == nil {
		g.clients[client.empresaID] = make(map[*verificationClient]struct{})
	}
	g.clients[client.empresaID][client] = struct{}{}
}

func (g *VerificationGateway) unregister(client *verificationClient) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if companyClients, ok := g.clients[client.empresaID]; ok {
		if _, exists := companyClients[client]; exists {
			delete(companyClients, client)
			close(client.send)
		}
		if len(companyClients) == 0 {
			delete(g.clients, client.empresaID)
		}
	}
}

func (c *verificationClient) readUntilClosed(unregister func(*verificationClient)) {
	defer func() {
		unregister(c)
		_ = c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.NextReader(); err != nil {
			return
		}
	}
}

func (c *verificationClient) writePump() {
	for payload := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			return
		}
	}
}

func parseEmpresaID(r *http.Request) (int64, error) {
	value := r.URL.Query().Get("empresa_id")
	empresaID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || empresaID <= 0 {
		return 0, fmt.Errorf("empresa_id query parameter is required")
	}
	return empresaID, nil
}

func eventType(status int) string {
	if status == empleados.ObservacionValidada {
		return "biometric_verification_passed"
	}
	return "biometric_verification_failed"
}

func sameHostOrNoOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}
