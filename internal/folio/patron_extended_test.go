package folio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// Helper: create a minimal mock server with a single handler
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestGetUserByBarcode_Success(t *testing.T) {
	testUser := models.User{
		ID:       "user-001",
		Username: "jdoe",
		Barcode:  "123456",
		Active:   true,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		query := r.URL.Query().Get("query")
		if strings.Contains(query, "barcode==") {
			resp := models.UserCollection{
				Users:        []models.User{testUser},
				TotalRecords: 1,
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	user, err := client.GetUserByBarcode(context.Background(), "token", "123456")
	if err != nil {
		t.Fatalf("GetUserByBarcode failed: %v", err)
	}
	if user.ID != "user-001" {
		t.Errorf("Expected user-001, got %s", user.ID)
	}
}

func TestGetUserByBarcode_NotFound(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := models.UserCollection{Users: []models.User{}, TotalRecords: 0}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	_, err := client.GetUserByBarcode(context.Background(), "token", "nonexistent")
	if err == nil {
		t.Error("Expected error for not found user")
	}
}

func TestGetUserByID_Success(t *testing.T) {
	testUser := models.User{ID: "user-999", Username: "testuser", Active: true}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/users/") {
			json.NewEncoder(w).Encode(testUser)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	user, err := client.GetUserByID(context.Background(), "token", "user-999")
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if user.ID != "user-999" {
		t.Errorf("Expected user-999, got %s", user.ID)
	}
}

func TestGetUserByUsername_Success(t *testing.T) {
	testUser := models.User{ID: "user-abc", Username: "janedoe", Active: true}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		query := r.URL.Query().Get("query")
		if strings.Contains(query, "username==") {
			resp := models.UserCollection{Users: []models.User{testUser}, TotalRecords: 1}
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	user, err := client.GetUserByUsername(context.Background(), "token", "janedoe")
	if err != nil {
		t.Fatalf("GetUserByUsername failed: %v", err)
	}
	if user.Username != "janedoe" {
		t.Errorf("Expected janedoe, got %s", user.Username)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := models.UserCollection{Users: []models.User{}, TotalRecords: 0}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	_, err := client.GetUserByUsername(context.Background(), "token", "ghost")
	if err == nil {
		t.Error("Expected error for not found username")
	}
}

func TestGetManualBlocks_Success(t *testing.T) {
	blocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Borrowing: true, Desc: "Overdue fines", PatronMessage: "Please pay your fines"},
		},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blocks)
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	result, err := client.GetManualBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetManualBlocks failed: %v", err)
	}
	if len(result.ManualBlocks) != 1 {
		t.Errorf("Expected 1 block, got %d", len(result.ManualBlocks))
	}
}

func TestGetAutomatedPatronBlocks_Success(t *testing.T) {
	blocks := models.AutomatedPatronBlock{
		AutomatedPatronBlocks: []models.AutomatedBlock{
			{BlockBorrowing: true, Message: "Account balance exceeded"},
		},
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blocks)
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	result, err := client.GetAutomatedPatronBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetAutomatedPatronBlocks failed: %v", err)
	}
	if len(result.AutomatedPatronBlocks) != 1 {
		t.Errorf("Expected 1 block, got %d", len(result.AutomatedPatronBlocks))
	}
}

func TestGetAutomatedPatronBlocks_404ReturnsEmpty(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	result, err := client.GetAutomatedPatronBlocks(context.Background(), "token", "user-noblocks")
	if err != nil {
		t.Fatalf("GetAutomatedPatronBlocks should not error on 404: %v", err)
	}
	if len(result.AutomatedPatronBlocks) != 0 {
		t.Errorf("Expected 0 blocks for 404 response, got %d", len(result.AutomatedPatronBlocks))
	}
}

func TestHasBlocks_WithBlocks(t *testing.T) {
	manualBlocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{{Borrowing: true, Desc: "Test block"}},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(manualBlocks)
		} else {
			json.NewEncoder(w).Encode(models.AutomatedPatronBlock{})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	hasBlocks, err := client.HasBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("HasBlocks failed: %v", err)
	}
	if !hasBlocks {
		t.Error("Expected HasBlocks to return true")
	}
}

