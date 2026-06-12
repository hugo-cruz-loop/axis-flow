package signing_test

import (
	"testing"

	"axis-flow-back/internal/report/signing"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		expires string
		userID  string
		secret  string
		mutate  func(key, expires, userID, sig, secret string) (string, string, string, string, string)
		want    bool
	}{
		{
			name:    "valid round-trip",
			key:     "reports/abc.pdf",
			expires: "2026-01-01T00:00:00Z",
			userID:  "user-42",
			secret:  "supersecret",
			mutate:  func(k, e, u, s, sec string) (string, string, string, string, string) { return k, e, u, s, sec },
			want:    true,
		},
		{
			name:    "tampered key",
			key:     "reports/abc.pdf",
			expires: "2026-01-01T00:00:00Z",
			userID:  "user-42",
			secret:  "supersecret",
			mutate:  func(k, e, u, s, sec string) (string, string, string, string, string) { return "reports/evil.pdf", e, u, s, sec },
			want:    false,
		},
		{
			name:    "tampered expires",
			key:     "reports/abc.pdf",
			expires: "2026-01-01T00:00:00Z",
			userID:  "user-42",
			secret:  "supersecret",
			mutate:  func(k, e, u, s, sec string) (string, string, string, string, string) { return k, "2099-01-01T00:00:00Z", u, s, sec },
			want:    false,
		},
		{
			name:    "tampered userID",
			key:     "reports/abc.pdf",
			expires: "2026-01-01T00:00:00Z",
			userID:  "user-42",
			secret:  "supersecret",
			mutate:  func(k, e, u, s, sec string) (string, string, string, string, string) { return k, e, "user-evil", s, sec },
			want:    false,
		},
		{
			name:    "different secret",
			key:     "reports/abc.pdf",
			expires: "2026-01-01T00:00:00Z",
			userID:  "user-42",
			secret:  "supersecret",
			mutate:  func(k, e, u, s, sec string) (string, string, string, string, string) { return k, e, u, s, "wrongsecret" },
			want:    false,
		},
		{
			name:    "empty sig",
			key:     "reports/abc.pdf",
			expires: "2026-01-01T00:00:00Z",
			userID:  "user-42",
			secret:  "supersecret",
			mutate:  func(k, e, u, s, sec string) (string, string, string, string, string) { return k, e, u, "", sec },
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sig := signing.Sign(tc.key, tc.expires, tc.userID, tc.secret)
			k, e, u, s, sec := tc.mutate(tc.key, tc.expires, tc.userID, sig, tc.secret)
			got := signing.Verify(k, e, u, s, sec)
			if got != tc.want {
				t.Errorf("Verify(...) = %v; want %v", got, tc.want)
			}
		})
	}
}
