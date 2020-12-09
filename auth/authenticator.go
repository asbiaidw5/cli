package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"
)

const (
	DefaultBaseURL       = "https://planetscale.us.auth0.com/"
	formMediaType        = "application/x-www-form-urlencoded"
	jsonMediaType        = "application/json"
	DefaultAudienceURL   = "https://bb-test-api.planetscale.com"
	DefaultOAuthClientID = "ZK3V2a5UERfOlWxi5xRXrZZFmvhnf1vg"
)

// Authenticator is the interface for authentication via device oauth
type Authenticator interface {
	VerifyDevice(ctx context.Context, oauthClientID string, audienceURL string) (*DeviceVerification, error)
	GetAccessTokenForDevice(ctx context.Context, v *DeviceVerification, clientID string) (string, error)
}

var _ Authenticator = (*DeviceAuthenticator)(nil)

// DeviceCodeResponse encapsulates the response for obtaining a device code.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationCompleteURI string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	PollingInterval         int    `json:"interval"`
}

// DeviceVerification represents the response from verifying a device.
type DeviceVerification struct {
	DeviceCode              string
	UserCode                string
	VerificationURL         string
	VerificationCompleteURL string
	CheckInterval           time.Duration
	ExpiresAt               time.Time
}

// ErrorResponse is an error response from the API.
type ErrorResponse struct {
	ErrorCode   string `json:"error"`
	Description string `json:"error_description"`
}

func (e ErrorResponse) Error() string {
	return e.Description
}

// DeviceAuthenticator performs the authentication flow for logging in.
type DeviceAuthenticator struct {
	client  *http.Client
	BaseURL *url.URL
}

// New returns an instance of the DeviceAuthenticator
func New(client *http.Client) (*DeviceAuthenticator, error) {
	if client == nil {
		client = cleanhttp.DefaultClient()
	}

	baseURL, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return nil, err
	}
	return &DeviceAuthenticator{
		client:  client,
		BaseURL: baseURL,
	}, nil
}

// VerifyDevice performs the device verification API calls.
func (d *DeviceAuthenticator) VerifyDevice(ctx context.Context, clientID string, audienceURL string) (*DeviceVerification, error) {
	payload := strings.NewReader(fmt.Sprintf("client_id=%s&scope=profile,email,read:databases,write:databases&audience=%s", clientID, audienceURL))
	req, err := d.NewFormRequest(ctx, http.MethodPost, "oauth/device/code", payload)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if err = checkErrorResponse(res); err != nil {
		return nil, err
	}

	deviceCodeRes := &DeviceCodeResponse{}
	err = json.NewDecoder(res.Body).Decode(deviceCodeRes)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding device code response")
	}

	checkInterval := time.Duration(deviceCodeRes.PollingInterval) * time.Second
	expiresAt := time.Now().Add(time.Duration(deviceCodeRes.ExpiresIn) * time.Second)

	return &DeviceVerification{
		DeviceCode:              deviceCodeRes.DeviceCode,
		UserCode:                deviceCodeRes.UserCode,
		VerificationCompleteURL: deviceCodeRes.VerificationCompleteURI,
		VerificationURL:         deviceCodeRes.VerificationURI,
		ExpiresAt:               expiresAt,
		CheckInterval:           checkInterval,
	}, nil
}

// GetAccessTokenForDevice uses the device verification response to fetch an
// access token.
func (d *DeviceAuthenticator) GetAccessTokenForDevice(ctx context.Context, v *DeviceVerification, clientID string) (string, error) {
	var accessToken string
	var err error

	for {
		time.Sleep(v.CheckInterval)
		accessToken, err = d.requestToken(ctx, v.DeviceCode, clientID)
		if accessToken == "" && err == nil {
			if time.Now().After(v.ExpiresAt) {
				err = errors.New("authentication timed out")
			} else {
				continue
			}
		}

		break
	}
	return accessToken, err
}

// OAuthTokenResponse contains the information returned after fetching an access
// token for a device.
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (d *DeviceAuthenticator) requestToken(ctx context.Context, deviceCode string, clientID string) (string, error) {
	payload := strings.NewReader(fmt.Sprintf("grant_type=urn%%3Aietf%%3Aparams%%3Aoauth%%3Agrant-type%%3Adevice_code&device_code=%s&client_id=%s", deviceCode, clientID))
	req, err := d.NewFormRequest(ctx, http.MethodPost, "oauth/token", payload)
	if err != nil {
		return "", errors.Wrap(err, "error creating request")
	}

	res, err := d.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "error performing http request")
	}

	defer res.Body.Close()

	if err = checkErrorResponse(res); err != nil {
		return "", err
	}

	tokenRes := &OAuthTokenResponse{}

	err = json.NewDecoder(res.Body).Decode(tokenRes)
	if err != nil {
		return "", errors.Wrap(err, "error decoding token response")
	}

	return tokenRes.AccessToken, nil
}

// NewFormRequest creates a new form URL encoded request
func (d *DeviceAuthenticator) NewFormRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	u, err := d.BaseURL.Parse(path)
	if err != nil {
		return nil, err
	}

	var req *http.Request
	switch method {
	case http.MethodGet:
		req, err = http.NewRequest(method, u.String(), nil)
		if err != nil {
			return nil, err
		}
	default:
		req, err = http.NewRequest(method, u.String(), body)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Content-Type", formMediaType)
	}

	req.Header.Set("Accept", jsonMediaType)
	req = req.WithContext(ctx)
	return req, nil
}

func checkErrorResponse(res *http.Response) error {
	if res.StatusCode >= 400 {
		errorRes := &ErrorResponse{}
		err := json.NewDecoder(res.Body).Decode(errorRes)
		if err != nil {
			return errors.Wrap(err, "error decoding token response")
		}

		// If we're polling and haven't authorized yet or we need to slow down, we
		// don't wanna terminate the polling
		if errorRes.ErrorCode == "authorization_pending" || errorRes.ErrorCode == "slow_down" {
			return nil
		}

		return errorRes
	}

	return nil
}
