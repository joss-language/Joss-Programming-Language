package server

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/jossecurity/joss/pkg/core"
)

func handleEarlySpecialRoutes(w http.ResponseWriter, r *http.Request, rt *core.Runtime, baseUrl string) bool {
	if r.URL.Path == "/favicon.ico" {
		handleFavicon(w, r)
		return true
	}

	if r.URL.Path == "/sitemap.xml" {
		w.Header().Set(headerContentType, "application/xml; charset=utf-8")
		fmt.Fprintf(w, "%s", rt.GenerateSitemapXML(baseUrl))
		return true
	}

	if r.URL.Path == "/sitemap.xsl" {
		w.Header().Set(headerContentType, "application/xslt+xml; charset=utf-8")
		w.Header().Set(headerCacheControl, headerCacheControlPublic)
		fmt.Fprintf(w, "%s", rt.GenerateSitemapXSL())
		return true
	}

	if indexNowKey := rt.GetIndexNowKey(); indexNowKey != "" && r.URL.Path == "/"+indexNowKey+".txt" {
		w.Header().Set(headerContentType, "text/plain; charset=utf-8")
		w.Header().Set(headerCacheControl, headerCacheControlPublic)
		fmt.Fprint(w, indexNowKey)
		return true
	}

	if strings.HasPrefix(r.URL.Path, "/assets/vendor/") {
		handleVendorAssets(w, r)
		return true
	}

	return false
}

func handleFavicon(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat("public/favicon.ico"); err == nil {
		http.ServeFile(w, r, "public/favicon.ico")
		return
	}
	if _, err := os.Stat("assets/logo.png"); err == nil {
		http.ServeFile(w, r, "assets/logo.png")
		return
	}
	if _, err := os.Stat("assets/logo.ico"); err == nil {
		http.ServeFile(w, r, "assets/logo.ico")
		return
	}
	w.Header().Set(headerContentType, "image/png")
	w.Header().Set(headerCacheControl, headerCacheControlPublic)
	w.Write(DefaultLogo)
}

func handleVendorAssets(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/assets/vendor/")
	fullPath := "node_modules/" + relPath

	if strings.Contains(relPath, "..") {
		http.Error(w, "Invalid path", http.StatusForbidden)
		return
	}

	if _, err := os.Stat(fullPath); err == nil {
		http.ServeFile(w, r, fullPath)
		return
	}
	http.NotFound(w, r)
}

func setupRequestLocale(r *http.Request, rt *core.Runtime) {
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		parts := strings.Split(acceptLang, ",")
		if len(parts) > 0 {
			first := strings.TrimSpace(parts[0])
			first = strings.Split(first, ";")[0]
			rt.SetLocale(first)
			return
		}
	}
	rt.SetLocale("en")
}

func handleCORSHeaders(w http.ResponseWriter, r *http.Request, env map[string]string) bool {
	corsPolicy := env["CORS_WEB"]
	if corsPolicy == "" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	allowOrigin := ""
	if corsPolicy == "*" {
		allowOrigin = "*"
	} else {
		allowed := strings.Split(corsPolicy, ",")
		for _, a := range allowed {
			if strings.TrimSpace(a) == origin {
				allowOrigin = origin
				break
			}
		}
	}

	if allowOrigin == "" {
		return false
	}

	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-CSRF-TOKEN")
	if corsPolicy != "*" {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}
