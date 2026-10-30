package folio

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// newNoCallServer returns a server that fails the test if any request reaches it.
// Placeholder methods must return ErrNotImplemented without calling FOLIO.
func newNoCallServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("placeholder made an HTTP request: %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	return server
}

func assertNotImplemented(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrNotImplemented) {
		t.Errorf("err = %v, want ErrNotImplemented", err)
	}
}

func TestRenewAll_Placeholder(t *testing.T) {
	client := NewCirculationClient(newNoCallServer(t).URL, "test-tenant")
	result, err := client.RenewAll(context.Background(), "test-token", "user123")
	assertNotImplemented(t, err)
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestCreateRequest_Placeholder(t *testing.T) {
	client := NewCirculationClient(newNoCallServer(t).URL, "test-tenant")
	result, err := client.CreateRequest(context.Background(), "test-token", &models.Request{RequestType: "Hold"})
	assertNotImplemented(t, err)
	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestCancelRequest_Placeholder(t *testing.T) {
	client := NewCirculationClient(newNoCallServer(t).URL, "test-tenant")
	err := client.CancelRequest(context.Background(), "test-token", "request-123", "reason-123", "info")
	assertNotImplemented(t, err)
}

func TestUpdateItemStatus_Placeholder(t *testing.T) {
	client := NewInventoryClient(newNoCallServer(t).URL, "test-tenant")
	err := client.UpdateItemStatus(context.Background(), "test-token", "item-123", "Missing")
	assertNotImplemented(t, err)
}

func TestGetUserByUsername_Placeholder(t *testing.T) {
	client := NewPatronClient(newNoCallServer(t).URL, "test-tenant")
	user, err := client.GetUserByUsername(context.Background(), "test-token", "kiosk")
	assertNotImplemented(t, err)
	if user != nil {
		t.Errorf("user = %v, want nil", user)
	}
}

func TestSearchInstances_Placeholder(t *testing.T) {
	client := NewInventoryClient(newNoCallServer(t).URL, "test-tenant")
	instances, err := client.SearchInstances(context.Background(), "test-token", `title all "x"`)
	assertNotImplemented(t, err)
	if instances != nil {
		t.Errorf("instances = %v, want nil", instances)
	}
}

func TestSearchInstancesByTitle_Placeholder(t *testing.T) {
	client := NewInventoryClient(newNoCallServer(t).URL, "test-tenant")
	instances, err := client.SearchInstancesByTitle(context.Background(), "test-token", "Go Programming")
	assertNotImplemented(t, err)
	if instances != nil {
		t.Errorf("instances = %v, want nil", instances)
	}
}

func TestSearchInstancesByISBN_Placeholder(t *testing.T) {
	client := NewInventoryClient(newNoCallServer(t).URL, "test-tenant")
	instances, err := client.SearchInstancesByISBN(context.Background(), "test-token", "9780134190440")
	assertNotImplemented(t, err)
	if instances != nil {
		t.Errorf("instances = %v, want nil", instances)
	}
}
