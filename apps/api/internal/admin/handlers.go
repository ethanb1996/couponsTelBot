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
	logger    *slog.Logger
	store     *store.Postgres
	templates *template.Template
}

type pageData struct {
	Title       string
	CurrentPath string
	Now         time.Time
	DatabaseOK  bool
	Notes       []string
}

func NewHandler(logger *slog.Logger, db *store.Postgres) (*Handler, error) {
	parsed, err := template.ParseFS(admintemplates.FS, "*.html")
	if err != nil {
		return nil, err
	}

	return &Handler{
		logger:    logger,
		store:     db,
		templates: parsed,
	}, nil
}

func (h *Handler) Route(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimSuffix(r.URL.Path, "/") {
	case "/admin", "":
		h.dashboard(w, r)
	case "/admin/listings":
		h.listings(w, r)
	case "/admin/orders":
		h.orders(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	data := h.basePageData("Admin Dashboard", r.URL.Path, []string{
		"This admin is intentionally small and embedded in the Go service.",
		"Use it to anchor inventory, listing, order, payment, and support workflows.",
	})
	h.render(w, "dashboard.html", data)
}

func (h *Handler) listings(w http.ResponseWriter, r *http.Request) {
	data := h.basePageData("Listings", r.URL.Path, []string{
		"Listings represent sellable Telegram SKUs.",
		"One listing can have many owned coupons behind it.",
		"Publishing and pause actions belong here in the next step.",
	})
	h.render(w, "listings.html", data)
}

func (h *Handler) orders(w http.ResponseWriter, r *http.Request) {
	data := h.basePageData("Orders", r.URL.Path, []string{
		"Orders are created when a user presses Buy for one selected coupon option.",
		"Payment and delivery evidence should be visible here in later steps.",
	})
	h.render(w, "orders.html", data)
}

func (h *Handler) basePageData(title, path string, notes []string) pageData {
	databaseOK := h.store != nil
	return pageData{
		Title:       title,
		CurrentPath: path,
		Now:         time.Now().UTC(),
		DatabaseOK:  databaseOK,
		Notes:       notes,
	}
}

func (h *Handler) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("failed to render admin template", "template", name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
