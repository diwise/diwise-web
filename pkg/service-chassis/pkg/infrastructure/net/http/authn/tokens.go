package authn

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

// PhantomTokenExchange is responsible for handling login and logout flows
// via a OAuth2/OIDC compatible token server, create an internal session,
// and manage automatic injection of the user's access token based on the
// session id that is stored in the user's browser. The goal with a phantom
// token approach is to keep the token in the backend and reduce the exposure
// of sensitive data to the frontend.
type PhantomTokenExchange interface {
	Middleware(http.Handler) http.Handler
	InstallHandlers(r *http.ServeMux)
	Connect(ctx context.Context, issuerURL string) error
	Shutdown()
}

// cookieContents holds the information that is stored in a cookie in the browser
type cookieContents struct {
	SessionID string `json:"session"`
	SourceIP  string `json:"ip"`
}

// phantomTokens is our internal implementation struct
type phantomTokens struct {
	logger *slog.Logger

	appRoot string

	clientID     string
	clientSecret string

	loginEndpoint  string
	logoutEndpoint string

	cookieName string

	secretKey []byte

	provider              *oidc.Provider
	oauth2Config          oauth2.Config
	insecureSkipVerify    bool
	insecureCookieAllowed bool

	endSessionEndpoint                  string
	pushedAuthenticationRequestEndpoint string

	sessions map[string]*session
	mu       sync.Mutex
}

// InstallHandlers takes a http ServeMux and adds route patterns to handle the
// initiation of login and logout flows from the frontend.
func (pt *phantomTokens) InstallHandlers(r *http.ServeMux) {
	r.HandleFunc("GET "+pt.loginEndpoint, pt.LoginHandler())
	r.HandleFunc("GET "+pt.loginEndpoint+"/{id}", pt.LoginExchangeHandler())
	r.HandleFunc("GET "+pt.logoutEndpoint, pt.LogoutHandler())
}

type PhantomTokenOption func(*phantomTokens)

// WithAppRoot sets the fully qualified domain name, port and base path where
// this service is exposed. If the protocol is http and domain is localhost,
// this function also turns off domain locking for the session cookie.
func WithAppRoot(appRoot string) PhantomTokenOption {
	return func(pt *phantomTokens) {
		for strings.HasSuffix(appRoot, "/") {
			appRoot = appRoot[0 : len(appRoot)-1]
		}
		pt.appRoot = appRoot

		if strings.HasPrefix(pt.appRoot, "http://localhost") {
			pt.insecureCookieAllowed = true
			if separatorIndex := strings.Index(pt.cookieName, "-"); separatorIndex > 0 {
				WithCookieName(pt.cookieName[separatorIndex+1:])(pt)
			}
		}
	}
}

// WithCookieName allows the service backend to specify a custom name to be used
// for the session cookie that is created in the browser. The name will automatically
// be prepended with __Host- to create a "domain locked" cookie.
// See: https://developer.mozilla.org/en-US/docs/Web/HTTP/Cookies#__host-
func WithCookieName(name string) PhantomTokenOption {
	return func(pt *phantomTokens) {
		if !pt.insecureCookieAllowed {
			pt.cookieName = fmt.Sprintf("__Host-%s", name)
		} else {
			pt.cookieName = fmt.Sprintf("insecure-%s", name)
		}
	}
}

// WithInsecureSkipVerify allows for easier testing in environments with self
// signed certificates by disabling the certificate verification when talking
// to the token server. Enabling this will cause a WARNING in the logs for each
// request to the token server. DO NOT put this into production.
func WithInsecureSkipVerify() PhantomTokenOption {
	return func(pt *phantomTokens) {
		pt.insecureSkipVerify = true
	}
}

// WithLogger allows the injection of a custom structured logger into the exchange
func WithLogger(logger *slog.Logger) PhantomTokenOption {
	return func(pt *phantomTokens) {
		pt.logger = logger
	}
}

