package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Content types
	ContentTypeJSON   = "application/json"
	ContentTypeXML    = "application/xml"
	ContentTypeJS     = "application/javascript"
	ContentTypeNDJSON = "application/x-ndjson"
	ContentTypeText   = "text/"

	// Time conversion constants
	SecondsPerMinute      = 60
	SecondsPerHour        = 3600
	MillisecondsPerSecond = 1000

	// Default timestamp format
	DefaultSRTTime = "00:00:00,000"
)

var textContentTypes = []string{ContentTypeText, ContentTypeJSON, ContentTypeXML, ContentTypeJS, ContentTypeNDJSON}

func IsHTTPText(header http.Header) bool {
	if contentType := header.Get("Content-Type"); contentType != "" {
		ct := strings.ToLower(contentType)
		for _, t := range textContentTypes {
			if strings.Contains(ct, t) {
				return true
			}
		}
	}
	return false
}

func BodyToString(header http.Header, body []byte) string {
	if IsHTTPText(header) {
		return string(body)
	}
	return fmt.Sprintf("<Binary Data: %d bytes>", len(body))
}

func ParseRequestBody(reqBody []byte) (map[string]interface{}, error) {
	var body map[string]interface{}
	if err := json.Unmarshal(reqBody, &body); err != nil {
		return nil, fmt.Errorf("unmarshal request body: %w", err)
	}
	return body, nil
}

func BuildGetRequestURL(baseURL string, body []byte) (string, error) {
	queryParams := make(map[string][]string)
	if err := json.Unmarshal(body, &queryParams); err != nil {
		return "", err
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	q := u.Query()
	for key, values := range queryParams {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
