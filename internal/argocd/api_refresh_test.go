package argocd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// memoryStore is a TokenStore held in memory.
type memoryStore struct {
	mu           sync.Mutex
	token        string
	refreshToken string
	saves        int
}

func (m *memoryStore) Load() (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token, m.refreshToken, nil
}

func (m *memoryStore) Save(token, refreshToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token, m.refreshToken = token, refreshToken
	m.saves++
	return nil
}

// ssoServer is an Argo CD server that accepts only one token, with an IdP
// under /idp that swaps a refresh token for a new one.
type ssoServer struct {
	*httptest.Server
	mu sync.Mutex
	// valid is the token Argo CD accepts.
	valid string
	// settings is the body of /api/v1/settings; "{url}" becomes the server URL.
	settings string
	// refreshToken is the one the IdP accepts; issued and rotated are what it returns.
	refreshToken, issued, rotated string
	// clientID is the client the IdP expects.
	clientID string
	// redemptions counts calls to the token endpoint.
	redemptions atomic.Int32
	// bodies records the body of each POST to Argo CD.
	bodies []string
	// supportedScopes is the scopes_supported of the discovery document.
	supportedScopes []string
	// scopes records the scope form value of each token request.
	scopes []string
}

func newSSOServer(t *testing.T) *ssoServer {
	t.Helper()
	server := &ssoServer{
		valid:        "fresh-id-token",
		settings:     `{"url":"{url}","oidcConfig":{"name":"Okta","issuer":"{url}/idp","clientID":"argo-cd","cliClientID":"argo-cd-cli-okta"}}`,
		refreshToken: "refresh-1",
		issued:       "fresh-id-token",
		rotated:      "refresh-2",
		clientID:     "argo-cd-cli-okta",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/settings", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.ReplaceAll(server.settings, "{url}", server.URL))
	})
	discovery := func(w http.ResponseWriter, r *http.Request) {
		issuer := server.URL + strings.TrimSuffix(r.URL.Path, "/.well-known/openid-configuration")
		response := map[string]any{"issuer": issuer, "token_endpoint": issuer + "/token"}
		if len(server.supportedScopes) > 0 {
			response["scopes_supported"] = server.supportedScopes
		}
		_ = json.NewEncoder(w).Encode(response)
	}
	mux.HandleFunc("/idp/.well-known/openid-configuration", discovery)
	mux.HandleFunc("/api/dex/.well-known/openid-configuration", discovery)
	token := func(w http.ResponseWriter, r *http.Request) {
		server.redemptions.Add(1)
		server.mu.Lock()
		server.scopes = append(server.scopes, r.FormValue("scope"))
		server.mu.Unlock()
		if r.Method != http.MethodPost || r.FormValue("grant_type") != "refresh_token" ||
			r.FormValue("refresh_token") != server.refreshToken || r.FormValue("client_id") != server.clientID {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid_grant","error_description":"refresh token is invalid or expired"}`)
			return
		}
		response := map[string]string{"access_token": "opaque", "token_type": "Bearer", "id_token": server.issued}
		if server.rotated != "" {
			response["refresh_token"] = server.rotated
		}
		_ = json.NewEncoder(w).Encode(response)
	}
	mux.HandleFunc("/idp/token", token)
	mux.HandleFunc("/api/dex/token", token)
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		server.mu.Lock()
		valid := server.valid
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			server.bodies = append(server.bodies, string(body))
		}
		server.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+valid {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"token is expired","code":16,"message":"token is expired"}`)
			return
		}
		_, _ = io.WriteString(w, `{"items":[]}`)
	})
	server.Server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func ssoSource(t *testing.T, url string, store *memoryStore) *APISource {
	t.Helper()
	source, err := NewConfiguredAPISource(url, store.token, APIOptions{TokenFromCLI: true, TokenStore: store})
	if err != nil {
		t.Fatal(err)
	}
	return source
}

// sentScopes returns the scope form value of each token request.
func (s *ssoServer) sentScopes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.scopes)
}

func TestAnExpiredSSOTokenIsRenewedWithTheRefreshTokenAndSaved(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}
	source := ssoSource(t, server.URL, store)

	if _, err := source.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v, want the token renewed and the request retried", err)
	}

	if store.token != "fresh-id-token" || store.refreshToken != "refresh-2" {
		t.Fatalf("saved %q / %q, want the new ID token and the rotated refresh token", store.token, store.refreshToken)
	}
	if got := server.redemptions.Load(); got != 1 {
		t.Fatalf("token endpoint called %d times, want once", got)
	}
	if scopes := server.sentScopes(); len(scopes) != 1 || scopes[0] != "openid profile email groups" {
		t.Fatalf("scope sent = %q, want the default scopes", scopes)
	}
}

func TestDexLoginsRenewThroughArgoCDsOwnDex(t *testing.T) {
	server := newSSOServer(t)
	server.settings = `{"url":"{url}","dexConfig":{"connectors":[{"name":"GitHub","type":"github"}]}}`
	server.clientID = "argo-cd-cli"
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}

	if _, err := ssoSource(t, server.URL, store).Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v, want the token renewed through /api/dex", err)
	}
	if store.token != "fresh-id-token" {
		t.Fatalf("saved token %q, want the one Dex issued", store.token)
	}
	if scopes := server.sentScopes(); len(scopes) != 1 || scopes[0] != "openid profile email groups federated:id" {
		t.Fatalf("scope sent = %q, want Dex's scopes", scopes)
	}
}

func TestTheConfiguredScopesAreSentOnRenewal(t *testing.T) {
	server := newSSOServer(t)
	server.settings = `{"url":"{url}","oidcConfig":{"issuer":"{url}/idp","clientID":"argo-cd","scopes":["openid","groups"]}}`
	server.clientID = "argo-cd"
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}

	if _, err := ssoSource(t, server.URL, store).Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if scopes := server.sentScopes(); len(scopes) != 1 || scopes[0] != "openid groups" {
		t.Fatalf("scope sent = %q, want the configured scopes", scopes)
	}
}

func TestOfflineAccessIsAskedForWhenTheProviderSupportsIt(t *testing.T) {
	server := newSSOServer(t)
	server.supportedScopes = []string{"openid", "profile", "email", "groups", "offline_access"}
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}

	if _, err := ssoSource(t, server.URL, store).Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if scopes := server.sentScopes(); len(scopes) != 1 || scopes[0] != "openid profile email groups offline_access" {
		t.Fatalf("scope sent = %q, want offline_access added", scopes)
	}
}

func TestTheRefreshTokenIsKeptWhenTheIdPDoesNotRotateIt(t *testing.T) {
	server := newSSOServer(t)
	server.rotated = ""
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}

	if _, err := ssoSource(t, server.URL, store).Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.refreshToken != "refresh-1" {
		t.Fatalf("saved refresh token %q, want the old one kept", store.refreshToken)
	}
}

func TestATokenTheCLIRenewedMeanwhileIsUsedWithoutAskingTheIdP(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}
	source := ssoSource(t, server.URL, store)
	// argocd itself renewed the login after argoxd started.
	store.token, store.refreshToken = "fresh-id-token", "refresh-2"

	if _, err := source.Load(context.Background()); err != nil {
		t.Fatalf("Load() error = %v, want the token from the store", err)
	}
	if got := server.redemptions.Load(); got != 0 {
		t.Fatalf("token endpoint called %d times, want none", got)
	}
}

func TestWithoutARefreshTokenAnExpiredLoginStillSaysToLogIn(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token"}

	_, err := ssoSource(t, server.URL, store).Load(context.Background())

	if err == nil || !strings.HasPrefix(err.Error(), "load applications: token expired? run: argocd login") {
		t.Fatalf("Load() error = %v, want the argocd login advice first", err)
	}
	if got := server.redemptions.Load(); got != 0 {
		t.Fatalf("token endpoint called %d times, want none", got)
	}
}

func TestAFailedRenewalSaysWhyAndIsNotRepeatedOnEveryRequest(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token", refreshToken: "revoked"}
	source := ssoSource(t, server.URL, store)

	_, err := source.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "argocd login") || !strings.Contains(err.Error(), "refresh token is invalid or expired") {
		t.Fatalf("Load() error = %v, want the login advice and the IdP's reason", err)
	}
	if advice, reason := strings.Index(err.Error(), "argocd login"), strings.Index(err.Error(), "refresh token is invalid"); advice > reason {
		t.Fatalf("error = %q, want the advice before the reason", err)
	}
	_, err = source.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refresh token is invalid or expired") {
		t.Fatalf("second Load() error = %v, want the reason kept", err)
	}
	if got := server.redemptions.Load(); got != 1 {
		t.Fatalf("token endpoint called %d times over two loads, want once", got)
	}
	if store.saves != 0 {
		t.Fatalf("store saved %d times, want nothing saved after a failure", store.saves)
	}
}

func TestANewLoginClearsAFailedRenewal(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token", refreshToken: "revoked"}
	source := ssoSource(t, server.URL, store)
	if _, err := source.Load(context.Background()); err == nil {
		t.Fatal("Load() = nil, want the failed renewal")
	}
	// argocd login saved a fresh login meanwhile.
	store.token, store.refreshToken = "fresh-id-token", "refresh-1"

	if _, err := source.Load(context.Background()); err != nil {
		t.Fatalf("Load() after a new login error = %v, want the new token used", err)
	}
	if got := server.redemptions.Load(); got != 1 {
		t.Fatalf("token endpoint called %d times, want only the failed one", got)
	}
}

func TestARenewedTokenThatIsAlsoRefusedIsNotRenewedAgainAndAgain(t *testing.T) {
	server := newSSOServer(t)
	server.valid = "something-else"
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}
	server.rotated = "refresh-1"
	source := ssoSource(t, server.URL, store)

	for range 3 {
		if _, err := source.Load(context.Background()); err == nil {
			t.Fatal("Load() = nil, want the refusal")
		}
	}
	if got := server.redemptions.Load(); got != 1 {
		t.Fatalf("token endpoint called %d times, want once", got)
	}
}

func TestARepeatedExpiryIsRenewedAgain(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}
	source := ssoSource(t, server.URL, store)

	if _, err := source.Load(context.Background()); err != nil {
		t.Fatalf("first renewal: %v", err)
	}
	// The token the first renewal got expires in its turn, as it does in a
	// session that outlives it.
	server.valid = "next-id-token"
	server.issued = "next-id-token"
	server.refreshToken = "refresh-2"
	server.rotated = "refresh-3"

	if _, err := source.Load(context.Background()); err != nil {
		t.Fatalf("Load() after the second expiry error = %v, want it renewed again", err)
	}
	if got := server.redemptions.Load(); got != 2 {
		t.Fatalf("token endpoint called %d times, want twice", got)
	}
	if store.token != "next-id-token" || store.refreshToken != "refresh-3" {
		t.Fatalf("saved %q / %q, want the second renewal", store.token, store.refreshToken)
	}
}

func TestConcurrentRequestsShareOneRenewal(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}
	source := ssoSource(t, server.URL, store)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			errs <- source.get(context.Background(), "/api/v1/applications", nil)
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("request error = %v", err)
		}
	}
	if got := server.redemptions.Load(); got != 1 {
		t.Fatalf("token endpoint called %d times, want once", got)
	}
}

func TestARetriedRequestSendsItsBodyAgain(t *testing.T) {
	server := newSSOServer(t)
	store := &memoryStore{token: "expired-id-token", refreshToken: "refresh-1"}
	source := ssoSource(t, server.URL, store)

	err := source.request(context.Background(), http.MethodPost, "/api/v1/applications/cart/sync", bytes.NewBufferString(`{"prune":true}`), nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(server.bodies) != 2 || server.bodies[1] != `{"prune":true}` {
		t.Fatalf("bodies = %q, want the same body sent twice", server.bodies)
	}
}

func TestATokenGivenByTheUserIsNeverRenewed(t *testing.T) {
	server := newSSOServer(t)
	source, _ := NewConfiguredAPISource(server.URL, "expired-id-token", APIOptions{})

	_, err := source.Load(context.Background())

	var status *httpStatusError
	if !errors.As(err, &status) || status.code != http.StatusUnauthorized {
		t.Fatalf("Load() error = %v, want the 401", err)
	}
	if got := server.redemptions.Load(); got != 0 {
		t.Fatalf("token endpoint called %d times, want none", got)
	}
}
