package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

func signedToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	input := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString(payload)
	h := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}
func TestVerifiedOIDCClaims(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"valid", "signature", "issuer", "audience", "expiry", "nonce", "azp", "missing_token", "pkce"} {
		t.Run(scenario, func(t *testing.T) {
			claims := map[string]any{"iss": "https://issuer.example", "aud": "client", "sub": "subject", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": "nonce", "name": "Person", "email": "person@example.com", "email_verified": true}
			signingKey := key
			switch scenario {
			case "signature":
				signingKey = other
			case "issuer":
				claims["iss"] = "https://wrong.example"
			case "audience":
				claims["aud"] = "wrong"
			case "expiry":
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
			case "nonce":
				claims["nonce"] = "wrong"
			case "azp":
				claims["aud"] = []string{"client", "other"}
				claims["azp"] = "other"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = r.ParseForm()
				if r.PostForm.Get("code_verifier") != "verifier" || scenario == "pkce" {
					http.Error(w, "invalid_grant", 400)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				body := map[string]any{"access_token": "unused", "token_type": "Bearer", "id_token": signedToken(t, signingKey, claims)}
				if scenario == "missing_token" {
					delete(body, "id_token")
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			defer server.Close()
			p := &googleProvider{config: oauth2.Config{ClientID: "client", ClientSecret: "secret", Endpoint: oauth2.Endpoint{TokenURL: server.URL, AuthStyle: oauth2.AuthStyleInParams}}, verifier: oidc.NewVerifier("https://issuer.example", &oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{&key.PublicKey}}, &oidc.Config{ClientID: "client"}), client: server.Client()}
			identity, err := p.Exchange(context.Background(), "code", "verifier", "nonce")
			if scenario == "valid" {
				if err != nil || identity.Subject != "subject" {
					t.Fatal(identity, err)
				}
			} else if err == nil {
				t.Fatalf("%s accepted", scenario)
			}
		})
	}
}
func TestNoSecretsInProviderURL(t *testing.T) {
	u := newGoogle(Config{Origin: "https://fitness.example.com", ClientID: "public-id", ClientSecret: "super-secret"}).URL("state", "nonce", "verifier")
	if strings.Contains(u, "super-secret") || strings.Contains(u, "code_verifier") {
		t.Fatal("credential in browser URL")
	}
}
