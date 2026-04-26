package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ethanb1996/couponsTelBot/apps/api/internal/store"
)

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.methodNotAllowed(w, http.MethodGet)
		return
	}

	data := h.basePageData("Admin Dashboard", r.URL.Path, r)

	sources, err := h.store.ListCouponSources(r.Context())
	if err != nil {
		h.serverError(w, "failed to list coupon sources", err)
		return
	}

	listings, err := h.store.ListListingsForAdmin(r.Context())
	if err != nil {
		h.serverError(w, "failed to list admin listings", err)
		return
	}

	orders, err := h.store.ListOrdersForAdmin(r.Context(), "", 10)
	if err != nil {
		h.serverError(w, "failed to list recent orders", err)
		return
	}

	supportCases, err := h.store.ListSupportCases(r.Context(), "open")
	if err != nil {
		h.serverError(w, "failed to list support cases", err)
		return
	}

	deliveryAlerts, err := h.store.ListPaidUndeliveredOrders(r.Context(), 5*time.Minute, 10)
	if err != nil {
		h.serverError(w, "failed to list delivery alerts", err)
		return
	}

	reconcileQueue, err := h.store.ListPendingPaymentReconciliationCandidates(r.Context(), 2*time.Minute, 10)
	if err != nil {
		h.serverError(w, "failed to list pending reconciliations", err)
		return
	}

	adminActions, err := h.store.ListAdminActions(r.Context(), store.AdminActionFilter{Limit: 10})
	if err != nil {
		h.serverError(w, "failed to list admin actions", err)
		return
	}

	var activeListingCount int
	var availableCouponCount int64
	for _, listing := range listings {
		if listing.Status == "active" {
			activeListingCount++
		}
		availableCouponCount += listing.AvailableCoupons
	}

	data.Dashboard = dashboardView{
		SourceCount:          len(sources),
		ListingCount:         len(listings),
		ActiveListingCount:   activeListingCount,
		AvailableCouponCount: availableCouponCount,
		RecentOrderCount:     len(orders),
		OpenSupportCount:     len(supportCases),
		DeliveryAlertCount:   len(deliveryAlerts),
		ReconcileQueueCount:  len(reconcileQueue),
		RecentAuditCount:     len(adminActions),
	}
	data.DeliveryAlerts = deliveryAlerts
	data.ReconcileQueue = reconcileQueue
	data.AdminActions = adminActions

	h.render(w, "dashboard.html", data)
}

