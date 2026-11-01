package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

type Provider interface {
	URL(state, nonce, verifier string) string
	Exchange(context.Context, string, string, string) (Identity, error)
}
type googleProvider struct {
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client
}

func newGoogle(c Config) Provider {
	client := &http.Client{Timeout: 8 * time.Second}
	ctx := oidc.ClientContext(context.Background(), client)
	return &googleProvider{config: oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: endpoints.Google, RedirectURL: c.Origin + "/api/auth/google/callback", Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}, verifier: oidc.NewVerifier("https://accounts.google.com", oidc.NewRemoteKeySet(ctx, "https://www.googleapis.com/oauth2/v3/certs"), &oidc.Config{ClientID: c.ClientID}), client: client}
}
func (p *googleProvider) URL(state, nonce, verifier string) string {
	return p.config.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "select_account"))
}
func (p *googleProvider) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	token, err := p.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, err
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("missing ID token")
	}
	id, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, err
	}
	var claims struct {
		Email           string `json:"email"`
		Name            string `json:"name"`
		AuthorizedParty string `json:"azp"`
		EmailVerified   bool   `json:"email_verified"`
	}
	if err = id.Claims(&claims); err != nil {
		return Identity{}, err
	}
	if id.Subject == "" || id.Nonce != nonce || (len(id.Audience) > 1 && claims.AuthorizedParty != p.config.ClientID) || (claims.AuthorizedParty != "" && claims.AuthorizedParty != p.config.ClientID) {
		return Identity{}, errors.New("invalid identity claims")
	}
	if !claims.EmailVerified {
		claims.Email = ""
	}
	return Identity{Subject: id.Subject, Email: claims.Email, Name: claims.Name}, nil
}
