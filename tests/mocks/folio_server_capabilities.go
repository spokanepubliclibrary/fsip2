package mocks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spokanepubliclibrary/fsip2/internal/folio"
	"github.com/spokanepubliclibrary/fsip2/internal/folio/models"
)

// EnableCapabilityEnforcement turns on Eureka-style capability checks and grants username
// the given capabilities (e.g. folio.RoleCapabilities(folio.RoleSIP2Service)). Call it once
// per user; a later call for the same user replaces that user's set. A user with no grant
// is denied every protected endpoint. /authn/* stays public.
func (m *FolioMockServer) EnableCapabilityEnforcement(username string, capabilities ...[]string) {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	m.enforceCapabilities = true
	if m.userCapabilities == nil {
		m.userCapabilities = make(map[string]map[string]bool)
	}
	set := make(map[string]bool)
	for _, list := range capabilities {
		for _, c := range list {
			set[c] = true
		}
	}
	m.userCapabilities[username] = set
}

// GetCapabilityDenials returns one "METHOD /path -> capability" entry per 403 sent by
// enforcement mode ("-> unmapped" when the endpoint is not in folio.Capabilities).
func (m *FolioMockServer) GetCapabilityDenials() []string {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	return append([]string(nil), m.capabilityDenials...)
}

// authorizeCapability enforces the endpoint's capability when enforcement is on. It writes
// the gateway's 403 and returns false when the token's user lacks it.
func (m *FolioMockServer) authorizeCapability(w http.ResponseWriter, r *http.Request, token string) bool {
	m.authMu.Lock()
	defer m.authMu.Unlock()
	if !m.enforceCapabilities {
		return true
	}

	entry, found := folio.LookupCapability(r.Method, r.URL.Path)
	if found && entry.Usage == folio.UsagePublic {
		return true
	}
	if found && m.userCapabilities[m.usernameForTokenLocked(token)][entry.Capability] {
		return true
	}

	required := "unmapped"
	if found {
		required = entry.Capability
	}
	m.capabilityDenials = append(m.capabilityDenials, fmt.Sprintf("%s %s -> %s", r.Method, r.URL.Path, required))
	// Body observed on Trillium; it names no capability
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{"code": "authorization_error", "message": "Access Denied"})
	return false
}

// usernameForTokenLocked returns the user an access token was issued to. Caller holds authMu.
func (m *FolioMockServer) usernameForTokenLocked(token string) string {
	if entry, ok := m.tokenEntries[token]; ok {
		return entry.Username
	}
	for username, t := range m.Tokens {
		if t == token {
			return username
		}
	}
	return ""
}

// handleUserByID handles GET and PUT /users/{id}. PUT applies expirationDate and active
// (the fields rolling renewal changes) and answers 204, as mod-users does.
func (m *FolioMockServer) handleUserByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/users/")
	var user *models.User
	for _, u := range m.Users {
		if u.ID == id {
			user = u
			break
		}
	}
	if user == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(user)
	case http.MethodPut:
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if s, ok := body["expirationDate"].(string); ok {
			for _, layout := range []string{"2006-01-02T15:04:05.000-07:00", time.RFC3339Nano} {
				if t, err := time.Parse(layout, s); err == nil {
					user.ExpirationDate = &t
					break
				}
			}
		}
		if active, ok := body["active"].(bool); ok {
			user.Active = active
		}
		m.UserPutCount++
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleRequests handles GET /circulation/requests?query=... matching by requesterId or
// itemId. A request is returned when its status appears quoted in the query, which covers
// fsip2's status clauses. limit=0 returns only totalRecords, as FOLIO does.
func (m *FolioMockServer) handleRequests(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	matches := []models.Request{}
	for _, req := range m.Requests {
		owner := strings.Contains(query, fmt.Sprintf(`requesterId=="%s"`, req.RequesterID)) ||
			strings.Contains(query, fmt.Sprintf("itemId==%s", req.ItemID))
		if owner && strings.Contains(query, `"`+req.Status+`"`) {
			matches = append(matches, *req)
		}
	}

	total := len(matches)
	if r.URL.Query().Get("limit") == "0" {
		matches = []models.Request{}
	}
	json.NewEncoder(w).Encode(models.RequestCollection{Requests: matches, TotalRecords: total})
}

// serveByID answers GET /<resource>/{id} from an id-keyed map, or 404.
func serveByID[T any](w http.ResponseWriter, r *http.Request, records map[string]*T) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.Method != http.MethodGet || len(parts) != 2 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	if record, ok := records[parts[1]]; ok {
		json.NewEncoder(w).Encode(record)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
}

// AddRequest adds a request keyed by request.ID.
func (m *FolioMockServer) AddRequest(request *models.Request) {
	m.Requests[request.ID] = request
}

// AddGroup adds a patron group keyed by group.ID.
func (m *FolioMockServer) AddGroup(group *models.PatronGroup) {
	m.Groups[group.ID] = group
}

// AddLocation adds a location keyed by location.ID.
func (m *FolioMockServer) AddLocation(location *models.Location) {
	m.Locations[location.ID] = location
}

// AddMaterialType adds a material type keyed by materialType.ID.
func (m *FolioMockServer) AddMaterialType(materialType *models.MaterialType) {
	m.MaterialTypes[materialType.ID] = materialType
}

// AddServicePoint adds a service point keyed by servicePoint.ID.
func (m *FolioMockServer) AddServicePoint(servicePoint *models.ServicePoint) {
	m.ServicePoints[servicePoint.ID] = servicePoint
}
