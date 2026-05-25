package admin

import (
	"fmt"
	"net/http"
	"strings"
)

func (h *Handler) paymentClaims(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderPaymentClaimsPage(w, r)
	case http.MethodPost:
		if h.payBox == nil {
			h.redirectWithError(w, r, "/admin/payments", "PayBox service is not configured")
			return
		}
		if err := r.ParseForm(); err != nil {
			h.redirectWithError(w, r, "/admin/payments", "invalid payment review form")
			return
		}

		claimID, err := parseInt64Field(r.PostForm, "claim_id")
		if err != nil {
			h.redirectWithError(w, r, "/admin/payments", "claim id is required")
			return
		}

		action := strings.TrimSpace(r.PostForm.Get("action"))
		note := strings.TrimSpace(r.PostForm.Get("review_note"))
		operator := h.operatorID(r)

		switch action {
		case "approve":
			_, err := h.payBox.ApproveClaim(r.Context(), claimID, operator, note)
			if err != nil {
				h.redirectWithError(w, r, "/admin/payments", err.Error())
				return
			}
			h.redirectWithSuccess(w, r, "/admin/payments", fmt.Sprintf("claim %d approved and QR delivery started", claimID))
		case "reject":
			_, _, err := h.payBox.RejectClaim(r.Context(), claimID, operator, note)
			if err != nil {
				h.redirectWithError(w, r, "/admin/payments", err.Error())
				return
			}
			h.redirectWithSuccess(w, r, "/admin/payments", fmt.Sprintf("claim %d rejected", claimID))
		default:
			h.redirectWithError(w, r, "/admin/payments", "unknown payment review action")
		}
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) renderPaymentClaimsPage(w http.ResponseWriter, r *http.Request) {
	data := h.basePageData("Payment claims", "/admin/payments", r)
	claims, err := h.store.ListPendingManualPaymentClaims(r.Context(), 100)
	if err != nil {
		h.serverError(w, "failed to list PayBox payment claims", err)
		return
	}
	data.PaymentClaims = claims
	h.render(w, "payments.html", data)
}
