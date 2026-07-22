package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type memStore struct {
	auth     map[string]any
	cfg      map[string]any
	settings map[string]any
}

func (m *memStore) PublicSettings(ctx context.Context) (map[string]any, error) {
	return map[string]any{"sub2api_config": redactIntegrationConfig("sub2api_config", m.cfg)}, nil
}
func (m *memStore) SetSetting(ctx context.Context, key string, value any) error {
	if key == "sub2api_config" {
		m.cfg, _ = value.(map[string]any)
		return nil
	}
	if m.settings == nil {
		m.settings = map[string]any{}
	}
	m.settings[key] = value
	return nil
}
func (m *memStore) GetSetting(ctx context.Context, key string) (any, error) {
	if key == "sub2api_config" {
		return m.cfg, nil
	}
	if m.settings != nil {
		return m.settings[key], nil
	}
	return nil, nil
}
func (m *memStore) ExportAuthMap(ctx context.Context, accountIDs []string, includeSecrets bool) (map[string]any, error) {
	return map[string]any{"ok": true, "auth": m.auth, "count": len(m.auth)}, nil
}

func TestExportSub2APIFormatDataPayload(t *testing.T) {
	st := &memStore{
		cfg: map[string]any{"notes_prefix": "g2a", "account_concurrency": 3},
		auth: map[string]any{
			"acc1": map[string]any{
				"email":         "a@x.com",
				"access_token":  "tok",
				"refresh_token": "rt",
				"expires_at":    float64(1700000000),
				"sso":           "cookie",
			},
			"acc2": map[string]any{"email": "b@x.com"}, // skip no token
		},
	}
	out, err := ExportSub2APIFormat(context.Background(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["type"] != "sub2api-data" {
		t.Fatalf("type=%v", out["type"])
	}
	if _, ok := out["proxies"].([]any); !ok {
		// may be typed empty slice
		if out["proxies"] == nil {
			t.Fatal("proxies missing")
		}
	}
	accs, _ := out["accounts"].([]map[string]any)
	if accs == nil {
		if arr, ok := out["accounts"].([]any); ok {
			for _, a := range arr {
				if m, ok := a.(map[string]any); ok {
					accs = append(accs, m)
				}
			}
		}
	}
	if len(accs) != 1 {
		t.Fatalf("accounts=%d out=%#v", len(accs), out["accounts"])
	}
	creds, _ := accs[0]["credentials"].(map[string]any)
	if creds["access_token"] != "tok" {
		t.Fatalf("creds=%#v", creds)
	}
}

func TestPublicConfigRedactsPassword(t *testing.T) {
	st := &memStore{cfg: map[string]any{"base_url": "http://x", "password": "secret", "email": "e"}}
	out := PublicConfig(context.Background(), st, "sub2api_config")
	if out["password"] != nil && out["password"] != "" {
		t.Fatalf("password leaked: %#v", out)
	}
	if out["has_password"] != true {
		t.Fatalf("has_password missing: %#v", out)
	}
}

func TestPushSub2APICreatesGroupAndMatchesProxy(t *testing.T) {
	var createdGroup bool
	var accountBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"jwt"}}`))
		case r.URL.Path == "/api/v1/admin/groups" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
		case r.URL.Path == "/api/v1/admin/groups" && r.Method == http.MethodPost:
			createdGroup = true
			_, _ = w.Write([]byte(`{"code":0,"data":{"id":12,"name":"grokcli-2api"}}`))
		case r.URL.Path == "/api/v1/admin/proxies":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":7,"protocol":"http","host":"192.0.2.10","port":7890}]}`))
		case r.URL.Path == "/api/v1/admin/accounts" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[]}}`))
		case r.URL.Path == "/api/v1/admin/accounts" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&accountBody); err != nil {
				t.Errorf("decode account body: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"id":99,"name":"user@example.com"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := &memStore{
		cfg: map[string]any{
			"base_url":          server.URL,
			"email":             "admin@example.com",
			"password":          "secret",
			"auto_create_group": true,
		},
		settings: map[string]any{
			"registration_config": map[string]any{"proxy": "http://192.0.2.10:7890"},
		},
		auth: map[string]any{
			"acc-1": map[string]any{"email": "user@example.com", "access_token": "token"},
		},
	}

	result, err := PushSub2API(context.Background(), store, []string{"acc-1"}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true {
		t.Fatalf("result=%#v", result)
	}
	if !createdGroup {
		t.Fatal("missing default group was not created")
	}
	if intField(accountBody, "proxy_id", 0) != 7 {
		t.Fatalf("proxy_id=%v body=%#v", accountBody["proxy_id"], accountBody)
	}
	groups, _ := accountBody["group_ids"].([]any)
	if len(groups) != 1 || int(groups[0].(float64)) != 12 {
		t.Fatalf("group_ids=%#v", accountBody["group_ids"])
	}
}

func TestPushSub2APIUpdatesFreshestAccountAndDeactivatesDuplicate(t *testing.T) {
	var created bool
	var canonicalBody map[string]any
	var duplicateBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/auth/login":
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"jwt"}}`))
		case r.URL.Path == "/api/v1/admin/accounts" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":20,"name":"user@example.com","platform":"grok","status":"active","credentials":{"expires_at":"2026-07-22T20:00:00Z"}},{"id":10,"name":"user@example.com","platform":"grok","status":"active","credentials":{"expires_at":"2026-07-22T18:00:00Z"}}]}}`))
		case r.URL.Path == "/api/v1/admin/accounts" && r.Method == http.MethodPost:
			created = true
			http.Error(w, "unexpected create", http.StatusInternalServerError)
		case r.URL.Path == "/api/v1/admin/accounts/20" && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&canonicalBody); err != nil {
				t.Errorf("decode canonical body: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"id":20,"name":"user@example.com"}}`))
		case r.URL.Path == "/api/v1/admin/accounts/10" && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&duplicateBody); err != nil {
				t.Errorf("decode duplicate body: %v", err)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"id":10,"name":"user@example.com"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := &memStore{
		cfg: map[string]any{
			"base_url": server.URL,
			"email":    "admin@example.com",
			"password": "secret",
			"group_id": 12,
			"proxy_id": 7,
		},
		auth: map[string]any{
			"acc-1": map[string]any{
				"email": "user@example.com", "access_token": "older-token",
				"refresh_token": "older-refresh", "expires_at": "2026-07-22T19:00:00Z",
			},
		},
	}

	result, err := PushSub2API(context.Background(), store, []string{"acc-1"}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true || created {
		t.Fatalf("result=%#v created=%v", result, created)
	}
	if _, exists := canonicalBody["credentials"]; exists {
		t.Fatalf("older credentials replaced fresher remote credentials: %#v", canonicalBody)
	}
	if duplicateBody["status"] != "inactive" {
		t.Fatalf("duplicate body=%#v", duplicateBody)
	}
	rows, _ := result["results"].([]map[string]any)
	if len(rows) != 1 || rows[0]["action"] != "updated" || intField(rows[0], "deduplicated", 0) != 1 {
		t.Fatalf("results=%#v", result["results"])
	}
}
