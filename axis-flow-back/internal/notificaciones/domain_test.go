// Package notificaciones_test exercises domain sentinel errors and typed constants.
package notificaciones_test

import (
	"testing"

	"axis-flow-back/internal/notificaciones"
)

func TestSentinelErrors_AreNonNilAndDistinct(t *testing.T) {
	errs := []error{
		notificaciones.ErrNotFound,
		notificaciones.ErrForbidden,
		notificaciones.ErrInvalidInput,
	}

	for i, e := range errs {
		if e == nil {
			t.Errorf("sentinel error at index %d is nil", i)
		}
	}

	seen := make(map[string]bool)
	for _, e := range errs {
		key := e.Error()
		if seen[key] {
			t.Errorf("duplicate sentinel error message: %q", key)
		}
		seen[key] = true
	}
}

func TestDeviceTypeConstants(t *testing.T) {
	cases := []struct {
		name  string
		value notificaciones.DeviceType
		want  string
	}{
		{"web", notificaciones.DeviceTypeWeb, "WEB"},
		{"android", notificaciones.DeviceTypeAndroid, "ANDROID"},
		{"ios", notificaciones.DeviceTypeIOS, "IOS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.value) != tc.want {
				t.Errorf("got %q, want %q", tc.value, tc.want)
			}
		})
	}
}

func TestEstatusNotifConstants(t *testing.T) {
	if notificaciones.EstatusUnread != 1 {
		t.Errorf("EstatusUnread: got %d, want 1", notificaciones.EstatusUnread)
	}
	if notificaciones.EstatusRead != 2 {
		t.Errorf("EstatusRead: got %d, want 2", notificaciones.EstatusRead)
	}
	if notificaciones.EstatusUnread == notificaciones.EstatusRead {
		t.Error("EstatusUnread and EstatusRead must not be equal")
	}
}