func TestHasBlocks_NoBlocks(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(models.ManualBlockCollection{ManualBlocks: []models.ManualBlock{}, TotalRecords: 0})
		} else {
			// Automated blocks - 404 = no blocks
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	hasBlocks, err := client.HasBlocks(context.Background(), "token", "user-clean")
	if err != nil {
		t.Fatalf("HasBlocks failed: %v", err)
	}
	if hasBlocks {
		t.Error("Expected HasBlocks to return false")
	}
}

func TestGetBorrowingBlocks_WithBlocks(t *testing.T) {
	manualBlocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Borrowing: true, PatronMessage: "Cannot borrow"},
		},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(manualBlocks)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetBorrowingBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetBorrowingBlocks failed: %v", err)
	}
	if !blocked {
		t.Error("Expected borrowing to be blocked")
	}
	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

func TestGetBorrowingBlocks_WithDescFallback(t *testing.T) {
	// Block with no PatronMessage but has Desc - should use Desc
	manualBlocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Borrowing: true, Desc: "Staff description", PatronMessage: ""},
		},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(manualBlocks)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetBorrowingBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetBorrowingBlocks failed: %v", err)
	}
	if !blocked {
		t.Error("Expected borrowing to be blocked")
	}
	if len(messages) == 0 || messages[0] != "Staff description" {
		t.Errorf("Expected 'Staff description', got %v", messages)
	}
}

func TestGetBorrowingBlocks_WithAutomatedBlocks(t *testing.T) {
	automatedBlocks := models.AutomatedPatronBlock{
		AutomatedPatronBlocks: []models.AutomatedBlock{
			{BlockBorrowing: true, Message: "Max items checked out"},
		},
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(models.ManualBlockCollection{ManualBlocks: []models.ManualBlock{}, TotalRecords: 0})
		} else {
			json.NewEncoder(w).Encode(automatedBlocks)
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetBorrowingBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetBorrowingBlocks failed: %v", err)
	}
	if !blocked {
		t.Error("Expected borrowing to be blocked by automated blocks")
	}
	if len(messages) == 0 {
		t.Error("Expected at least one message from automated blocks")
	}
}

func TestGetRenewalsBlocks_WithBlocks(t *testing.T) {
	manualBlocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Renewals: true, PatronMessage: "Cannot renew"},
		},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(manualBlocks)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetRenewalsBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetRenewalsBlocks failed: %v", err)
	}
	if !blocked {
		t.Error("Expected renewals to be blocked")
	}
	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

func TestGetRenewalsBlocks_WithDescFallback(t *testing.T) {
	manualBlocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Renewals: true, Desc: "Renewal block desc", PatronMessage: ""},
		},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(manualBlocks)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetRenewalsBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetRenewalsBlocks failed: %v", err)
	}
	if !blocked || len(messages) == 0 || messages[0] != "Renewal block desc" {
		t.Errorf("Unexpected result: blocked=%v messages=%v", blocked, messages)
	}
}

func TestGetRenewalsBlocks_WithAutomatedBlocks(t *testing.T) {
	automatedBlocks := models.AutomatedPatronBlock{
		AutomatedPatronBlocks: []models.AutomatedBlock{
			{BlockRenewals: true, Message: "Automated renewal block"},
		},
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(models.ManualBlockCollection{ManualBlocks: []models.ManualBlock{}, TotalRecords: 0})
		} else {
			json.NewEncoder(w).Encode(automatedBlocks)
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetRenewalsBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetRenewalsBlocks failed: %v", err)
	}
	if !blocked || len(messages) == 0 {
		t.Errorf("Expected renewal block from automated blocks: blocked=%v messages=%v", blocked, messages)
	}
}

func TestGetRequestsBlocks_WithBlocks(t *testing.T) {
	manualBlocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Requests: true, PatronMessage: "Cannot place requests"},
		},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(manualBlocks)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetRequestsBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetRequestsBlocks failed: %v", err)
	}
	if !blocked {
		t.Error("Expected requests to be blocked")
	}
	if len(messages) == 0 {
		t.Error("Expected at least one message")
	}
}

