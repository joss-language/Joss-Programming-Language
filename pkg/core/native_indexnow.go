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
	// 1. If dynamic base is already detected from current HTTP request
	if r.SEO != nil && r.SEO.Canonical != "" {
		if parsed, err := url.Parse(r.SEO.Canonical); err == nil && parsed.Host != "" {
			return parsed.Scheme + "://" + parsed.Host
		}
	}

	// 2. Check APP_URL in Env
	if appUrl, ok := r.Env["APP_URL"]; ok && strings.TrimSpace(appUrl) != "" {
		return strings.TrimRight(strings.TrimSpace(appUrl), "/")
	}

	if fallback != "" {
		return strings.TrimRight(fallback, "/")
	}

	return ""
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

		// Collect URL list
		var rawUrls []string
		switch v := args[0].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				rawUrls = append(rawUrls, strings.TrimSpace(v))
			}
		case []interface{}:
			for _, item := range v {
				if str := strings.TrimSpace(fmt.Sprintf("%v", item)); str != "" {
					rawUrls = append(rawUrls, str)
				}
			}
		case []string:
			for _, item := range v {
				if str := strings.TrimSpace(item); str != "" {
					rawUrls = append(rawUrls, str)
				}
			}
		default:
			str := strings.TrimSpace(fmt.Sprintf("%v", args[0]))
			if str != "" {
				rawUrls = append(rawUrls, str)
			}
		}

		if len(rawUrls) == 0 {
			return false
		}

		baseUrl := r.resolveDynamicBaseUrl("")
		detectedHost := ""
		if baseUrl != "" {
			if u, err := url.Parse(baseUrl); err == nil {
				detectedHost = u.Host
			}
		}

		// Normalize URLs to absolute URLs and determine host
		var absoluteUrls []string
		for _, uStr := range rawUrls {
			if strings.HasPrefix(uStr, "http://") || strings.HasPrefix(uStr, "https://") {
				absoluteUrls = append(absoluteUrls, uStr)
				if detectedHost == "" {
					if parsed, err := url.Parse(uStr); err == nil {
						detectedHost = parsed.Host
					}
				}
			} else {
				// Relative URL
				rel := "/" + strings.TrimLeft(uStr, "/")
				if baseUrl != "" {
					absoluteUrls = append(absoluteUrls, baseUrl+rel)
				} else {
					absoluteUrls = append(absoluteUrls, rel)
				}
			}
		}

		if detectedHost == "" {
			if appUrl, ok := r.Env["APP_URL"]; ok && appUrl != "" {
				if parsed, err := url.Parse(appUrl); err == nil {
					detectedHost = parsed.Host
				}
			}
		}

		keyLocation := ""
		if baseUrl != "" && key != "" {
			keyLocation = baseUrl + "/" + key + ".txt"
		}

		payload := IndexNowPayload{
			Host:        detectedHost,
			Key:         key,
			KeyLocation: keyLocation,
			URLList:     absoluteUrls,
		}

		// Send to IndexNow API
		return r.sendIndexNowRequest(payload)
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
