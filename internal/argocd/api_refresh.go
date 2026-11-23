package argocd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// TokenStore holds an SSO login, such as the one the argocd CLI saves with
// "argocd login --sso", so that an expired token can be renewed and the new
// one kept for the next run, and for the CLI.
type TokenStore interface {
	// Load returns the saved token and refresh token. The refresh token is
	// empty for a login that cannot be renewed.
	Load() (token, refreshToken string, err error)
	// Save replaces the saved token and refresh token.
	Save(token, refreshToken string) error
}

// The argocd CLI asks for these scopes when Argo CD does not configure them.
var defaultOIDCScopes = []string{"openid", "profile", "email", "groups"}

const (
	// dexCLIClientID is the client the argocd CLI uses with Argo CD's own Dex.
	dexCLIClientID = "argo-cd-cli"
	// dexFederatedScope is the extra scope the argocd CLI asks of Argo CD's Dex.
	dexFederatedScope = "federated:id"
	// offlineAccessScope is what a login needs to be given a refresh token.
	offlineAccessScope = "offline_access"
)

// currentToken returns the token to send.
func (s *APISource) currentToken() string {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	return s.token
}

// renew is called after Argo CD refused the token a request sent. It reports
// whether there is a new token to retry with, and whether the IdP issued it
// now (as opposed to one saved meanwhile by the argocd CLI or another request).
// A failure to renew is returned to be shown with the refusal, and is returned
// again with every later refusal until a renewal works, so the reason stays on
// screen. A token that could not be renewed is not renewed again, so the IdP is
// not asked on every refresh.
func (s *APISource) renew(ctx context.Context, refused string) (renewed, issued bool, err error) {
	if s.tokenStore == nil {
		return false, false, nil
	}
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if s.token != refused {
		s.stuckErr = nil
		return true, false, nil
	}
	saved, refreshToken, err := s.tokenStore.Load()
	if err == nil && saved != "" && saved != refused {
		s.token, s.stuckToken, s.stuckErr = saved, "", nil
		return true, false, nil
	}
	if refreshToken == "" {
		return false, false, nil
	}
	if refused == s.stuckToken {
		return false, false, s.stuckErr
	}
	// Until it works, this token is not renewed again, and every refresh keeps
	// saying why it could not be.
	s.stuckToken, s.stuckErr = refused, nil
	token, newRefreshToken, err := s.redeem(ctx, refreshToken)
	if err != nil {
		s.stuckErr = err
		return false, false, err
	}
	// The new token is marked stuck only if Argo CD refuses it as well, so a
	// later expiry of it is renewed too.
	s.token, s.stuckToken, s.stuckErr = token, "", nil
	// A login that cannot be saved still works for this run.
	_ = s.tokenStore.Save(token, newRefreshToken)
	return true, true, nil
}

// markStuck records that Argo CD refused a token the IdP had just issued, so
// it is not renewed again on every request. A token the CLI saves later still
// replaces it.
func (s *APISource) markStuck(token string) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if s.token == token {
		s.stuckToken, s.stuckErr = token, nil
	}
}

// redeem swaps a refresh token for a new ID token at the IdP Argo CD logs in
// through, as "argocd" does. The refresh token is kept when the IdP does not
// issue a new one.
func (s *APISource) redeem(ctx context.Context, refreshToken string) (string, string, error) {
	issuer, clientID, scopes, err := s.oidcClient(ctx)
	if err != nil {
		return "", "", err
	}
	endpoint, supportedScopes, err := s.tokenEndpoint(ctx, issuer)
	if err != nil {
		return "", "", err
	}
	// The argocd CLI asks for the configured scopes again. Some IdPs, such as
	// Azure AD and Okta, only return an ID token when "openid" is asked for.
	scopes = addScope(scopes, supportedScopes, offlineAccessScope)
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {clientID}}
	if len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	var token struct {
		IDToken          string `json:"id_token"`
		RefreshToken     string `json:"refresh_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token)
	if response.StatusCode != http.StatusOK {
		reason := token.ErrorDescription
		if reason == "" {
			reason = token.Error
		}
		if reason == "" {
			reason = response.Status
		}
		return "", "", errors.New(reason)
	}
	if decodeErr != nil {
		return "", "", fmt.Errorf("read token response: %w", decodeErr)
	}
	if token.IDToken == "" {
		return "", "", errors.New("the IdP returned no ID token")
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token.IDToken, token.RefreshToken, nil
}

// oidcClient returns the issuer, client ID and scopes the argocd CLI logs in
// with, from Argo CD's public settings: the configured OIDC provider, or else
// Argo CD's own Dex.
func (s *APISource) oidcClient(ctx context.Context) (string, string, []string, error) {
	var settings struct {
		URL        string `json:"url"`
		OIDCConfig *struct {
			Issuer      string   `json:"issuer"`
			ClientID    string   `json:"clientID"`
			CLIClientID string   `json:"cliClientID"`
			Scopes      []string `json:"scopes"`
		} `json:"oidcConfig"`
		DexConfig *struct {
			Connectors []json.RawMessage `json:"connectors"`
		} `json:"dexConfig"`
	}
	// The settings need no token, and sending the refused one would only fail.
	response, err := s.sendWith(ctx, s.client, http.MethodGet, "/api/v1/settings", nil, "")
	if err != nil {
		return "", "", nil, err
	}
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(&settings); err != nil {
		return "", "", nil, fmt.Errorf("read Argo CD settings: %w", err)
	}
	switch {
	case settings.OIDCConfig != nil && settings.OIDCConfig.Issuer != "":
		clientID := settings.OIDCConfig.CLIClientID
		if clientID == "" {
			clientID = settings.OIDCConfig.ClientID
		}
		scopes := settings.OIDCConfig.Scopes
		if len(scopes) == 0 {
			scopes = defaultOIDCScopes
		}
		return settings.OIDCConfig.Issuer, clientID, slices.Clone(scopes), nil
	case settings.DexConfig != nil && len(settings.DexConfig.Connectors) > 0:
		base := strings.TrimRight(settings.URL, "/")
		if base == "" {
			base = s.baseURL
		}
		scopes := append(slices.Clone(defaultOIDCScopes), dexFederatedScope)
		return base + "/api/dex", dexCLIClientID, scopes, nil
	}
	return "", "", nil, errors.New("no SSO is configured in Argo CD")
}

// addScope adds a scope to a request when the provider supports it and it is
// not already asked for.
func addScope(scopes, supported []string, scope string) []string {
	if slices.Contains(scopes, scope) || !slices.Contains(supported, scope) {
		return scopes
	}
	return append(scopes, scope)
}

// tokenEndpoint finds the token endpoint and supported scopes of an OpenID
// Connect issuer.
func (s *APISource) tokenEndpoint(ctx context.Context, issuer string) (string, []string, error) {
	discovery := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discovery, nil)
	if err != nil {
		return "", nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("%s returned %s", discovery, response.Status)
	}
	var configuration struct {
		TokenEndpoint   string   `json:"token_endpoint"`
		ScopesSupported []string `json:"scopes_supported"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&configuration); err != nil || configuration.TokenEndpoint == "" {
		return "", nil, fmt.Errorf("%s has no token endpoint", discovery)
	}
	return configuration.TokenEndpoint, configuration.ScopesSupported, nil
}
