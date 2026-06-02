package authn

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	testutil "github.com/diwise/service-chassis/pkg/test/http"
	"github.com/diwise/service-chassis/pkg/test/http/expects"
	"github.com/diwise/service-chassis/pkg/test/http/response"
	"github.com/matryer/is"
)

func TestA(t *testing.T) {
	is_ := is.New(t)
	r := http.NewServeMux()

	var ms testutil.MockService
	var authmock testutil.MockService
	var parmock testutil.MockService
	var tokenmock testutil.MockService

	ms = testutil.NewMockServiceThat(
		testutil.Expects(
			is_,
			expects.AnyInput(),
		),
		testutil.Returns(
			response.ContentType("application/json"),
			response.Code(http.StatusOK),
			response.BodyFromCallback(
				func() []byte {
					return oidcConfigResponse(ms.URL(), authmock.URL(), parmock.URL(), tokenmock.URL())
				}),
		),
	)
	defer ms.Close()

	var redirect_uri string
	var state string

	parmock = testutil.NewMockServiceThat(
		testutil.Expects(
			is_,
			expects.RequestHeaderContains("Content-Type", "application/x-www-form-urlencoded"),
			expects.RequestPath("/ext/par"),
			expects.RequestBodyIsForm(func(is *is.I, v url.Values) {
				is.Equal(v.Get("response_type"), "code")
				redirect_uri, _ = url.QueryUnescape(v.Get("redirect_uri"))
				state = v.Get("state")
			}),
		),
		testutil.Returns(
			response.ContentType("application/json"),
			response.Code(http.StatusCreated),
			response.BodyFromCallback(func() []byte { return parResponse("thisisauri") }),
		),
	)
	defer parmock.Close()

	authmock = testutil.NewMockServiceThat(
		testutil.Expects(
			is_,
			expects.RequestPath("/auth"),
			expects.QueryParamEquals("request_uri", "thisisauri"),
		),
		testutil.Returns(
			response.Code(http.StatusOK),
		),
	)
	defer authmock.Close()

	tokenmock = testutil.NewMockServiceThat(
		testutil.Expects(
			is_,
			expects.RequestPath("/token"),
		),
		testutil.Returns(
			response.Code(http.StatusOK),
			response.BodyFromCallback(func() []byte { return tokenResponse(ms.URL()) }),
		),
	)
	defer tokenmock.Close()

	pte, err := NewPhantomTokenExchange(
		WithClientCredentials("hello-world", "passw0rd"),
	)
	is_.NoErr(err)
	defer pte.Shutdown()

	pte.Connect(context.Background(), ms.URL())

	pte.InstallHandlers(r)

	server := httptest.NewServer(r)
	defer server.Close()

	resp, _ := testRequest(is_, server, http.MethodGet, "/login", nil)
	is_.Equal(resp.StatusCode, http.StatusOK)

	resp, _ = testRequest(is_, server, http.MethodGet, redirect_uri+"?state="+state, nil)
	// We have to check for 500 here because the token validation will fail
	// until we add mocks for providing a matching JWKS
	// See "failed to verify signature: fetching keys oidc: get keys failed" in test output
	is_.Equal(resp.StatusCode, http.StatusInternalServerError)

	is_.Equal(ms.RequestCount(), 1)
	is_.Equal(parmock.RequestCount(), 1)
	is_.Equal(authmock.RequestCount(), 1)
	is_.Equal(tokenmock.RequestCount(), 1)
}

const oidcResponseFmt string = `{
	"issuer": "%s",
	"authorization_endpoint": "%s/auth",
	"pushed_authorization_request_endpoint": "%s/ext/par",
	"token_endpoint": "%s/token"
}`

func oidcConfigResponse(issuer, authEndpoint, parEndpoint, tokenEndpoint string) []byte {
	response := fmt.Sprintf(oidcResponseFmt, issuer, authEndpoint, parEndpoint, tokenEndpoint)
	return []byte(response)
}

const parResponseFmt string = `{
	"request_uri": "%s",
	"expires_in": 90
}`

func parResponse(uri string) []byte {
	response := fmt.Sprintf(parResponseFmt, uri)
	return []byte(response)
}

const accessTokenFmt string = `{"exp":%d,"iat":%d,"auth_time":%d,"jti":"f4d12961-0d2e-4f98-9a76-734a16566430","iss":"%s","sub":"8b82f1d1-d1c4-49dc-8d42-f4e9cb299f76","typ":"Bearer","azp":"hello-world","session_state":"0c6609ff-9e06-4bbe-8910-cbf7471a9c65","acr":"1","allowed-origins":["*"],"scope":"openid profile email","sid":"0c6609ff-9e06-4bbe-8910-cbf7471a9c65","email_verified":true,"name":"Anna Panna","preferred_username":"anna","given_name":"Anna","family_name":"Panna","email":"anna@somewhere.net"}`

