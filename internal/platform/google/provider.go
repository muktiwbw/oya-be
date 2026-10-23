package google

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"oya-be/internal/user"
)

const issuer = "https://accounts.google.com"

type Provider struct {
	config   oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func New(ctx context.Context, clientID, clientSecret, redirectURL string) (*Provider, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover google provider: %w", err)
	}
	return &Provider{
		config:   oauth2.Config{ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"}},
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

func (p *Provider) AuthorizationURL(state, nonce, verifier string) string {
	return p.config.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier), oauth2.AccessTypeOffline)
}

func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (user.GoogleIdentity, error) {
	tokens, err := p.config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return user.GoogleIdentity{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	raw, ok := tokens.Extra("id_token").(string)
	if !ok {
		return user.GoogleIdentity{}, fmt.Errorf("google response missing id token")
	}
	idToken, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return user.GoogleIdentity{}, fmt.Errorf("verify google id token: %w", err)
	}
	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Nonce         string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return user.GoogleIdentity{}, fmt.Errorf("decode google claims: %w", err)
	}
	if claims.Nonce != nonce {
		return user.GoogleIdentity{}, fmt.Errorf("google nonce mismatch")
	}
	return user.GoogleIdentity{Subject: claims.Subject, Email: claims.Email, EmailVerified: claims.EmailVerified, DisplayName: claims.Name}, nil
}

var _ user.GoogleProvider = (*Provider)(nil)
