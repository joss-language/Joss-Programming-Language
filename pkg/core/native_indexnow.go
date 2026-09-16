package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// IndexNowPayload structure for IndexNow API
type IndexNowPayload struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation,omitempty"`
	URLList     []string `json:"urlList"`
}

// GetIndexNowKey returns the active IndexNow key (from Runtime, Env, or os.Getenv)
func (r *Runtime) GetIndexNowKey() string {
	if r.IndexNowKey != "" {
		return r.IndexNowKey
	}
	if val, ok := r.Env["INDEXNOW_KEY"]; ok && strings.TrimSpace(val) != "" {
		r.IndexNowKey = strings.TrimSpace(val)
		return r.IndexNowKey
	}
	if envVal := os.Getenv("INDEXNOW_KEY"); strings.TrimSpace(envVal) != "" {
		r.IndexNowKey = strings.TrimSpace(envVal)
		return r.IndexNowKey
	}
	return ""
}

func (r *Runtime) resolveDynamicBaseUrl(fallback string) string {
	if r.SEO != nil && r.SEO.Canonical != "" {
		if parsed, err := url.Parse(r.SEO.Canonical); err == nil && parsed.Host != "" {
			return parsed.Scheme + "://" + parsed.Host
		}
	}

	if appUrl, ok := r.Env["APP_URL"]; ok && strings.TrimSpace(appUrl) != "" {
		return strings.TrimRight(strings.TrimSpace(appUrl), "/")
	}

	if fallback != "" {
		return strings.TrimRight(fallback, "/")
	}

	return ""
}

func extractRawUrls(arg interface{}) []string {
	var rawUrls []string
	switch v := arg.(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			rawUrls = append(rawUrls, s)
		}
	case []interface{}:
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprintf("%v", item)); s != "" {
				rawUrls = append(rawUrls, s)
			}
		}
	case []string:
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				rawUrls = append(rawUrls, s)
			}
		}
	default:
		if s := strings.TrimSpace(fmt.Sprintf("%v", arg)); s != "" {
			rawUrls = append(rawUrls, s)
		}
	}
	return rawUrls
}

func (r *Runtime) resolveUrlsAndHost(rawUrls []string, baseUrl string) ([]string, string) {
	detectedHost := hostFromUrl(baseUrl)

	var absoluteUrls []string
	for _, uStr := range rawUrls {
		absUrl, host := resolveSingleUrl(uStr, baseUrl)
		absoluteUrls = append(absoluteUrls, absUrl)
		if detectedHost == "" && host != "" {
			detectedHost = host
		}
	}

	if detectedHost == "" {
		detectedHost = hostFromUrl(r.Env["APP_URL"])
	}

	return absoluteUrls, detectedHost
}

func hostFromUrl(raw string) string {
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil {
		return parsed.Host
	}
	return ""
}

func resolveSingleUrl(uStr, baseUrl string) (string, string) {
	if strings.HasPrefix(uStr, "http://") || strings.HasPrefix(uStr, "https://") {
		return uStr, hostFromUrl(uStr)
	}
	rel := "/" + strings.TrimLeft(uStr, "/")
	if baseUrl != "" {
		return baseUrl + rel, ""
	}
	return rel, ""
}

func (r *Runtime) handleSubmitIndexNow(args []interface{}) bool {
	if len(args) < 1 || args[0] == nil {
		return false
	}

	key := r.GetIndexNowKey()
	if len(args) >= 2 && args[1] != nil && strings.TrimSpace(fmt.Sprintf("%v", args[1])) != "" {
		key = strings.TrimSpace(fmt.Sprintf("%v", args[1]))
	}
	if key == "" {
		return false
	}

	rawUrls := extractRawUrls(args[0])
	if len(rawUrls) == 0 {
		return false
	}

	baseUrl := r.resolveDynamicBaseUrl("")
	absoluteUrls, detectedHost := r.resolveUrlsAndHost(rawUrls, baseUrl)

	keyLocation := ""
	if baseUrl != "" && key != "" {
		keyLocation = baseUrl + "/" + key + ".txt"
	}

	return r.sendIndexNowRequest(IndexNowPayload{
		Host:        detectedHost,
		Key:         key,
		KeyLocation: keyLocation,
		URLList:     absoluteUrls,
	})
}

// executeIndexNowMethod handles IndexNow class methods
func (r *Runtime) executeIndexNowMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "key":
		if len(args) >= 1 && args[0] != nil {
			r.IndexNowKey = strings.TrimSpace(fmt.Sprintf("%v", args[0]))
			return r.IndexNowKey
		}
		return r.GetIndexNowKey()

	case "enabled":
		return r.GetIndexNowKey() != ""

	case "keyLocation":
		key := r.GetIndexNowKey()
		if key == "" {
			return ""
		}
		var baseUrl string
		if len(args) >= 1 && args[0] != nil {
			baseUrl = strings.TrimRight(fmt.Sprintf("%v", args[0]), "/")
		} else {
			baseUrl = r.resolveDynamicBaseUrl("")
		}
		if baseUrl == "" {
			return "/" + key + ".txt"
		}
		return baseUrl + "/" + key + ".txt"

	case "submit":
		return r.handleSubmitIndexNow(args)
	}

	return nil
}

func (r *Runtime) sendIndexNowRequest(payload IndexNowPayload) bool {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return false
	}

	client := &http.Client{
		Timeout: 7 * time.Second,
	}

	req, err := http.NewRequest("POST", "https://api.indexnow.org/indexnow", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted
}