const refreshTokenFmt string = `{"exp":%d,"iat":%d,"jti": "944bce5d-8b65-4ec7-928e-e449585bd456","iss":"%s","aud":"%s","sub":"8b82f1d1-d1c4-49dc-8d42-f4e9cb299f76","typ": "Refresh","azp": "hello-world","session_state":"0c6609ff-9e06-4bbe-8910-cbf7471a9c65","scope": "openid profile email","sid": "0c6609ff-9e06-4bbe-8910-cbf7471a9c65"}`

const idTokenFmt string = `{"exp":%d,"iat":%d,"auth_time":%d,"jti":"6ca15cfb-6b66-4fd5-bfcc-0606bc0cdc02","iss":"%s","aud":"hello-world","sub":"8b82f1d1-d1c4-49dc-8d42-f4e9cb299f76","typ":"ID","azp":"hello-world","session_state":"0c6609ff-9e06-4bbe-8910-cbf7471a9c65","at_hash":"1eFxzp643bDAPMC3U9Yi_g","acr":"1","sid":"0c6609ff-9e06-4bbe-8910-cbf7471a9c65","email_verified":true,"name":"Anna Panna","preferred_username":"anna","given_name":"Anna","family_name":"Panna","email":"anna@somewhere.net"}`

const tokenResponseFmt string = `{"access_token":"eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJrZV96VGhKU1hoMDM0bEt3Q1BBeDJQWmltdlA4QnhYRS01c1h2Vng0cklnIn0.%s.M6ti-WbjCs6ZBiGc9yA9KIWc-GaQTDO9xiyyZjyNDNFP3sYQX3ullVTomCxGF5_5IDn2MBUb82s0nqTzyELrlu1jiOR81BsZ5hGkbQPrmFSXqVuaESGwPhWwqewAeAJ7P-gp0B5ZRBEdZGfIo9VyfoMcyeoUcAXeDKhPTXUlf59OJOXYbDZmpSmE-pH67TqSDRKoPDkTwa7yaKPDNS7hHhpjm0t-t1proEG5mOm4IsD_N0ZLpUS7mTH_EbQx3kRki60mwe2fvGfnZjuc_yyxi-xeK6aQGbVRqTYZodaXzDO6PkRVM0CdMlSb4NTgASqk9zePJaXWRjvHnKg_dQryyw","expires_in":30,"refresh_expires_in":120,"refresh_token":"eyJhbGciOiJIUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICI2MTU4MWI2MC04YjcyLTQyZmQtYmVmZi1hYzE1MDQ2OTE3YjIifQ.%s.FrTC4LVQWN9o_oqKpSNae-1wIqGeks_IL0XsTQWlC_w","token_type":"Bearer","id_token":"eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJrZV96VGhKU1hoMDM0bEt3Q1BBeDJQWmltdlA4QnhYRS01c1h2Vng0cklnIn0.%s.vQ2RNFPA0Ogdk12zDGIEymZp3ARF8XBue8zVgueoWoP0JvF0_aEkObwNP3zR0ETKW1fxAPJQ5BGbZl2b0j0Zfi0NIJQ0HluSMJSIes9G2eW_Tvdh8_Q8uknEbrPn6l9kDqJ_5bRaahHYWMEieEvdRuyGKdMPAJrMD5_wC9oKARZjgPP9-B_BYcVevQJCucXARq4kLrfz8nrF4DG_dDQHoUrbkjejTd1jTCKJIltO8nN_RlSLGgPK4icroVzuOj_u3PqJzEHqeU4XIPYBhL54E-KsyuaJrnrnSIoLEqA0EomO6bmwzAbDG8XSTpTMaj8ZY426nxvwgDKXV41ZJgQ7ew","not-before-policy":0,"session_state":"0c6609ff-9e06-4bbe-8910-cbf7471a9c65","scope":"openid profile email"}`

func tokenResponse(issuer string) []byte {
	unixTime := time.Now().Unix()

	accessToken := base64.RawStdEncoding.EncodeToString(
		fmt.Appendf(nil, accessTokenFmt, unixTime+30, unixTime, unixTime, issuer),
	)
	refreshToken := base64.RawStdEncoding.EncodeToString(
		fmt.Appendf(nil, refreshTokenFmt, unixTime+300, unixTime, issuer, issuer),
	)
	idToken := base64.RawStdEncoding.EncodeToString(
		fmt.Appendf(nil, idTokenFmt, unixTime+30, unixTime, unixTime, issuer),
	)
	response := fmt.Sprintf(tokenResponseFmt, accessToken, refreshToken, idToken)
	return []byte(response)
}

func testRequest(_ *is.I, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, _ := http.NewRequest(method, ts.URL+path, body)

	resp, _ := http.DefaultClient.Do(req)
	respBody, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	return resp, string(respBody)
}
