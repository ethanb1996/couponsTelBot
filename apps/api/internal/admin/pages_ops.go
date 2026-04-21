package admin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func (h *Handler) orders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	orders, err := h.store.ListOrdersForAdmin(r.Context(), status, 100)
	if err != nil {
		h.serverError(w, "failed to list orders", err)
		return
	}

	data := h.basePageData("Orders", r.URL.Path, r)
	data.Orders = orders
	data.SelectedStatus = status

	h.render(w, "orders.html", data)
}

func (h *Handler) orderDetail(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}

	orderID, err := parsePathID(path, "/admin/orders/")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	order, err := h.store.GetOrderForAdmin(r.Context(), orderID)
	if err != nil {
		h.serverError(w, "failed to load order detail", err)
		return
	}

	payments, err := h.store.ListPaymentsForOrder(r.Context(), orderID)
	if err != nil {
		h.serverError(w, "failed to load order payments", err)
		return
	}

	delivery, err := h.store.GetCouponDeliveryForOrder(r.Context(), orderID)
	if err != nil {
		h.serverError(w, "failed to load order delivery", err)
		return
	}

	data := h.basePageData("Order Review", r.URL.Path, r)
	data.Order = &order
	data.Payments = payments
	data.Delivery = delivery
	data.FormOrderID = fmt.Sprintf("%d", order.ID)
	if order.CouponID != nil {
		data.FormCouponID = fmt.Sprintf("%d", *order.CouponID)
	}
	data.FormUserID = fmt.Sprintf("%d", order.UserID)

	h.render(w, "order_detail.html", data)
}

func (h *Handler) support(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		supportCases, err := h.store.ListSupportCases(r.Context(), status)
		if err != nil {
			h.serverError(w, "failed to list support cases", err)
			return
		}

		data := h.basePageData("Support Cases", r.URL.Path, r)
		data.SupportCases = supportCases
		data.SelectedStatus = status
		data.FormOrderID = strings.TrimSpace(r.URL.Query().Get("order_id"))
		data.FormCouponID = strings.TrimSpace(r.URL.Query().Get("coupon_id"))
		data.FormUserID = strings.TrimSpace(r.URL.Query().Get("user_id"))

		h.render(w, "support.html", data)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			h.redirectWithError(w, r, "/admin/support", "invalid support form data")
			return
		}

		supportCase, err := h.store.CreateSupportCase(r.Context(), store.CreateSupportCaseParams{
			UserID:          parseOptionalInt64(r.PostForm.Get("user_id")),
			OrderID:         parseOptionalInt64(r.PostForm.Get("order_id")),
			CouponID:        parseOptionalInt64(r.PostForm.Get("coupon_id")),
			CaseType:        strings.TrimSpace(r.PostForm.Get("case_type")),
			Status:          "open",
			Priority:        strings.TrimSpace(r.PostForm.Get("priority")),
			Summary:         strings.TrimSpace(r.PostForm.Get("summary")),
			AssignedAdminID: h.operatorID(r),
		})
		if err != nil {
			h.redirectWithError(w, r, "/admin/support", err.Error())
			return
		}

		h.redirectWithSuccess(w, r, fmt.Sprintf("/admin/support/%d", supportCase.ID), "support case created")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) supportDetail(w http.ResponseWriter, r *http.Request, path string) {
	supportCaseID, err := parsePathID(path, "/admin/support/")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		supportCase, err := h.store.GetSupportCase(r.Context(), supportCaseID)
		if err != nil {
			h.serverError(w, "failed to load support case", err)
			return
		}

		data := h.basePageData("Support Case", r.URL.Path, r)
		data.SupportCase = &supportCase

		h.render(w, "support_detail.html", data)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			h.redirectWithError(w, r, fmt.Sprintf("/admin/support/%d", supportCaseID), "invalid support resolution form")
			return
		}

		_, err := h.store.ResolveSupportCase(r.Context(), supportCaseID, strings.TrimSpace(r.PostForm.Get("resolution_note")), h.operatorID(r))
		if err != nil {
			h.redirectWithError(w, r, fmt.Sprintf("/admin/support/%d", supportCaseID), err.Error())
			return
		}

		h.redirectWithSuccess(w, r, fmt.Sprintf("/admin/support/%d", supportCaseID), "support case resolved")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}