func (h *Handler) sources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderSourcesPage(w, r, nil)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			h.redirectWithError(w, r, "/admin/sources", "invalid source form data")
			return
		}

		isActive := r.PostForm.Get("is_active") == "on"
		source, err := h.store.CreateCouponSource(r.Context(), store.CreateCouponSourceParams{
			SourceName:        strings.TrimSpace(r.PostForm.Get("source_name")),
			SourceType:        strings.TrimSpace(r.PostForm.Get("source_type")),
			ContactReference:  strings.TrimSpace(r.PostForm.Get("contact_reference")),
			RightsStatus:      strings.TrimSpace(r.PostForm.Get("rights_status")),
			VerificationNotes: strings.TrimSpace(r.PostForm.Get("verification_notes")),
			RiskRating:        strings.TrimSpace(r.PostForm.Get("risk_rating")),
			IsActive:          boolPtr(isActive),
		})
		if err != nil {
			h.redirectWithError(w, r, "/admin/sources", err.Error())
			return
		}

		h.recordAdminAction(r.Context(), h.operatorID(r), "coupon_source", source.ID, "create_source", nil, source, "coupon source created in admin")

		h.redirectWithSuccess(w, r, "/admin/sources", "coupon source created")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) sourceDetail(w http.ResponseWriter, r *http.Request, path string) {
	sourceID, err := parsePathID(path, "/admin/sources/")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		data, loadErr := h.loadSourceDetailPage(r, sourceID)
		if loadErr != nil {
			h.serverError(w, "failed to load source detail", loadErr)
			return
		}
		h.render(w, "source_detail.html", data)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			h.redirectWithError(w, r, fmt.Sprintf("/admin/sources/%d", sourceID), "invalid source form data")
			return
		}

		before, err := h.store.GetCouponSource(r.Context(), sourceID)
		if err != nil {
			h.redirectWithError(w, r, fmt.Sprintf("/admin/sources/%d", sourceID), err.Error())
			return
		}

		isActive := r.PostForm.Get("is_active") == "on"
		source, err := h.store.UpdateCouponSource(r.Context(), sourceID, store.CreateCouponSourceParams{
			SourceName:        strings.TrimSpace(r.PostForm.Get("source_name")),
			SourceType:        strings.TrimSpace(r.PostForm.Get("source_type")),
			ContactReference:  strings.TrimSpace(r.PostForm.Get("contact_reference")),
			RightsStatus:      strings.TrimSpace(r.PostForm.Get("rights_status")),
			VerificationNotes: strings.TrimSpace(r.PostForm.Get("verification_notes")),
			RiskRating:        strings.TrimSpace(r.PostForm.Get("risk_rating")),
			IsActive:          boolPtr(isActive),
		})
		if err != nil {
			h.redirectWithError(w, r, fmt.Sprintf("/admin/sources/%d", sourceID), err.Error())
			return
		}

		h.recordAdminAction(r.Context(), h.operatorID(r), "coupon_source", source.ID, "update_source", before, source, "coupon source updated in admin")

		h.redirectWithSuccess(w, r, fmt.Sprintf("/admin/sources/%d", sourceID), "coupon source updated")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) listings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderListingsPage(w, r)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			h.redirectWithError(w, r, "/admin/listings", "invalid listing form data")
			return
		}

		listing, err := h.store.CreateListing(r.Context(), store.CreateListingParams{
			MerchantName:            strings.TrimSpace(r.PostForm.Get("merchant_name")),
			Title:                   strings.TrimSpace(r.PostForm.Get("title")),
			Description:             strings.TrimSpace(r.PostForm.Get("description")),
			CouponValueAmount:       parseInt64Default(r.PostForm.Get("coupon_value_amount"), 0),
			SalePriceAmount:         parseInt64Default(r.PostForm.Get("sale_price_amount"), 0),
			ResellPriceAmount:       parseInt64Default(r.PostForm.Get("resell_price_amount"), 0),
			CurrencyCode:            strings.TrimSpace(r.PostForm.Get("currency_code")),
			ExpirySummary:           strings.TrimSpace(r.PostForm.Get("expiry_summary")),
			TermsSummary:            strings.TrimSpace(r.PostForm.Get("terms_summary")),
			RedemptionInstructions:  strings.TrimSpace(r.PostForm.Get("redemption_instructions")),
			FinalSaleDisclosureText: strings.TrimSpace(r.PostForm.Get("final_sale_disclosure_text")),
			Status:                  "draft",
			CreatedByAdminID:        h.operatorID(r),
		})
		if err != nil {
			h.redirectWithError(w, r, "/admin/listings", err.Error())
			return
		}

		h.recordAdminAction(r.Context(), h.operatorID(r), "listing", listing.ID, "create_listing", nil, listing, "listing created in admin")

		h.redirectWithSuccess(w, r, fmt.Sprintf("/admin/listings/%d", listing.ID), "listing created")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) listingDetail(w http.ResponseWriter, r *http.Request, path string) {
	listingID, err := parsePathID(path, "/admin/listings/")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.renderListingDetailPage(w, r, listingID)
	case http.MethodPost:
		action, err := h.parseListingDetailAction(r)
		if err != nil {
			h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), err.Error())
			return
		}

		switch action {
		case "save_listing":
			before, err := h.store.GetListing(r.Context(), listingID)
			if err != nil {
				h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), err.Error())
				return
			}

			listing, err := h.store.UpdateListing(r.Context(), listingID, store.CreateListingParams{
				MerchantName:            strings.TrimSpace(r.FormValue("merchant_name")),
				Title:                   strings.TrimSpace(r.FormValue("title")),
				Description:             strings.TrimSpace(r.FormValue("description")),
				CouponValueAmount:       parseInt64Default(r.FormValue("coupon_value_amount"), 0),
				SalePriceAmount:         parseInt64Default(r.FormValue("sale_price_amount"), 0),
				ResellPriceAmount:       parseInt64Default(r.FormValue("resell_price_amount"), 0),
				CurrencyCode:            strings.TrimSpace(r.FormValue("currency_code")),
				ExpirySummary:           strings.TrimSpace(r.FormValue("expiry_summary")),
				TermsSummary:            strings.TrimSpace(r.FormValue("terms_summary")),
				RedemptionInstructions:  strings.TrimSpace(r.FormValue("redemption_instructions")),
				FinalSaleDisclosureText: strings.TrimSpace(r.FormValue("final_sale_disclosure_text")),
				CreatedByAdminID:        h.operatorID(r),
			})
			if err != nil {
				h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), err.Error())
				return
			}
			h.recordAdminAction(r.Context(), h.operatorID(r), "listing", listing.ID, "update_listing", before, listing, "listing details updated")
			h.redirectWithSuccess(w, r, fmt.Sprintf("/admin/listings/%d", listingID), "listing updated")
		case "publish_listing":
			h.updateListingStatus(w, r, listingID, "active", "publish_listing", "listing published")
		case "pause_listing":
			h.updateListingStatus(w, r, listingID, "paused", "pause_listing", "listing paused")
		case "mark_sold_out":
			h.updateListingStatus(w, r, listingID, "sold_out", "mark_sold_out", "listing marked sold out")
		case "mark_expired":
			h.updateListingStatus(w, r, listingID, "expired", "mark_expired", "listing marked expired")
		case "import_coupons":
			before, err := h.store.GetListingForAdmin(r.Context(), listingID)
			if err != nil {
				h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), err.Error())
				return
			}
			if err := h.importCouponsForListing(r, listingID); err != nil {
				h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), err.Error())
				return
			}
			after, err := h.store.GetListingForAdmin(r.Context(), listingID)
			if err == nil {
				h.recordAdminAction(r.Context(), h.operatorID(r), "listing", listingID, "import_coupons", before, after, "coupon inventory imported for listing")
			}
			h.redirectWithSuccess(w, r, fmt.Sprintf("/admin/listings/%d", listingID), "coupon inventory ingested")
		default:
			h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), "unknown listing action")
		}
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) coupons(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderCouponsPage(w, r)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			h.redirectWithError(w, r, "/admin/coupons", "invalid coupon action form data")
			return
		}

		couponID, err := parseInt64Field(r.PostForm, "coupon_id")
		if err != nil {
			h.redirectWithError(w, r, "/admin/coupons", "coupon id is required")
			return
		}

		var nextStatus string
		switch strings.TrimSpace(r.PostForm.Get("action")) {
		case "mark_voided":
			nextStatus = "voided"
		case "mark_disputed":
			nextStatus = "disputed"
		case "mark_available":
			nextStatus = "available"
		default:
			h.redirectWithError(w, r, "/admin/coupons", "unknown coupon action")
			return
		}

		before, err := h.store.GetCoupon(r.Context(), couponID)
		if err != nil {
			h.redirectWithError(w, r, "/admin/coupons", err.Error())
			return
		}

		coupon, err := h.store.UpdateCouponInventoryStatus(r.Context(), couponID, nextStatus)
		if err != nil {
			h.redirectWithError(w, r, "/admin/coupons", err.Error())
			return
		}

		h.recordAdminAction(r.Context(), h.operatorID(r), "coupon", coupon.ID, "update_coupon_inventory", before, coupon, "coupon inventory status changed to "+nextStatus)

		h.redirectWithSuccess(w, r, "/admin/coupons", "coupon inventory updated")
	default:
		h.methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) renderSourcesPage(w http.ResponseWriter, r *http.Request, source *store.CouponSource) {
	sources, err := h.store.ListCouponSources(r.Context())
	if err != nil {
		h.serverError(w, "failed to list coupon sources", err)
		return
	}

	data := h.basePageData("Coupon Sources", r.URL.Path, r)
	data.Sources = sources
	data.Source = source

	h.render(w, "sources.html", data)
}