func TestGetRequestsBlocks_WithDescFallback(t *testing.T) {
	manualBlocks := models.ManualBlockCollection{
		ManualBlocks: []models.ManualBlock{
			{Requests: true, Desc: "Requests block desc", PatronMessage: ""},
		},
		TotalRecords: 1,
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(manualBlocks)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetRequestsBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetRequestsBlocks failed: %v", err)
	}
	if !blocked || len(messages) == 0 || messages[0] != "Requests block desc" {
		t.Errorf("Unexpected result: blocked=%v messages=%v", blocked, messages)
	}
}

func TestGetRequestsBlocks_WithAutomatedBlocks(t *testing.T) {
	automatedBlocks := models.AutomatedPatronBlock{
		AutomatedPatronBlocks: []models.AutomatedBlock{
			{BlockRequests: true, Message: "Automated request block"},
		},
	}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "manualblocks") {
			json.NewEncoder(w).Encode(models.ManualBlockCollection{ManualBlocks: []models.ManualBlock{}, TotalRecords: 0})
		} else {
			json.NewEncoder(w).Encode(automatedBlocks)
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	blocked, messages, err := client.GetRequestsBlocks(context.Background(), "token", "user-001")
	if err != nil {
		t.Fatalf("GetRequestsBlocks failed: %v", err)
	}
	if !blocked || len(messages) == 0 {
		t.Errorf("Expected request block from automated blocks: blocked=%v messages=%v", blocked, messages)
	}
}

func TestUpdateUser_Success(t *testing.T) {
	testUser := &models.User{
		ID:       "user-update",
		Username: "updateduser",
		Active:   true,
	}

	putCalled := false
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/users/") {
			putCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	err := client.UpdateUser(context.Background(), "token", testUser)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	if !putCalled {
		t.Error("Expected PUT request to be made")
	}
}

func TestVerifyPatronPasswordWithLogin_Success(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/authn/login-with-expiry" && r.Method == http.MethodPost {
			resp := models.LoginResponse{AccessToken: "valid-token", ExpiresIn: 3600}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	valid, err := client.VerifyPatronPasswordWithLogin(context.Background(), "jdoe", "secret")
	if err != nil {
		t.Fatalf("VerifyPatronPasswordWithLogin failed: %v", err)
	}
	if !valid {
		t.Error("Expected password verification to succeed")
	}
}

func TestVerifyPatronPasswordWithLogin_InvalidCredentials(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	valid, err := client.VerifyPatronPasswordWithLogin(context.Background(), "jdoe", "wrongpass")
	if err != nil {
		t.Fatalf("VerifyPatronPasswordWithLogin should not error on invalid credentials: %v", err)
	}
	if valid {
		t.Error("Expected password verification to fail")
	}
}

func TestVerifyPatronPasswordWithLogin_422(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(422)
		json.NewEncoder(w).Encode(map[string]string{"error": "unprocessable"})
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	valid, err := client.VerifyPatronPasswordWithLogin(context.Background(), "jdoe", "wrongpass")
	if err != nil {
		t.Fatalf("VerifyPatronPasswordWithLogin should not error on 422: %v", err)
	}
	if valid {
		t.Error("Expected password verification to fail on 422")
	}
}

func TestVerifyPatronPin_Success(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/patron-pin/verify" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	valid, err := client.VerifyPatronPin(context.Background(), "token", "user-001", "1234")
	if err != nil {
		t.Fatalf("VerifyPatronPin failed: %v", err)
	}
	if !valid {
		t.Error("Expected PIN verification to succeed")
	}
}

func TestVerifyPatronPin_InvalidPin(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte("invalid pin"))
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	valid, err := client.VerifyPatronPin(context.Background(), "token", "user-001", "9999")
	if err != nil {
		t.Fatalf("VerifyPatronPin should not error on invalid PIN: %v", err)
	}
	if valid {
		t.Error("Expected PIN verification to fail")
	}
}

func TestGetPatronGroupByID_Success(t *testing.T) {
	testGroup := models.PatronGroup{ID: "group-001", Group: "Faculty"}

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/groups/") {
			json.NewEncoder(w).Encode(testGroup)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	client := NewPatronClient(server.URL, "test-tenant")
	group, err := client.GetPatronGroupByID(context.Background(), "token", "group-001")
	if err != nil {
		t.Fatalf("GetPatronGroupByID failed: %v", err)
	}
	if group.ID != "group-001" {
		t.Errorf("Expected group-001, got %s", group.ID)
	}
}
