package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type CDNirClient struct {
	BaseURL    string
	APIToken   string
	HTTPClient *http.Client
}

type Record struct {
	ID         int    `json:"id"`
	DomainID   int    `json:"domain_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	TTL        string `json:"ttl"`
	Content    string `json:"content"`
	RecordName string `json:"record_name"`
	Actionable bool   `json:"actionable"`
}

type ListRecordsResponse struct {
	StatusCode int      `json:"status_code"`
	Response   []Record `json:"response"`
}

type APIErrorResponse struct {
	Error struct {
		Message    string `json:"message"`
		StatusCode int    `json:"status_code"`
	} `json:"error"`
	Message string `json:"message"`
}

type JWTPayload struct {
	Exp int64 `json:"exp"`
	Iat int64 `json:"iat"`
}

func NewCDNirClient(baseURL, apiToken string) *CDNirClient {
	if baseURL == "" {
		baseURL = "https://bss.asiatech.cloud/api/v3"
	}

	checkTokenExpiry(apiToken)

	return &CDNirClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIToken:   apiToken,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func checkTokenExpiry(token string) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return
	}

	payloadSegment := parts[1]
	// Add base64 padding
	switch len(payloadSegment) % 4 {
	case 2:
		payloadSegment += "=="
	case 3:
		payloadSegment += "="
	}

	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(payloadSegment)
		if err != nil {
			return
		}
	}

	var jwtPayload JWTPayload
	if err := json.Unmarshal(decoded, &jwtPayload); err == nil && jwtPayload.Exp > 0 {
		expTime := time.Unix(jwtPayload.Exp, 0).UTC()
		now := time.Now().UTC()
		remaining := expTime.Sub(now)

		if remaining <= 0 {
			klog.Errorf("CDN.ir API token is EXPIRED! Expired at %s UTC", expTime.Format(time.RFC3339))
		} else if remaining < 24*time.Hour {
			klog.Warningf("CDN.ir API token will expire soon: %s UTC (remaining: %v)", expTime.Format(time.RFC3339), remaining.Round(time.Minute))
		} else {
			klog.V(2).Infof("CDN.ir API token valid until %s UTC (remaining: %v)", expTime.Format(time.RFC3339), remaining.Round(time.Hour))
		}
	}
}

func (c *CDNirClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := fmt.Sprintf("%s/%s", c.BaseURL, strings.TrimLeft(path, "/"))
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "cert-manager-webhook-cdnir/1.0")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIErrorResponse
		if err := json.Unmarshal(respBody, &apiErr); err == nil {
			if apiErr.Error.Message != "" {
				return respBody, fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Error.Message)
			}
			if apiErr.Message != "" {
				return respBody, fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Message)
			}
		}
		return respBody, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *CDNirClient) ListRecords(zone string, recordType string) ([]Record, error) {
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}
	path := fmt.Sprintf("/domain/%s", zone)
	if recordType != "" {
		path = fmt.Sprintf("%s?type=%s", path, recordType)
	}

	respBytes, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var res ListRecordsResponse
	if err := json.Unmarshal(respBytes, &res); err != nil {
		return nil, fmt.Errorf("failed to parse list records response: %w", err)
	}

	return res.Response, nil
}

func (c *CDNirClient) AddRecord(zone, recordName, recordType, content string, ttl int) error {
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}

	reqBody := map[string]string{
		"domain_name": zone,
		"record_name": recordName,
		"type":        recordType,
		"ttl":         fmt.Sprintf("%d", ttl),
		"content":     strings.Trim(content, "\""),
	}

	_, err := c.doRequest("POST", "/domain/add-record", reqBody)
	return err
}

func (c *CDNirClient) DeleteRecord(zone string, recordID int) error {
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}
	path := fmt.Sprintf("/domain/delete-record/%s/%d", zone, recordID)
	_, err := c.doRequest("DELETE", path, nil)
	return err
}