func (h *Handler) loadSourceDetailPage(r *http.Request, sourceID int64) (pageData, error) {
	source, err := h.store.GetCouponSource(r.Context(), sourceID)
	if err != nil {
		return pageData{}, err
	}

	actions, err := h.store.ListAdminActions(r.Context(), store.AdminActionFilter{
		EntityType: "coupon_source",
		EntityID:   &sourceID,
		Limit:      20,
	})
	if err != nil {
		return pageData{}, err
	}

	data := h.basePageData("Coupon Source", r.URL.Path, r)
	data.Source = &source
	data.AdminActions = actions
	return data, nil
}

func (h *Handler) renderListingsPage(w http.ResponseWriter, r *http.Request) {
	listings, err := h.store.ListListingsForAdmin(r.Context())
	if err != nil {
		h.serverError(w, "failed to list listings", err)
		return
	}

	data := h.basePageData("Listings", r.URL.Path, r)
	data.Listings = listings

	h.render(w, "listings.html", data)
}

func (h *Handler) renderListingDetailPage(w http.ResponseWriter, r *http.Request, listingID int64) {
	listing, err := h.store.GetListingForAdmin(r.Context(), listingID)
	if err != nil {
		h.serverError(w, "failed to load listing detail", err)
		return
	}

	sources, err := h.store.ListCouponSources(r.Context())
	if err != nil {
		h.serverError(w, "failed to list coupon sources", err)
		return
	}

	coupons, err := h.store.ListCouponsForAdmin(r.Context(), store.CouponListFilter{
		ListingID: &listingID,
		Limit:     250,
	})
	if err != nil {
		h.serverError(w, "failed to list listing coupons", err)
		return
	}

	data := h.basePageData("Listing Detail", r.URL.Path, r)
	data.Listing = &listing
	data.Sources = sources
	data.Coupons = coupons
	data.ImportExample = "masked-display,plain-code,2026-12-31 or plain-code,2026-12-31"
	data.AdminActions, _ = h.store.ListAdminActions(r.Context(), store.AdminActionFilter{
		EntityType: "listing",
		EntityID:   &listingID,
		Limit:      20,
	})

	h.render(w, "listing_detail.html", data)
}