// WithLoginLogoutEndpoints allows for overriding the default /login and /logout endpoints
func WithLoginLogoutEndpoints(loginEndpoint, logoutEndpoint string) PhantomTokenOption {
	mustBeNonEmptyAndNotEndWithSlash := func(ep string) {
		if len(ep) == 0 {
			panic("endpoint must not be empty")
		}

		if strings.HasSuffix(ep, "/") {
			panic("endpoint must not end with a slash")
		}
	}

	return func(pt *phantomTokens) {
		mustBeNonEmptyAndNotEndWithSlash(loginEndpoint)
		mustBeNonEmptyAndNotEndWithSlash(logoutEndpoint)

		pt.loginEndpoint = loginEndpoint
		pt.logoutEndpoint = logoutEndpoint
	}
}

// WithClientCredentials is used to configure the client name and secret to
// use when talking to the token server
func WithClientCredentials(clientID, clientSecret string) PhantomTokenOption {
	return func(pt *phantomTokens) {
		pt.clientID = clientID
		pt.clientSecret = clientSecret
	}
}

// WithSecretKey specifies the key to use for AES256 encryption of the cookie contents
// NOTE: This key must be exactly 32 bytes of length or else panic will ensue.
func WithSecretKey(key []byte) PhantomTokenOption {
	return func(pt *phantomTokens) {
		if len(key) != 32 {
			panic("aes key size must be 32 bytes")
		}
		pt.secretKey = key
	}
}

// WithRandomKey creates a random 32 byte long key to be used for AES256 encryption of
// the cookie contents.
func WithRandomKey() PhantomTokenOption {
	return func(pt *phantomTokens) {
		// Create a random secret
		secretKey := make([]byte, 32)
		_, err := io.ReadFull(rand.Reader, secretKey)
		if err != nil {
			panic("failed to create secret: " + err.Error())
		}

		WithSecretKey(secretKey)(pt)
	}
}

// NewPhantomTokenExchange constructs and returns a new exchange with a configuration
// according to the supplied configuration options.
func NewPhantomTokenExchange(opts ...PhantomTokenOption) (PhantomTokenExchange, error) {

	defaults := []PhantomTokenOption{
		WithCookieName("id"),
		WithLogger(slog.New(slog.NewTextHandler(os.Stdout, nil))),
		WithLoginLogoutEndpoints("/login", "/logout"),
		WithRandomKey(),
	}

	pt := &phantomTokens{
		sessions: map[string]*session{},
	}

	opts = append(defaults, opts...)
	for _, opt := range opts {
		opt(pt)
	}

	return pt, nil
}

