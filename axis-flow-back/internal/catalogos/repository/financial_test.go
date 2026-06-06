package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"axis-flow-back/internal/catalogos/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory mocks — financial
// ---------------------------------------------------------------------------

type mockBankRepo struct {
	banks  []domain.Bank
	nextID int64
}

func newMockBankRepo() *mockBankRepo { return &mockBankRepo{nextID: 1} }

func (m *mockBankRepo) List(_ context.Context) ([]domain.Bank, error) {
	var out []domain.Bank
	for _, b := range m.banks {
		if b.DeletedAt == nil {
			out = append(out, b)
		}
	}
	return out, nil
}

func (m *mockBankRepo) Create(_ context.Context, b *domain.Bank) error {
	for _, existing := range m.banks {
		if existing.Code == b.Code && existing.DeletedAt == nil {
			return domain.ErrDuplicateCode
		}
	}
	b.ID = m.nextID
	m.nextID++
	b.CreatedAt = time.Now()
	b.UpdatedAt = time.Now()
	m.banks = append(m.banks, *b)
	return nil
}

// ---------------------------------------------------------------------------

type mockTaxRegimeRepo struct {
	regimes []domain.TaxRegime
	nextID  int64
}

func newMockTaxRegimeRepo() *mockTaxRegimeRepo { return &mockTaxRegimeRepo{nextID: 1} }

func (m *mockTaxRegimeRepo) List(_ context.Context) ([]domain.TaxRegime, error) {
	var out []domain.TaxRegime
	for _, t := range m.regimes {
		if t.DeletedAt == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockTaxRegimeRepo) Create(_ context.Context, t *domain.TaxRegime) error {
	t.ID = m.nextID
	m.nextID++
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	m.regimes = append(m.regimes, *t)
	return nil
}

// ---------------------------------------------------------------------------

type mockPaymentConditionRepo struct {
	conditions []domain.PaymentCondition
	nextID     int64
}

func newMockPaymentConditionRepo() *mockPaymentConditionRepo {
	return &mockPaymentConditionRepo{nextID: 1}
}

func (m *mockPaymentConditionRepo) Create(_ context.Context, p *domain.PaymentCondition) error {
	if p.Days < 0 {
		return fmt.Errorf("payment_condition: days must be >= 0")
	}
	p.ID = m.nextID
	m.nextID++
	m.conditions = append(m.conditions, *p)
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestBankRepo_List_ReturnsOnlyNonDeleted(t *testing.T) {
	repo := newMockBankRepo()
	ctx := context.Background()

	now := time.Now()
	b1 := domain.Bank{Code: "BBVA", Name: "BBVA Mexico"}
	b2 := domain.Bank{Code: "HSBC", Name: "HSBC"}

	require.NoError(t, repo.Create(ctx, &b1))
	require.NoError(t, repo.Create(ctx, &b2))

	repo.banks[1].DeletedAt = &now

	got, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "BBVA", got[0].Code)
}

func TestBankRepo_Create_Succeeds(t *testing.T) {
	repo := newMockBankRepo()
	ctx := context.Background()

	b := domain.Bank{Code: "SANT", Name: "Santander"}
	require.NoError(t, repo.Create(ctx, &b))
	assert.NotZero(t, b.ID)
}

func TestTaxRegimeRepo_List_IncludesPersonaFields(t *testing.T) {
	repo := newMockTaxRegimeRepo()
	ctx := context.Background()

	tr := domain.TaxRegime{
		Code:          "601",
		Name:          "General de Ley Personas Morales",
		PersonaFisica: false,
		PersonaMoral:  true,
	}
	require.NoError(t, repo.Create(ctx, &tr))

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.False(t, got[0].PersonaFisica)
	assert.True(t, got[0].PersonaMoral)
}

func TestPaymentConditionRepo_Create_WithNegativeDays_Fails(t *testing.T) {
	repo := newMockPaymentConditionRepo()
	ctx := context.Background()

	p := domain.PaymentCondition{Code: "NEG", Name: "Negative", Days: -1}
	err := repo.Create(ctx, &p)
	assert.Error(t, err)
}
