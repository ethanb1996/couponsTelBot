package payments

import "testing"

func TestApprovalURLFromPayPalLinksPrefersApprove(t *testing.T) {
	links := []struct {
		Href   string `json:"href"`
		Rel    string `json:"rel"`
		Method string `json:"method"`
	}{
		{Href: "https://example.com/self", Rel: "self", Method: "GET"},
		{Href: "https://example.com/approve", Rel: "approve", Method: "GET"},
		{Href: "https://example.com/payer-action", Rel: "payer-action", Method: "GET"},
	}

	got := approvalURLFromPayPalLinks(links)
	if got != "https://example.com/approve" {
		t.Fatalf("expected approve URL, got %q", got)
	}
}

func TestApprovalURLFromPayPalLinksFallsBackToPayerAction(t *testing.T) {
	links := []struct {
		Href   string `json:"href"`
		Rel    string `json:"rel"`
		Method string `json:"method"`
	}{
		{Href: "https://example.com/self", Rel: "self", Method: "GET"},
		{Href: "https://example.com/payer-action", Rel: "payer-action", Method: "GET"},
	}

	got := approvalURLFromPayPalLinks(links)
	if got != "https://example.com/payer-action" {
		t.Fatalf("expected payer-action URL, got %q", got)
	}
}

func TestApprovalURLFromPayPalLinksFallsBackToGetLink(t *testing.T) {
	links := []struct {
		Href   string `json:"href"`
		Rel    string `json:"rel"`
		Method string `json:"method"`
	}{
		{Href: "https://example.com/redirect", Rel: "redirect", Method: "GET"},
	}

	got := approvalURLFromPayPalLinks(links)
	if got != "https://example.com/redirect" {
		t.Fatalf("expected GET fallback URL, got %q", got)
	}
}
