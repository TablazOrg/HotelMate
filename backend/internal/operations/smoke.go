package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) smoke(ctx context.Context, config resolvedConfig) (any, error) {
	baseURL, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return nil, failure(ExitInvalid, "invalid smoke base URL", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	checks := []struct {
		path, field, expected string
	}{{"/healthz", "status", "ok"}, {"/readyz", "status", "ready"}, {"/api/v1", "name", "HotelMate API"}}
	results := make([]checkResult, 0, len(checks)+1)
	for _, check := range checks {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String()+check.path, nil)
		response, requestErr := client.Do(request)
		if requestErr != nil {
			results = append(results, checkResult{Name: check.path, Message: requestErr.Error()})
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		_ = response.Body.Close()
		var payload map[string]any
		decodeErr := json.Unmarshal(body, &payload)
		ok := response.StatusCode >= 200 && response.StatusCode < 300 && readErr == nil && decodeErr == nil && payload[check.field] == check.expected
		message := response.Status
		if !ok {
			message = fmt.Sprintf("unexpected response status=%d", response.StatusCode)
		}
		results = append(results, checkResult{Name: check.path, OK: ok, Message: message})
	}
	hotelSlug := config.lookup("SMOKE_HOTEL_SLUG")
	staffEmail := config.lookup("SMOKE_STAFF_EMAIL")
	staffPassword := config.lookup("SMOKE_STAFF_PASSWORD")
	if hotelSlug != "" || staffEmail != "" || staffPassword != "" {
		if hotelSlug == "" || staffEmail == "" || staffPassword == "" {
			return nil, failure(ExitInvalid, "set all SMOKE_HOTEL_SLUG, SMOKE_STAFF_EMAIL, and SMOKE_STAFF_PASSWORD", nil)
		}
		loginBody, _ := json.Marshal(map[string]string{"hotelSlug": hotelSlug, "email": staffEmail, "password": staffPassword})
		loginRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL.String()+"/api/v1/auth/staff/login", bytes.NewReader(loginBody))
		loginRequest.Header.Set("Content-Type", "application/json")
		loginResponse, loginErr := client.Do(loginRequest)
		if loginErr != nil {
			results = append(results, checkResult{Name: "authenticated-login", Message: loginErr.Error()})
		} else {
			var loginPayload struct {
				Token string `json:"token"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(loginResponse.Body, 1<<20)).Decode(&loginPayload)
			_ = loginResponse.Body.Close()
			loginOK := loginResponse.StatusCode == http.StatusOK && decodeErr == nil && loginPayload.Token != ""
			results = append(results, checkResult{Name: "authenticated-login", OK: loginOK, Message: loginResponse.Status})
			if loginOK {
				mePayload, meOK := authenticatedJSON(ctx, client, baseURL.String()+"/api/v1/staff/me", loginPayload.Token)
				_, staffOK := mePayload["staff"].(map[string]any)
				hotel, hotelOK := mePayload["hotel"].(map[string]any)
				meOK = meOK && staffOK && hotelOK
				results = append(results, checkResult{Name: "authenticated-session", OK: meOK, Message: statusMessage(meOK)})
				reportPayload, reportOK := authenticatedJSON(ctx, client, baseURL.String()+"/api/v1/staff/reports/operations", loginPayload.Token)
				report, reportExists := reportPayload["report"].(map[string]any)
				timezone, _ := hotel["timezone"].(string)
				reportTimezone, _ := report["timezone"].(string)
				reportOK = reportOK && reportExists && timezone != "" && reportTimezone == timezone && report["summary"] != nil
				results = append(results, checkResult{Name: "authenticated-report", OK: reportOK, Message: statusMessage(reportOK)})
			}
		}
	}
	data := map[string]any{"baseURL": baseURL.String(), "checks": results}
	for _, result := range results {
		if !result.OK {
			return data, failure(ExitVerification, "smoke verification failed", nil)
		}
	}
	return data, nil
}

func (a *App) waitForSmoke(ctx context.Context, config resolvedConfig, timeout time.Duration) (any, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var lastData any
	var lastErr error
	for {
		lastData, lastErr = a.smoke(ctx, config)
		if lastErr == nil {
			return lastData, nil
		}
		select {
		case <-ctx.Done():
			return lastData, failure(ExitVerification, "smoke readiness wait canceled", ctx.Err())
		case <-deadline.C:
			return lastData, lastErr
		case <-ticker.C:
		}
	}
}

func authenticatedJSON(ctx context.Context, client *http.Client, target, token string) (map[string]any, bool) {
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return nil, false
	}
	defer response.Body.Close()
	payload := map[string]any{}
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload)
	return payload, err == nil && response.StatusCode >= 200 && response.StatusCode < 300
}

func statusMessage(ok bool) string {
	if ok {
		return "verified"
	}
	return "verification failed"
}

func (a *App) acceptance(ctx context.Context, config resolvedConfig) (any, error) {
	script := filepath.Join(a.WorkingDir, "scripts", "acceptance.sh")
	if _, err := os.Stat(script); err != nil {
		return nil, failure(ExitPrecondition, "acceptance runner is unavailable", err)
	}
	var stdout, stderr bytes.Buffer
	if err := a.Executor.Run(ctx, script, []string{config.BaseURL}, nil, nil, &stdout, &stderr); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return map[string]any{"baseURL": config.BaseURL}, failure(ExitVerification, "acceptance verification failed", fmt.Errorf("%s", message))
	}
	return map[string]any{"baseURL": config.BaseURL, "result": strings.TrimSpace(stdout.String())}, nil
}
