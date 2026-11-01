package templates

import (
	"github.com/agersk0v/crow-fitness/backend/internal/auth"
	"io"
	"net/http"
	"net/http/httptest"
)

func ownedRequest(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	return r.WithContext(auth.WithPrincipal(r.Context(), auth.Account{ID: 1, OwnerID: 1}))
}