func (h *Handler) renderCouponsPage(w http.ResponseWriter, r *http.Request) {
	listings, err := h.store.ListListingsForAdmin(r.Context())
	if err != nil {
		h.serverError(w, "failed to list listings for coupons page", err)
		return
	}

	var listingID *int64
	var selectedListingID int64
	if raw := strings.TrimSpace(r.URL.Query().Get("listing_id")); raw != "" {
		value := parseInt64Default(raw, 0)
		if value > 0 {
			selectedListingID = value
			listingID = &value
		}
	}

	status := strings.TrimSpace(r.URL.Query().Get("status"))
	coupons, err := h.store.ListCouponsForAdmin(r.Context(), store.CouponListFilter{
		ListingID: listingID,
		Status:    status,
		Limit:     250,
	})
	if err != nil {
		h.serverError(w, "failed to list coupons", err)
		return
	}

	data := h.basePageData("Coupons", r.URL.Path, r)
	data.Listings = listings
	data.Coupons = coupons
	data.SelectedStatus = status
	data.SelectedListingID = selectedListingID
	data.AdminActions, _ = h.store.ListAdminActions(r.Context(), store.AdminActionFilter{
		EntityType: "coupon",
		Limit:      20,
	})

	h.render(w, "coupons.html", data)
}

func (h *Handler) parseListingDetailAction(r *http.Request) (string, error) {
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			return "", errors.New("invalid multipart form")
		}
		return strings.TrimSpace(r.FormValue("action")), nil
	}

	if err := r.ParseForm(); err != nil {
		return "", errors.New("invalid listing form data")
	}

	return strings.TrimSpace(r.PostForm.Get("action")), nil
}

func (h *Handler) updateListingStatus(w http.ResponseWriter, r *http.Request, listingID int64, nextStatus string, actionType string, successMessage string) {
	before, err := h.store.GetListing(r.Context(), listingID)
	if err != nil {
		h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), err.Error())
		return
	}

	listing, err := h.store.UpdateListingStatus(r.Context(), listingID, nextStatus)
	if err != nil {
		h.redirectWithError(w, r, fmt.Sprintf("/admin/listings/%d", listingID), err.Error())
		return
	}

	h.recordAdminAction(r.Context(), h.operatorID(r), "listing", listing.ID, actionType, before, listing, "listing status changed to "+nextStatus)

	h.redirectWithSuccess(w, r, fmt.Sprintf("/admin/listings/%d", listingID), successMessage)
}

func (h *Handler) importCouponsForListing(r *http.Request, listingID int64) error {
	listing, err := h.store.GetListing(r.Context(), listingID)
	if err != nil {
		return err
	}

	sourceID := parseInt64Default(r.FormValue("source_id"), 0)
	if sourceID == 0 {
		return errors.New("source id is required for coupon ingest")
	}

	rows, err := parseCouponImportRequest(r)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errors.New("at least one coupon row is required")
	}

	couponTitle := strings.TrimSpace(r.FormValue("coupon_title"))
	if couponTitle == "" {
		couponTitle = listing.Title
	}

	transferabilityStatus := strings.TrimSpace(r.FormValue("transferability_status"))
	rightsNote := strings.TrimSpace(r.FormValue("rights_verification_note"))
	acquiredCostAmount := parseInt64Default(r.FormValue("acquired_cost_amount"), 0)

	inputs := make([]store.CouponInventoryInput, 0, len(rows))
	for _, row := range rows {
		ciphertext, nonce, err := encryptCouponCode(h.couponEncryptionKey, row.PlainCode)
		if err != nil {
			return fmt.Errorf("encrypt coupon code: %w", err)
		}

		inputs = append(inputs, store.CouponInventoryInput{
			SourceID:               sourceID,
			MerchantName:           listing.MerchantName,
			CouponTitle:            couponTitle,
			CouponValueAmount:      listing.CouponValueAmount,
			SalePriceAmount:        listingEffectivePriceAmount(listing),
			CurrencyCode:           listing.CurrencyCode,
			CouponCodeCiphertext:   ciphertext,
			CouponCodeNonce:        nonce,
			CouponMaskedDisplay:    row.MaskedDisplay,
			ExpiryAt:               row.ExpiryAt,
			TransferabilityStatus:  defaultString(transferabilityStatus, "unknown"),
			RightsVerificationNote: rightsNote,
			AcquiredCostAmount:     acquiredCostAmount,
		})
	}

	_, err = h.store.IngestCoupons(r.Context(), store.IngestCouponsParams{
		ListingID: listingID,
		Coupons:   inputs,
	})
	return err
}
