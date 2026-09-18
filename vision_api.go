package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VisionWebApi is a Go client for the Keysight Vision NPB Web API,
// mirroring the relevant parts of ksvisionlib.VisionWebApi.
type VisionWebApi struct {
	Host     string
	Port     int
	Username string
	Password string
	Debug    bool
	Timeout  time.Duration
	Retries  int

	token      string
	authB64    string
	httpClient *http.Client
	baseURL    string
}

// VisionAPIError represents an HTTP-level error returned by the API
type VisionAPIError struct {
	StatusCode int
	Body       string
}

func (e *VisionAPIError) Error() string {
	return fmt.Sprintf("API error: status=%d body=%s", e.StatusCode, e.Body)
}

// NewVisionWebApi creates a new client and authenticates ("connect"),
// mirroring the Python __init__ behavior (auth happens on construction).
func NewVisionWebApi(host, username, password string, port int, debug bool, timeout time.Duration, retries int) (*VisionWebApi, error) {
	nto := &VisionWebApi{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Debug:    debug,
		Timeout:  timeout,
		Retries:  retries,
		baseURL:  fmt.Sprintf("https://%s:%d", host, port),
	}

	nto.authB64 = base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	nto.httpClient = &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	if err := nto.connect(); err != nil {
		return nil, err
	}

	return nto, nil
}

// connect performs the initial authentication call and stores the token.
// This corresponds to the "connect" function requested.
func (v *VisionWebApi) connect() error {
	var lastErr error

	for attempt := 0; attempt <= v.Retries; attempt++ {
		req, err := http.NewRequest("GET", v.baseURL+"/api/auth", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Basic "+v.authB64)
		req.Header.Set("Content-Type", "application/json")

		resp, err := v.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if v.Debug {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Status=%d\nHeaders=%v\nData=%s\n", resp.StatusCode, resp.Header, string(body))
		}

		token := resp.Header.Get("x-auth-token")
		if token == "" {
			lastErr = fmt.Errorf("authentication failed: no x-auth-token header returned")
			continue
		}

		v.token = token
		return nil
	}

	return fmt.Errorf("connection error after %d retries: %w", v.Retries, lastErr)
}

// sendRequest sends an authenticated request to the Web API.
func (v *VisionWebApi) sendRequest(method, path string, args interface{}, decode bool) (interface{}, error) {
	var bodyBytes []byte
	var err error

	if args != nil {
		bodyBytes, err = json.Marshal(args)
		if err != nil {
			return nil, err
		}
	} else {
		bodyBytes = []byte("null")
	}

	if v.Debug {
		fmt.Printf("Sending request: method=%s url=%s args=%s\n", method, path, string(bodyBytes))
	}

	req, err := http.NewRequest(method, v.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authentication", v.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if v.Debug {
		fmt.Printf("Response: status=%d data=%s\n", resp.StatusCode, string(data))
	}

	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		return nil, &VisionAPIError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return nil, &VisionAPIError{StatusCode: resp.StatusCode, Body: string(data)}
	}

	if !decode || len(data) == 0 {
		return string(data), nil
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		// Some endpoints return empty/non-JSON bodies on success
		return string(data), nil
	}

	return result, nil
}

// ModifySystem updates system properties (PUT /api/system)
func (v *VisionWebApi) ModifySystem(args map[string]interface{}) (interface{}, error) {
	return v.sendRequest("PUT", "/api/system", args, false)
}

// GetSystem retrieves system properties, optionally filtered
func (v *VisionWebApi) GetSystem(properties string) (map[string]interface{}, error) {
	path := "/api/system"
	if properties != "" {
		path += "?properties=" + properties
	}

	result, err := v.sendRequest("GET", path, nil, true)
	if err != nil {
		return nil, err
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type for GetSystem")
	}
	return m, nil
}

// GetSystemProperty fetches a single system property by name
func (v *VisionWebApi) GetSystemProperty(property string) (interface{}, error) {
	sys, err := v.GetSystem(property)
	if err != nil {
		return nil, err
	}
	return sys[property], nil
}

// Logout invalidates the current auth token
func (v *VisionWebApi) Logout() error {
	_, err := v.sendRequest("POST", "/api/auth/logout", map[string]interface{}{}, false)
	if err != nil {
		// try old API style as fallback
		_, err2 := v.sendRequest("GET", "/api/auth/logout", nil, false)
		if err2 != nil {
			return fmt.Errorf("logout failed: %v / %v", err, err2)
		}
	}
	return nil
}
