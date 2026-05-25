package admin

import (
	"net/http"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func (h *Handler) auditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}

	actions, err := h.store.ListAdminActions(r.Context(), store.AdminActionFilter{Limit: 100})
	if err != nil {
		h.serverError(w, "failed to load audit log", err)
		return
	}

	data := h.basePageData("Audit Log", "/admin/audit", r)
	data.AdminActions = actions
	h.render(w, "audit.html", data)
}
