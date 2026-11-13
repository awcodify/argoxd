package argocd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// dexCLIClientID is the client the argocd CLI uses with Argo CD's own Dex.
const dexCLIClientID = "argo-cd-cli"

// currentToken returns the token to send.
func (s *APISource) currentToken() string {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	return s.token
}

// renew is called after Argo CD refused the token a request sent. It
// reports whether there is a new token to retry with: one saved meanwhile, by
// the argocd CLI or by another request, or one the IdP issued for the saved
// refresh token. A failure to renew is returned to be shown with the refusal.
// A token that could not be renewed, or that was refused right after being
// issued, is not renewed again, so the IdP is not asked on every refresh.
func (s *APISource) renew(ctx context.Context, refused string) (bool, error) {
	if s.tokenStore == nil {
		return false, nil
	}
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if s.token != refused {
		return true, nil
	}
	saved, refreshToken, err := s.tokenStore.Load()
	if err == nil && saved != "" && saved != refused {
		s.token, s.stuckToken = saved, ""
		return true, nil
	}
	if refreshToken == "" || refused == s.stuckToken {
		return false, nil
	}
	// Until it works, this token is not renewed again; nor is the new one, if
	// Argo CD refuses it as well.
	s.stuckToken = refused
	token, newRefreshToken, err := s.redeem(ctx, refreshToken)
	if err != nil {
		return false, err
	}
	s.token, s.stuckToken = token, token
	// A login that cannot be saved still works for this run.
	_ = s.tokenStore.Save(token, newRefreshToken)
	return true, nil
}

// redeem swaps a refresh token for a new ID token at the IdP Argo CD logs in
// through, as "argocd" does. The refresh token is kept when the IdP does not
// issue a new one.
func (s *APISource) redeem(ctx context.Context, refreshToken string) (string, string, error) {
	issuer, clientID, err := s.oidcClient(ctx)
	if err != nil {
		return "", "", err
	}
	endpoint, err := s.tokenEndpoint(ctx, issuer)
	if err != nil {
		return "", "", err
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {clientID}}
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

// oidcClient returns the issuer and client ID the argocd CLI logs in with,
// from Argo CD's public settings: the configured OIDC provider, or else
// Argo CD's own Dex.
func (s *APISource) oidcClient(ctx context.Context) (string, string, error) {
	var settings struct {
		URL        string `json:"url"`
		OIDCConfig *struct {
			Issuer      string `json:"issuer"`
			ClientID    string `json:"clientID"`
			CLIClientID string `json:"cliClientID"`
		} `json:"oidcConfig"`
		DexConfig *struct {
			Connectors []json.RawMessage `json:"connectors"`
		} `json:"dexConfig"`
	}
	// The settings need no token, and sending the refused one would only fail.
	response, err := s.sendWith(ctx, s.client, http.MethodGet, "/api/v1/settings", nil, "")
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(&settings); err != nil {
		return "", "", fmt.Errorf("read Argo CD settings: %w", err)
	}
	switch {
	case settings.OIDCConfig != nil && settings.OIDCConfig.Issuer != "":
		clientID := settings.OIDCConfig.CLIClientID
		if clientID == "" {
			clientID = settings.OIDCConfig.ClientID
		}
		return settings.OIDCConfig.Issuer, clientID, nil
	case settings.DexConfig != nil && len(settings.DexConfig.Connectors) > 0:
		base := strings.TrimRight(settings.URL, "/")
		if base == "" {
			base = s.baseURL
		}
		return base + "/api/dex", dexCLIClientID, nil
	}
	return "", "", errors.New("no SSO is configured in Argo CD")
}

// tokenEndpoint finds the token endpoint of an OpenID Connect issuer.
func (s *APISource) tokenEndpoint(ctx context.Context, issuer string) (string, error) {
	discovery := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discovery, nil)
	if err != nil {
		return "", err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s returned %s", discovery, response.Status)
	}
	var configuration struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&configuration); err != nil || configuration.TokenEndpoint == "" {
		return "", fmt.Errorf("%s has no token endpoint", discovery)
	}
	return configuration.TokenEndpoint, nil
}
