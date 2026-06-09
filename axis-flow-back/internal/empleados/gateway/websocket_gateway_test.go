package gateway_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"axis-flow-back/internal/empleados/gateway"
	"axis-flow-back/internal/empleados/service"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestGatewayBroadcastsAsistenciaVerificationToCompanyDashboard(t *testing.T) {
	gw := gateway.NewVerificationGateway()
	server := httptest.NewServer(gw)
	defer server.Close()

	client, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?empresa_id=12", nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer client.Close()

	asistenciaID := uuid.MustParse("8f37bc44-593b-48c2-a9b7-f58c73491a92")
	eventTime := time.Date(2026, 6, 8, 14, 4, 35, 0, time.UTC)
	if err := gw.PublishAsistenciaVerificada(context.Background(), service.AsistenciaVerificadaEvent{
		AsistenciaID:       asistenciaID,
		EmpleadoID:         105,
		EmpresaID:          12,
		SimilitudFacial:    52.4,
		EstatusObservacion: 3,
		Timestamp:          eventTime,
	}); err != nil {
		t.Fatalf("publish verification event: %v", err)
	}

	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read broadcast: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode broadcast: %v", err)
	}

	if got["type"] != "biometric_verification_failed" {
		t.Fatalf("type = %v, want biometric_verification_failed", got["type"])
	}
	if got["channel"] != "company:12:dashboard" {
		t.Fatalf("channel = %v, want company:12:dashboard", got["channel"])
	}
	if got["asistencia_id"] != asistenciaID.String() || got["empleado_id"].(float64) != 105 || got["empresa_id"].(float64) != 12 {
		t.Fatalf("unexpected identity fields: %#v", got)
	}
	if got["similitud_facial"].(float64) != 52.4 || got["estatus_observacion"].(float64) != 3 {
		t.Fatalf("unexpected verification fields: %#v", got)
	}
	if got["timestamp"] != eventTime.Format(time.RFC3339) {
		t.Fatalf("timestamp = %v, want %s", got["timestamp"], eventTime.Format(time.RFC3339))
	}
}

func TestGatewayScopesBroadcastsByEmpresa(t *testing.T) {
	gw := gateway.NewVerificationGateway()
	server := httptest.NewServer(gw)
	defer server.Close()

	company12, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?empresa_id=12", nil)
	if err != nil {
		t.Fatalf("dial company 12: %v", err)
	}
	defer company12.Close()

	company99, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL)+"?empresa_id=99", nil)
	if err != nil {
		t.Fatalf("dial company 99: %v", err)
	}
	defer company99.Close()

	if err := gw.PublishAsistenciaVerificada(context.Background(), service.AsistenciaVerificadaEvent{
		AsistenciaID:       uuid.MustParse("8f37bc44-593b-48c2-a9b7-f58c73491a92"),
		EmpleadoID:         105,
		EmpresaID:          12,
		SimilitudFacial:    91.2,
		EstatusObservacion: 2,
		Timestamp:          time.Date(2026, 6, 8, 14, 4, 30, 0, time.UTC),
	}); err != nil {
		t.Fatalf("publish verification event: %v", err)
	}

	_ = company12.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := company12.ReadMessage()
	if err != nil {
		t.Fatalf("company 12 should receive broadcast: %v", err)
	}
	if !strings.Contains(string(payload), "biometric_verification_passed") {
		t.Fatalf("company 12 payload = %s, want passed event", payload)
	}

	_ = company99.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, payload, err := company99.ReadMessage(); err == nil {
		t.Fatalf("company 99 received cross-company payload: %s", payload)
	}
}

func TestGatewayRejectsConnectionsWithoutEmpresaScope(t *testing.T) {
	server := httptest.NewServer(gateway.NewVerificationGateway())
	defer server.Close()

	_, resp, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	if err == nil {
		t.Fatal("dial without empresa_id unexpectedly succeeded")
	}
	if resp == nil || resp.StatusCode != 400 {
		t.Fatalf("status = %v, want 400", resp)
	}
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}
