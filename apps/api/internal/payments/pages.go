package payments

import (
	"fmt"
	"net/http"
)

func ReturnPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Payment received</title>
</head>
<body>
  <h1>Payment received</h1>
  <p>You can return to Telegram. We will deliver your coupon after PayPal confirms the payment.</p>
</body>
</html>`)
}

func CancelPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Payment cancelled</title>
</head>
<body>
  <h1>Payment cancelled</h1>
  <p>No charge was confirmed. Return to Telegram if you want to try the checkout again.</p>
</body>
</html>`)
}