func (pt *phantomTokens) Connect(ctx context.Context, issuerURL string) error {
	ctx = pt.providerClientContext(ctx)

	provider, err := oidc.NewProvider(ctx, issuerURL)
	for err != nil {
		pt.logger.Info("failed to connect to oidc provider", "err", err.Error())
		time.Sleep(2 * time.Second)
		provider, err = oidc.NewProvider(ctx, issuerURL)
	}

	c := struct {
		EndpointPAR        string `json:"pushed_authorization_request_endpoint"`
		EndpointEndSession string `json:"end_session_endpoint"`
	}{}

	if provider.Claims(&c) == nil {
		if c.EndpointPAR == "" {
			return fmt.Errorf("issuer at %s does not have required support for PAR", issuerURL)
		}

		pt.logger.Info("PAR endpoint found at " + c.EndpointPAR)
		pt.pushedAuthenticationRequestEndpoint = c.EndpointPAR
		pt.endSessionEndpoint = c.EndpointEndSession
	}

	pt.provider = provider
	pt.oauth2Config = oauth2.Config{
		ClientID:     pt.clientID,
		ClientSecret: pt.clientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return nil
}

// Shutdown performs any necessary cleanup
func (pt *phantomTokens) Shutdown() {}

// providerClientContext checks the insecureSkipVerify setting and optionally
// returns a new context with an embedded client with disabled certificate
// verification if this setting is true
func (pt *phantomTokens) providerClientContext(ctx context.Context) context.Context {
	if pt.insecureSkipVerify {
		pt.logger.Warn("!!! - PROVIDER CERTIFICATE VERIFICATION DISABLED - !!!")

		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	}

	return ctx
}

// Middleware handles the actual injection of the correct access token (if any)
// based on the session id that may be found in the session cookie
func (pt *phantomTokens) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := pt.getCookie(w, r)

		if err != nil {
			if errors.Is(err, http.ErrAbortHandler) {
				return
			} else if errors.Is(err, errInvalidCookie) {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
		}

		if err == nil {
			tokenChan, err := pt.sessionToken(r.Context(), cookie.SessionID)
			var token *oauth2.Token
			if err == nil {
				token = <-tokenChan
			}

			if err == nil && token == nil {
				err = errors.New("sessionToken returned nil token")
			}

			if err != nil {
				pt.logger.Error("failed to lookup access token", "err", err.Error(), "session", cookie.SessionID)
				pt.clearCookie(w)
				pt.clearSession(cookie.SessionID)
			} else if token != nil {
				r.Header.Set("Authorization", token.TokenType+" "+token.AccessToken)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (pt *phantomTokens) clearCookie(w http.ResponseWriter) {
	cookie := http.Cookie{
		Name:     pt.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !pt.insecureCookieAllowed,
		SameSite: http.SameSiteStrictMode,
	}

	pt.logger.Debug("clearing session cookie from browser")
	http.SetCookie(w, &cookie)
}

var errInvalidCookie = errors.New("invalid cookie")

func (pt *phantomTokens) getCookie(w http.ResponseWriter, r *http.Request) (*cookieContents, error) {
	cookie, err := r.Cookie(pt.cookieName)
	if err != nil {
		return nil, err
	}

	encryptedValue, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		pt.clearCookie(w)
		return nil, fmt.Errorf("decoding failed: %w", err)
	}

	// Create a new AES cipher block from the secret key.
	block, err := aes.NewCipher(pt.secretKey)
	if err != nil {
		panic("new cipher failed: " + err.Error())
	}

	// Wrap the cipher block in Galois Counter Mode.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		panic("new gcm failed: " + err.Error())
	}

	// Get the nonce size.
	nonceSize := aesGCM.NonceSize()

	// To avoid a potential 'index out of range' panic in the next step, we
	// check that the length of the encrypted value is at least the nonce size.
	if len(encryptedValue) < nonceSize {
		err = fmt.Errorf("encrypted value too short: %w", errInvalidCookie)
		pt.logger.Error(err.Error(), "length", len(encryptedValue))
		pt.clearCookie(w)
		return nil, err
	}

	// Split apart the nonce from the actual encrypted data.
	nonce := encryptedValue[:nonceSize]
	ciphertext := encryptedValue[nonceSize:]

	// Use aesGCM.Open() to decrypt and authenticate the data.
	plaintext, err := aesGCM.Open(nil, []byte(nonce), []byte(ciphertext), nil)
	if err != nil {
		pt.logger.Error(
			"failed to decrypt and authenticate cookie data",
			"name", pt.cookieName,
			"value", cookie.Value,
			"keysize", len(pt.secretKey),
			"noncesize", nonceSize,
			"err", err.Error(),
		)
		pt.clearCookie(w)
		// Redirect the browser to the login endpoint to attempt a refresh
		// of the authorization token and cookie
		path := url.QueryEscape(r.URL.Path)
		http.Redirect(w, r, pt.loginEndpoint+"?path="+path, http.StatusFound)
		return nil, http.ErrAbortHandler
	}

	value := &cookieContents{}
	err = json.Unmarshal(plaintext, &value)
	if err != nil {
		pt.logger.Error("cookie contents error", "err", err.Error())
		pt.clearCookie(w)
		return nil, fmt.Errorf("cookie contents error: %w", errInvalidCookie)
	}

	if value.SourceIP != "" && value.SourceIP != r.Header.Get("X-Real-IP") {
		pt.logger.Error("session ip address changed", "old", value.SourceIP, "new", r.Header.Get("X-Real-IP"), "session", value.SessionID)
		pt.clearCookie(w)
		return nil, fmt.Errorf("session ip address changed: %w", errInvalidCookie)
	}

	return value, nil
}

func (pt *phantomTokens) newCookie(value cookieContents) (*http.Cookie, error) {

	// Set httponly, secure and strict samesite mode for our cookies
	// See https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#cookies
	sameSiteMode := map[bool]http.SameSite{false: http.SameSiteStrictMode, true: http.SameSiteLaxMode}[pt.insecureCookieAllowed]
	cookie := http.Cookie{
		Name:     pt.cookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   !pt.insecureCookieAllowed,
		SameSite: sameSiteMode,
	}

	if pt.insecureCookieAllowed {
		pt.logger.Warn("!!! - INSECURE COOKIE CREATED - !!!")
	}

	// Create a new AES cipher block from the secret key.
	block, err := aes.NewCipher(pt.secretKey)
	if err != nil {
		return nil, err
	}

	// Wrap the cipher block in Galois Counter Mode.
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a unique nonce containing 12 random bytes.
	nonce := make([]byte, aesGCM.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	marshalledBytes, _ := json.Marshal(value)

	// Encrypt the data using aesGCM.Seal(). By passing the nonce as the first
	// parameter, the encrypted data will be appended to the nonce — meaning
	// that the returned encryptedValue variable will be in the format
	// "{nonce}{encrypted plaintext data}".
	encryptedValue := aesGCM.Seal(nonce, nonce, marshalledBytes, nil)

	// Encode the encrypted cookie value using base64.
	cookie.Value = base64.URLEncoding.EncodeToString(encryptedValue)

	return &cookie, nil
}

type tokenState int

const (
	NONE       tokenState = 0
	REFRESHING tokenState = 1
	ACTIVE     tokenState = 2
)

type session struct {
	ID           string
	LoginState   string
	PKCEVerifier string

	IDToken    string
	Token      *oauth2.Token
	TokenState tokenState
	TokenQueue []chan (*oauth2.Token)
}

func (pt *phantomTokens) clearSession(sessionID string) *session {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s, ok := pt.sessions[sessionID]
	if !ok {
		return nil
	}

	pt.logger.Info("clearing session from memory", "session", sessionID)

	// Release any blocked requests that are waiting for a token refresh
	if len(s.TokenQueue) > 0 {
		pt.logger.Warn("clearing session with pending token requests", "count", len(s.TokenQueue), "session", sessionID)
		for _, consumer := range s.TokenQueue {
			consumer <- nil
		}
		s.TokenQueue = []chan (*oauth2.Token){}
	}

	delete(pt.sessions, sessionID)
	return s
}

func (pt *phantomTokens) newSession() *session {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s := &session{
		ID:           uuid.NewString(),
		LoginState:   uuid.NewString(),
		PKCEVerifier: oauth2.GenerateVerifier(),
		TokenQueue:   []chan (*oauth2.Token){},
	}
	pt.sessions[s.ID] = s
	return s
}

var ErrNoSuchSession error = errors.New("no such session")
var ErrNoToken error = errors.New("session has no token")
var ErrRefreshTokenExpired error = errors.New("refresh token expired")

func (pt *phantomTokens) sessionLoginState(_ context.Context, sessionID string) (string, error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s, ok := pt.sessions[sessionID]
	if !ok {
		return "", ErrNoSuchSession
	}

	return s.LoginState, nil
}

func (pt *phantomTokens) sessionPKCEVerifier(_ context.Context, sessionID string) (string, error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s, ok := pt.sessions[sessionID]
	if !ok {
		return "", ErrNoSuchSession
	}

	return s.PKCEVerifier, nil
}

func (pt *phantomTokens) sessionHasToken(_ context.Context, sessionID string) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s, ok := pt.sessions[sessionID]
	if ok && s.TokenState != NONE {
		return true
	}

	return false
}

func (pt *phantomTokens) sessionToken(ctx context.Context, sessionID string) (chan (*oauth2.Token), error) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s, ok := pt.sessions[sessionID]
	if !ok {
		return nil, ErrNoSuchSession
	}

	if s.Token == nil {
		return nil, ErrNoToken
	}

	result := make(chan (*oauth2.Token), 1)

	if s.Token.Valid() {
		result <- s.Token
	} else {
		s.TokenQueue = append(s.TokenQueue, result)

		if s.TokenState == ACTIVE {
			s.TokenState = REFRESHING
			pt.logger.Info("initiating token refresh", "session", sessionID)

			go func(t *oauth2.Token) {
				ctx := pt.providerClientContext(context.WithoutCancel(ctx))
				tokenSource := pt.oauth2Config.TokenSource(ctx, t)
				t, err := tokenSource.Token()

				if err != nil {
					pt.logger.Error("failed to refresh token", "err", err.Error(), "session", sessionID)
					t = nil
				}

				pt.sessionTokens(ctx, sessionID, s.IDToken, t)
			}(s.Token)
		} else {
			pt.logger.Info("queuing token request due to pending refresh", "session", sessionID)
		}
	}

	return result, nil
}

func (pt *phantomTokens) sessionTokens(_ context.Context, sessionID, idToken string, token *oauth2.Token) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	s, ok := pt.sessions[sessionID]
	if !ok {
		return ErrNoSuchSession
	}

	s.IDToken = idToken
	s.Token = token
	s.TokenState = ACTIVE

	if len(s.TokenQueue) > 0 {
		pt.logger.Info("sending refreshed token to blocked consumers", "count", len(s.TokenQueue), "session", sessionID)

		for _, consumer := range s.TokenQueue {
			consumer <- token
		}
		s.TokenQueue = []chan (*oauth2.Token){}
	}

	return nil
}

