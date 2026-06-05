package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"axis-flow-back/internal/crypto"
	"axis-flow-back/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"
)

// ErrUnauthorized is returned when authentication or authorisation fails.
var ErrUnauthorized = errors.New("unauthorized")

// TokenPair holds a newly issued access/refresh token pair.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Claims are the JWT payload claims used by this service.
type Claims struct {
	UserID   string `json:"uid"`
	TenantID string `json:"tid"`
	Email    string `json:"email"`
	Role     string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

// AuthService handles login, token refresh, and profile retrieval.
type AuthService struct {
	users      UserRepository
	sessions   SessionRepository
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewAuthService creates an AuthService with the given dependencies.
func NewAuthService(
	users UserRepository,
	sessions SessionRepository,
	jwtSecret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{
		users:      users,
		sessions:   sessions,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Login authenticates a user and returns a token pair on success.
// deviceType must be one of: WEB, ANDROID, IOS.
func (s *AuthService) Login(
	ctx context.Context,
	email, password string,
	deviceType, deviceID, ipAddress string,
) (TokenPair, error) {
	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID().String()

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		slog.ErrorContext(ctx, "login: find user by email",
			slog.String("error", err.Error()),
			slog.String("trace_id", traceID),
		)
		return TokenPair{}, ErrUnauthorized
	}

	if !domain.CanLogin(user.Status) {
		slog.WarnContext(ctx, "login: account not active",
			slog.String("user_id", user.ID.String()),
			slog.String("status", string(user.Status)),
			slog.String("trace_id", traceID),
		)
		return TokenPair{}, ErrUnauthorized
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// Persist the failed attempt; ignore secondary errors to keep the primary error clean
		_ = s.users.IncrementFailedAttempts(ctx, user.ID)
		slog.WarnContext(ctx, "login: wrong password",
			slog.String("user_id", user.ID.String()),
			slog.String("trace_id", traceID),
		)
		return TokenPair{}, ErrUnauthorized
	}

	// Successful auth — reset counter and record login time
	now := time.Now()
	_ = s.users.ResetFailedAttempts(ctx, user.ID)
	_ = s.users.UpdateLastLogin(ctx, user.ID, now)

	pair, err := s.issueTokenPair(ctx, user, domain.DeviceType(deviceType), deviceID, ipAddress)
	if err != nil {
		return TokenPair{}, fmt.Errorf("login: issue tokens: %w", err)
	}

	slog.InfoContext(ctx, "login: success",
		slog.String("user_id", user.ID.String()),
		slog.String("trace_id", traceID),
	)
	return pair, nil
}

// GetPermissionCodes returns all permission codes assigned to the user's roles.
func (s *AuthService) GetPermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	codes, err := s.users.ListPermissionCodes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get permission codes: %w", err)
	}
	return codes, nil
}

// RefreshToken validates a refresh token and returns a new token pair.
func (s *AuthService) RefreshToken(ctx context.Context, rawRefreshToken string) (TokenPair, error) {
	hash := hashToken(rawRefreshToken)

	sess, err := s.sessions.FindByRefreshTokenHash(ctx, hash)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}

	if sess.RevokedAt != nil {
		slog.WarnContext(ctx, "refresh: token already revoked", slog.String("session_id", sess.ID.String()))
		return TokenPair{}, ErrUnauthorized
	}

	if time.Now().After(sess.ExpiresAt) {
		return TokenPair{}, ErrUnauthorized
	}

	user, err := s.users.FindByID(ctx, sess.UserID)
	if err != nil {
		return TokenPair{}, ErrUnauthorized
	}

	if !domain.CanLogin(user.Status) {
		return TokenPair{}, ErrUnauthorized
	}

	// Rotate: revoke the old session and issue a new pair
	if err := s.sessions.Revoke(ctx, sess.ID); err != nil {
		return TokenPair{}, fmt.Errorf("refresh: revoke session: %w", err)
	}

	pair, err := s.issueTokenPair(ctx, user, sess.DeviceType, sess.DeviceID, sess.IPAddress)
	if err != nil {
		return TokenPair{}, fmt.Errorf("refresh: issue tokens: %w", err)
	}

	return pair, nil
}

// GetProfile returns the user associated with the given user ID.
func (s *AuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return user, nil
}

// ParseAccessToken validates a JWT and returns the embedded Claims.
func (s *AuthService) ParseAccessToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrUnauthorized
	}
	return claims, nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func (s *AuthService) issueTokenPair(
	ctx context.Context,
	user *domain.User,
	deviceType domain.DeviceType,
	deviceID, ipAddress string,
) (TokenPair, error) {
	// Resolve the user's primary role to embed in the JWT.
	// Fail-open: if the lookup errors, the token is issued without a role.
	roleCode, _ := s.users.FindPrimaryRoleCode(ctx, user.ID)

	// Access token (JWT)
	now := time.Now()
	claims := &Claims{
		UserID:   user.ID.String(),
		TenantID: user.TenantID.String(),
		Email:    user.Email,
		Role:     roleCode,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("sign access token: %w", err)
	}

	// Refresh token: cryptographically random 32 bytes
	rawRefresh, err := generateRandomToken()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	sess := &domain.Session{
		ID:               uuid.New(),
		UserID:           user.ID,
		RefreshTokenHash: hashToken(rawRefresh),
		DeviceType:       deviceType,
		DeviceID:         deviceID,
		IPAddress:        ipAddress,
		ExpiresAt:        now.Add(s.refreshTTL),
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return TokenPair{}, fmt.Errorf("persist session: %w", err)
	}

	return TokenPair{AccessToken: accessToken, RefreshToken: rawRefresh}, nil
}

// generateRandomToken returns a hex-encoded 32-byte random token.
func generateRandomToken() (string, error) {
	return crypto.GenerateRandomToken()
}

// hashToken returns the SHA-256 hex digest of token.
func hashToken(token string) string {
	return crypto.HashToken(token)
}
