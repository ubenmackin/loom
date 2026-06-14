package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mockMaxTotalSetter records the last value passed to SetMaxTotal.
type mockMaxTotalSetter struct {
	mu  sync.Mutex
	val int
}

func (m *mockMaxTotalSetter) SetMaxTotal(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.val = n
}

func (m *mockMaxTotalSetter) Value() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.val
}

func TestSettingsGet_Default(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	rr := doRequest(t, mux, http.MethodGet, "/api/settings/global_max_concurrency", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/settings/global_max_concurrency status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]int
	decodeRespJSON(t, rr, &resp)

	if resp["value"] != 0 {
		t.Errorf("value = %d, want 0 (default when not set)", resp["value"])
	}
}

func TestSettingsGet_WithValue(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	// Insert a known value directly into the settings table.
	_, err := dbConn.Exec("INSERT INTO settings (key, value) VALUES (?, ?)", "global_max_concurrency", "7")
	if err != nil {
		t.Fatalf("insert test setting: %v", err)
	}

	rr := doRequest(t, mux, http.MethodGet, "/api/settings/global_max_concurrency", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/settings/global_max_concurrency status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]int
	decodeRespJSON(t, rr, &resp)

	if resp["value"] != 7 {
		t.Errorf("value = %d, want 7", resp["value"])
	}
}

func TestSettingsPut_Valid(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	body := map[string]any{"value": 5}
	rr := doRequest(t, mux, http.MethodPut, "/api/settings/global_max_concurrency", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/settings/global_max_concurrency status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]int
	decodeRespJSON(t, rr, &resp)

	if resp["value"] != 5 {
		t.Errorf("value = %d, want 5", resp["value"])
	}

	// Verify the value persisted by re-reading via GET.
	rr = doRequest(t, mux, http.MethodGet, "/api/settings/global_max_concurrency", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET after PUT status = %d, want %d", rr.Code, http.StatusOK)
	}

	var getResp map[string]int
	decodeRespJSON(t, rr, &getResp)

	if getResp["value"] != 5 {
		t.Errorf("GET after PUT value = %d, want 5", getResp["value"])
	}
}

func TestSettingsPut_Negative(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	body := map[string]any{"value": -1}
	rr := doRequest(t, mux, http.MethodPut, "/api/settings/global_max_concurrency", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT with negative value status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	// Verify the error message mentions the constraint.
	var errResp map[string]string
	decodeRespJSON(t, rr, &errResp)

	if !strings.Contains(errResp["error"], ">= 0") {
		t.Errorf("error message = %q, want it to contain '>= 0'", errResp["error"])
	}
}

func TestSettingsPut_TooLarge(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	body := map[string]any{"value": 10001}
	rr := doRequest(t, mux, http.MethodPut, "/api/settings/global_max_concurrency", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT with too-large value status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp map[string]string
	decodeRespJSON(t, rr, &errResp)

	if !strings.Contains(errResp["error"], "<= 10000") {
		t.Errorf("error message = %q, want it to contain '<= 10000'", errResp["error"])
	}
}

func TestSettingsPut_InvalidBody(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	// Send malformed JSON.
	rr := doRawRequest(t, mux, http.MethodPut, "/api/settings/global_max_concurrency", `{"bad`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("PUT with invalid body status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var errResp map[string]string
	decodeRespJSON(t, rr, &errResp)

	if !strings.Contains(errResp["error"], "invalid request body") {
		t.Errorf("error message = %q, want it to contain 'invalid request body'", errResp["error"])
	}
}

// doRawRequest sends an HTTP request with a raw string body (bypassing JSON
// marshalling) so we can test malformed payloads.
func doRawRequest(t *testing.T, mux chi.Router, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := lookupAuthToken(t.Name()); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}
