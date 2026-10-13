package sessions

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestHistoryFilterValidationAndCalendarDays(t *testing.T) {
	for _, query := range []string{"exercise=0", "exercise=9223372036854775808", "exercise=x", "from=2030-02-30", "from=0000-01-01", "to=2030-1-1", "from=2030-02-02&to=2030-02-01", "tz=Missing/Zone", "tz=Local", "q=a&q=b", "unknown=value", "from=%ZZ", "q=a;b"} {
		if _, err := ParseHistoryQuery(query); err == nil {
			t.Fatal("invalid filter accepted", query)
		}
		for _, path := range []string{"/api/sets?", "/api/history/sessions?"} {
			mux := http.NewServeMux()
			store := &fakeStore{}
			Register(mux, store)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest("GET", path+query, nil))
			if response.Code != 400 || response.Header().Get("Cache-Control") != "no-store" || store.owner != 0 {
				t.Fatal("invalid query reached store", query, response.Code)
			}
		}
	}
	for _, tc := range []struct {
		day   string
		hours time.Duration
	}{{"2026-03-29", 23}, {"2026-10-25", 25}} {
		f, err := ParseHistoryFilter(url.Values{"from": {tc.day}, "to": {tc.day}, "tz": {"Europe/Copenhagen"}, "q": {"  PrEsS  "}, "exercise": {"7"}})
		if err != nil || f.Query != "PrEsS" || f.Exercise != "7" || f.From == nil || f.Until == nil || f.Until.Sub(*f.From) != tc.hours*time.Hour {
			t.Fatal("calendar/DST boundary", f, err)
		}
	}
}
