package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	catalogoscache "axis-flow-back/internal/catalogos/cache"
	"axis-flow-back/internal/catalogos/domain"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

// ── Consumer interfaces ───────────────────────────────────────────────────────

type BankRepositorier interface {
	List(ctx context.Context) ([]domain.Bank, error)
	FindByID(ctx context.Context, id int64) (*domain.Bank, error)
	Create(ctx context.Context, b *domain.Bank) error
	Update(ctx context.Context, b *domain.Bank) error
	Delete(ctx context.Context, id int64) error
}

type TaxRegimeRepositorier interface {
	List(ctx context.Context) ([]domain.TaxRegime, error)
	FindByID(ctx context.Context, id int64) (*domain.TaxRegime, error)
	Create(ctx context.Context, t *domain.TaxRegime) error
	Update(ctx context.Context, t *domain.TaxRegime) error
	Delete(ctx context.Context, id int64) error
}

type PaymentFormRepositorier interface {
	List(ctx context.Context) ([]domain.PaymentForm, error)
	FindByID(ctx context.Context, id int64) (*domain.PaymentForm, error)
	Create(ctx context.Context, p *domain.PaymentForm) error
	Update(ctx context.Context, p *domain.PaymentForm) error
	Delete(ctx context.Context, id int64) error
}

type PaymentConditionRepositorier interface {
	List(ctx context.Context) ([]domain.PaymentCondition, error)
	FindByID(ctx context.Context, id int64) (*domain.PaymentCondition, error)
	Create(ctx context.Context, p *domain.PaymentCondition) error
	Update(ctx context.Context, p *domain.PaymentCondition) error
	Delete(ctx context.Context, id int64) error
}

// ── Handler ───────────────────────────────────────────────────────────────────

type FinancialHandler struct {
	banks             BankRepositorier
	taxRegimes        TaxRegimeRepositorier
	paymentForms      PaymentFormRepositorier
	paymentConditions PaymentConditionRepositorier
	rdb               *redis.Client
}

func NewFinancialHandler(
	banks BankRepositorier,
	taxRegimes TaxRegimeRepositorier,
	paymentForms PaymentFormRepositorier,
	paymentConditions PaymentConditionRepositorier,
	rdb *redis.Client,
) *FinancialHandler {
	return &FinancialHandler{
		banks:             banks,
		taxRegimes:        taxRegimes,
		paymentForms:      paymentForms,
		paymentConditions: paymentConditions,
		rdb:               rdb,
	}
}

// ── Banks ─────────────────────────────────────────────────────────────────────

func (h *FinancialHandler) ListBanks(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:banks:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.Bank, error) { return h.banks.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *FinancialHandler) CreateBank(w http.ResponseWriter, r *http.Request) {
	var b domain.Bank
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.banks.Create(r.Context(), &b); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:banks:all")
	writeJSON(w, http.StatusCreated, b)
}

func (h *FinancialHandler) UpdateBank(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var b domain.Bank
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	b.ID = id
	if err := h.banks.Update(r.Context(), &b); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:banks:all")
	writeJSON(w, http.StatusOK, b)
}

func (h *FinancialHandler) DeleteBank(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.banks.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:banks:all")
	w.WriteHeader(http.StatusNoContent)
}

// ── Tax Regimes ───────────────────────────────────────────────────────────────

func (h *FinancialHandler) ListTaxRegimes(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:tax_regimes:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.TaxRegime, error) { return h.taxRegimes.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *FinancialHandler) CreateTaxRegime(w http.ResponseWriter, r *http.Request) {
	var t domain.TaxRegime
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.taxRegimes.Create(r.Context(), &t); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:tax_regimes:all")
	writeJSON(w, http.StatusCreated, t)
}

func (h *FinancialHandler) UpdateTaxRegime(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var t domain.TaxRegime
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	t.ID = id
	if err := h.taxRegimes.Update(r.Context(), &t); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:tax_regimes:all")
	writeJSON(w, http.StatusOK, t)
}

func (h *FinancialHandler) DeleteTaxRegime(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.taxRegimes.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:tax_regimes:all")
	w.WriteHeader(http.StatusNoContent)
}

// ── Payment Forms ─────────────────────────────────────────────────────────────

func (h *FinancialHandler) ListPaymentForms(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:payment_forms:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.PaymentForm, error) { return h.paymentForms.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *FinancialHandler) CreatePaymentForm(w http.ResponseWriter, r *http.Request) {
	var p domain.PaymentForm
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.paymentForms.Create(r.Context(), &p); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:payment_forms:all")
	writeJSON(w, http.StatusCreated, p)
}

func (h *FinancialHandler) UpdatePaymentForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var p domain.PaymentForm
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	p.ID = id
	if err := h.paymentForms.Update(r.Context(), &p); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:payment_forms:all")
	writeJSON(w, http.StatusOK, p)
}

func (h *FinancialHandler) DeletePaymentForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.paymentForms.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:payment_forms:all")
	w.WriteHeader(http.StatusNoContent)
}

// ── Payment Conditions ────────────────────────────────────────────────────────

func (h *FinancialHandler) ListPaymentConditions(w http.ResponseWriter, r *http.Request) {
	const key = "catalog:payment_conditions:all"
	items, err := catalogoscache.Aside(r.Context(), h.rdb, key, catalogoscache.DefaultTTL,
		func(ctx context.Context) ([]domain.PaymentCondition, error) { return h.paymentConditions.List(ctx) })
	if err != nil {
		writeCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *FinancialHandler) CreatePaymentCondition(w http.ResponseWriter, r *http.Request) {
	var p domain.PaymentCondition
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.paymentConditions.Create(r.Context(), &p); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:payment_conditions:all")
	writeJSON(w, http.StatusCreated, p)
}

func (h *FinancialHandler) UpdatePaymentCondition(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var p domain.PaymentCondition
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	p.ID = id
	if err := h.paymentConditions.Update(r.Context(), &p); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:payment_conditions:all")
	writeJSON(w, http.StatusOK, p)
}

func (h *FinancialHandler) DeletePaymentCondition(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.paymentConditions.Delete(r.Context(), id); err != nil {
		writeCatalogError(w, err)
		return
	}
	_ = catalogoscache.Del(r.Context(), h.rdb, "catalog:payment_conditions:all")
	w.WriteHeader(http.StatusNoContent)
}
