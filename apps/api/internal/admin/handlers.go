package admin

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
	admintemplates "github.com/ethanb1996/couponsTelBot/apps/api/templates"
)

type Handler struct {
	logger              *slog.Logger
	store               *store.Postgres
	templates           *template.Template
	couponEncryptionKey string
}

type pageData struct {
	Title             string
	CurrentPath       string
	Now               time.Time
	DatabaseOK        bool
	ErrorMessage      string
	SuccessMessage    string
	SourceTypes       []string
	RightsStatuses    []string
	RiskRatings       []string
	ListingStatuses   []string
	CouponStatuses    []string
	OrderStatuses     []string
	SupportStatuses   []string
	SupportPriorities []string
	CaseTypes         []string
	TransferStatuses  []string
	SelectedStatus    string
	SelectedListingID int64
	Dashboard         dashboardView
	Sources           []store.CouponSource
	Source            *store.CouponSource
	Listings          []store.ListingInventorySummary
	Listing           *store.ListingInventorySummary
	Coupons           []store.CouponAdminSummary
	Orders            []store.OrderAdminSummary
	Order             *store.OrderAdminSummary
	Payments          []store.Payment
	Delivery          *store.CouponDelivery
	SupportCases      []store.SupportCaseSummary
	SupportCase       *store.SupportCaseSummary
	AdminActions      []store.AdminAction
	DeliveryAlerts    []store.PaidUndeliveredOrderAlert
	ReconcileQueue    []store.PendingPaymentReconciliationCandidate
	ImportExample     string
	FormOrderID       string
	FormCouponID      string
	FormUserID        string
}

type dashboardView struct {
	SourceCount          int
	ListingCount         int
	ActiveListingCount   int
	AvailableCouponCount int64
	RecentOrderCount     int
	OpenSupportCount     int
	DeliveryAlertCount   int
	ReconcileQueueCount  int
	RecentAuditCount     int
}

func NewHandler(logger *slog.Logger, db *store.Postgres, couponEncryptionKey string) (*Handler, error) {
	parsed, err := template.ParseFS(admintemplates.FS, "*.html")
	if err != nil {
		return nil, err
	}

	return &Handler{
		logger:              logger,
		store:               db,
		templates:           parsed,
		couponEncryptionKey: couponEncryptionKey,
	}, nil
}

func (h *Handler) Route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/admin"
	}

	switch {
	case path == "/admin":
		h.dashboard(w, r)
	case path == "/admin/sources":
		h.sources(w, r)
	case strings.HasPrefix(path, "/admin/sources/"):
		h.sourceDetail(w, r, path)
	case path == "/admin/listings":
		h.listings(w, r)
	case strings.HasPrefix(path, "/admin/listings/"):
		h.listingDetail(w, r, path)
	case path == "/admin/coupons":
		h.coupons(w, r)
	case path == "/admin/orders":
		h.orders(w, r)
	case strings.HasPrefix(path, "/admin/orders/"):
		h.orderDetail(w, r, path)
	case path == "/admin/support":
		h.support(w, r)
	case strings.HasPrefix(path, "/admin/support/"):
		h.supportDetail(w, r, path)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) basePageData(title, path string, r *http.Request) pageData {
	return pageData{
		Title:             title,
		CurrentPath:       path,
		Now:               time.Now().UTC(),
		DatabaseOK:        h.store != nil,
		ErrorMessage:      strings.TrimSpace(r.URL.Query().Get("error")),
		SuccessMessage:    strings.TrimSpace(r.URL.Query().Get("success")),
		SourceTypes:       []string{"merchant_partner", "reseller", "licensed_distributor", "manual_source"},
		RightsStatuses:    []string{"unknown", "review_pending", "approved", "restricted", "rejected"},
		RiskRatings:       []string{"low", "medium", "high"},
		ListingStatuses:   []string{"draft", "active", "paused", "sold_out", "expired", "removed"},
		CouponStatuses:    []string{"available", "reserved", "assigned", "delivered", "expired", "voided", "disputed"},
		OrderStatuses:     []string{"draft", "pending_payment", "paid", "delivery_pending", "delivered", "failed", "cancelled", "disputed"},
		SupportStatuses:   []string{"open", "in_progress", "waiting_on_user", "resolved", "closed"},
		SupportPriorities: []string{"low", "medium", "high"},
		CaseTypes:         []string{"invalid_coupon", "delivery_issue", "payment_issue", "chargeback_review", "other"},
		TransferStatuses:  []string{"unknown", "not_transferable", "transferable_with_review", "transferable"},
	}
}

func (h *Handler) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("failed to render admin template", "template", name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