// LoginHandler services GET requests directed at the login endpoint by pushing an
// authentication request to the token server's PAR endpoint, and redirecting the
// client using the redirect uri that the token server returns.
// See: https://oauth.net/2/pushed-authorization-requests/
func (pt *phantomTokens) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := pt.newSession()

		redirectUri := pt.appRoot + pt.loginEndpoint + "/" + s.ID
		path := r.URL.Query().Get("path")
		if path != "" {
			redirectUri = redirectUri + "?path=" + path
		}

		par := url.Values{}
		par.Add("response_type", "code")
		par.Add("client_id", pt.clientID)
		par.Add("scope", strings.Join(pt.oauth2Config.Scopes, " "))
		par.Add("state", s.LoginState)
		par.Add("code_challenge_method", "S256")
		par.Add("code_challenge", oauth2.S256ChallengeFromVerifier(s.PKCEVerifier))
		par.Add("redirect_uri", redirectUri)
		//par.Add("response_mode", "form_post")

		postReq, _ := http.NewRequest(http.MethodPost, pt.pushedAuthenticationRequestEndpoint, strings.NewReader(par.Encode()))
		postReq.SetBasicAuth(url.QueryEscape(pt.clientID), url.QueryEscape(pt.clientSecret))
		postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		client := http.DefaultClient

		if pt.insecureSkipVerify {
			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}
		}

		resp, err := client.Do(postReq)
		if err != nil {
			pt.logger.Error("par endpoint failure", "err", err.Error(), "session", s.ID)
			pt.clearSession(s.ID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		respJson, _ := io.ReadAll(resp.Body)

		requestObject := struct {
			URI              string `json:"request_uri"`
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}{}
		json.Unmarshal(respJson, &requestObject)

		if resp.StatusCode >= http.StatusBadRequest {
			pt.logger.Error("par error", "code", resp.StatusCode, "error", requestObject.Error, "description", requestObject.ErrorDescription, "session", s.ID)
			pt.clearSession(s.ID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if resp.StatusCode != http.StatusCreated {
			pt.logger.Error("invalid response from par endoint", "code", resp.StatusCode, "session", s.ID)
			pt.clearSession(s.ID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		redirectURI := fmt.Sprintf("%s?client_id=%s&request_uri=%s",
			pt.oauth2Config.Endpoint.AuthURL,
			pt.clientID,
			url.QueryEscape(requestObject.URI),
		)

		pt.logger.Debug("redirecting user to token server for authentication", "redirect_uri", redirectURI, "session", s.ID)

		http.Redirect(w, r, redirectURI, http.StatusFound)
	}
}

// LoginExchangeHandler services GET requests directed at the redirect uri that was specified
// when the login handler pushed the initiating authentication request to the token server.
// The authorization code that was issued by the token server is extracted in this endpoint and
// sent to the token server to be exchanged for access, refresh and id tokens.
func (pt *phantomTokens) LoginExchangeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := pt.providerClientContext(r.Context())

		sessionID := r.PathValue("id")
		loginState, err1 := pt.sessionLoginState(ctx, sessionID)
		pkceVerifier, err2 := pt.sessionPKCEVerifier(ctx, sessionID)

		if err1 != nil || err2 != nil {
			pt.logger.Warn("attempt to login with invalid session id")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if pt.sessionHasToken(ctx, sessionID) {
			pt.logger.Warn("possibly malicious call: this session has already exchanged token", "session", sessionID)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var err error
		defer func() {
			if err != nil {
				pt.clearSession(sessionID)
			}
		}()

		state := r.URL.Query().Get("state")
		if state != loginState {
			err = errors.New("state parameter does not match")
			pt.logger.Warn("suspicious login attempt", "session", sessionID, "err", err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}

		redirectUri := pt.appRoot + pt.loginEndpoint + "/" + sessionID
		path := r.URL.Query().Get("path")
		if path != "" {
			redirectUri = redirectUri + "?path=" + path
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			code = r.Form.Get("code")
		}

		pt.logger.Debug("received authorization code", "code", code, "session", sessionID)

		if code == "" {
			err = errors.New("code parameter missing in request")
			pt.logger.Warn("invalid login attempt", "session", sessionID, "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		exchange := url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"code_verifier": {pkceVerifier},
			"redirect_uri":  {redirectUri},
		}

		postReq, _ := http.NewRequest(http.MethodPost, pt.oauth2Config.Endpoint.TokenURL, strings.NewReader(exchange.Encode()))
		postReq.SetBasicAuth(url.QueryEscape(pt.clientID), url.QueryEscape(pt.clientSecret))
		postReq.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		client := http.DefaultClient

		if pt.insecureSkipVerify {
			client = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				},
			}
		}

		exchangeResponse, err := client.Do(postReq)

		if err != nil {
			pt.logger.Error("failed to exchange token", "err", err.Error(), "session", sessionID)
			http.Error(w, "failed to exchange token", http.StatusInternalServerError)
			return
		}
		defer exchangeResponse.Body.Close()

		if exchangeResponse.StatusCode != http.StatusOK {
			err = fmt.Errorf("invalid status code")
			pt.logger.Error("token server error", "code", exchangeResponse.StatusCode, "session", sessionID)
			http.Error(w, "invalid status code from token server", http.StatusInternalServerError)
			return
		}

		body, _ := io.ReadAll(exchangeResponse.Body)

		oauth2Token := &oauth2.Token{}
		err = json.Unmarshal(body, &oauth2Token)
		if err != nil {
			pt.logger.Error("failed to unmarshal token", "err", err.Error(), "session", sessionID)
			http.Error(w, "failed to unmarshal token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		extra := &struct {
			IDToken          string `json:"id_token"`
			ExpiresIn        int32  `json:"expires_in"`
			RefreshExpiresIn int32  `json:"refresh_expires_in"`
		}{}
		err = json.Unmarshal(body, extra)
		if err != nil || extra.IDToken == "" {
			http.Error(w, "no id_token field in oauth2 token.", http.StatusInternalServerError)
			return
		}

		oauth2Token.Expiry = time.Now().Add(time.Duration(extra.ExpiresIn) * time.Second)
		pt.logger.Info("token expiry", "when", oauth2Token.Expiry.Format(time.RFC3339), "session", sessionID)

		verifier := pt.provider.Verifier(&oidc.Config{ClientID: pt.clientID})
		_, err = verifier.Verify(ctx, extra.IDToken)
		if err != nil {
			pt.logger.Error("failed to verify id token", "err", err.Error(), "session", sessionID)
			http.Error(w, "failed to verify id token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		pt.sessionTokens(ctx, sessionID, extra.IDToken, oauth2Token)

		var newCookie *http.Cookie
		newCookie, err = pt.newCookie(cookieContents{
			SessionID: sessionID,
			SourceIP:  r.Header.Get("X-Real-IP"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		redirectUri = "/"
		path = r.URL.Query().Get("path")
		if path != "" {
			path, err := url.QueryUnescape(path)
			if err == nil {
				redirectUri = path
			}
		}

		http.SetCookie(w, newCookie)
		http.Redirect(w, r, redirectUri, http.StatusFound)
	}
}

// LogoutHandler services GET requests directed at the logout endpoint. It clears the session,
// deletes the cookie and redirects the client to the end session endpoint of the token server
func (pt *phantomTokens) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := pt.getCookie(w, r)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		pt.clearCookie(w)
		s := pt.clearSession(cookie.SessionID)

		if s != nil && s.Token.Valid() {
			logoutURL := pt.endSessionEndpoint + "?id_token_hint=" + s.IDToken + "&post_logout_redirect_uri="
			logoutURL += url.QueryEscape(pt.appRoot)

			w.Header().Set("Location", logoutURL)
			w.WriteHeader(http.StatusFound)
			return
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
