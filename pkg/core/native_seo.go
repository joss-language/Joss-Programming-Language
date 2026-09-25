package core

import (
	"fmt"
	"html"
	"path/filepath"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

// executeSEOMethod handles SEO class methods
func (r *Runtime) executeSEOMethod(instance *Instance, method string, args []interface{}) interface{} {
	if r.SEO == nil {
		r.SEO = &SEOData{
			Meta: make(map[string]string),
			OG:   make(map[string]string),
		}
	}

	switch method {
	case "title":
		if len(args) >= 1 {
			r.SEO.Title = fmt.Sprintf("%v", args[0])
		}
		return nil
	case "description":
		if len(args) >= 1 {
			r.SEO.Description = fmt.Sprintf("%v", args[0])
		}
		return nil
	case "keywords":
		if len(args) >= 1 {
			switch v := args[0].(type) {
			case string:
				r.SEO.Keywords = strings.Split(v, ",")
			case []interface{}:
				for _, item := range v {
					r.SEO.Keywords = append(r.SEO.Keywords, fmt.Sprintf("%v", item))
				}
			}
		}
		return nil
	case "canonical":
		if len(args) >= 1 {
			r.SEO.Canonical = fmt.Sprintf("%v", args[0])
		}
		return nil
	case "og":
		if len(args) >= 2 {
			prop := fmt.Sprintf("%v", args[0])
			content := fmt.Sprintf("%v", args[1])
			r.SEO.OG[prop] = content
		}
		return nil
	case "meta":
		if len(args) >= 2 {
			name := fmt.Sprintf("%v", args[0])
			content := fmt.Sprintf("%v", args[1])
			r.SEO.Meta[name] = content
		}
		return nil
	case "render":
		return r.RenderSEOTags()
	}

	return nil
}

// RenderSEOTags generates the HTML block for <head>
func (r *Runtime) RenderSEOTags() string {
	if r.SEO == nil {
		return ""
	}

	var sb strings.Builder

	// Title
	if r.SEO.Title != "" {
		sb.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(r.SEO.Title)))
	}

	// Description
	if r.SEO.Description != "" {
		sb.WriteString(fmt.Sprintf("<meta name=\"description\" content=\"%s\">\n", html.EscapeString(r.SEO.Description)))
	}

	// Keywords
	if len(r.SEO.Keywords) > 0 {
		sb.WriteString(fmt.Sprintf("<meta name=\"keywords\" content=\"%s\">\n", html.EscapeString(strings.Join(r.SEO.Keywords, ", "))))
	}

	// Canonical
	if r.SEO.Canonical != "" {
		sb.WriteString(fmt.Sprintf("<link rel=\"canonical\" href=\"%s\">\n", html.EscapeString(r.SEO.Canonical)))
	}

	// Standard Meta
	for name, content := range r.SEO.Meta {
		sb.WriteString(fmt.Sprintf("<meta name=\"%s\" content=\"%s\">\n", html.EscapeString(name), html.EscapeString(content)))
	}

	// Open Graph
	for prop, content := range r.SEO.OG {
		// Ensure og: prefix
		p := prop
		if !strings.HasPrefix(p, "og:") {
			p = "og:" + p
		}
		sb.WriteString(fmt.Sprintf("<meta property=\"%s\" content=\"%s\">\n", html.EscapeString(p), html.EscapeString(content)))
	}

	// Automatic OG Title/Desc/URL if missing
	if _, ok := r.SEO.OG["og:title"]; !ok && r.SEO.Title != "" {
		sb.WriteString(fmt.Sprintf("<meta property=\"og:title\" content=\"%s\">\n", html.EscapeString(r.SEO.Title)))
	}
	if _, ok := r.SEO.OG["og:description"]; !ok && r.SEO.Description != "" {
		sb.WriteString(fmt.Sprintf("<meta property=\"og:description\" content=\"%s\">\n", html.EscapeString(r.SEO.Description)))
	}

	// Twitter Card (Default)
	sb.WriteString("<meta name=\"twitter:card\" content=\"summary_large_image\">\n")

	return sb.String()
}

// executeSitemapMethod handles Sitemap class methods
func (r *Runtime) executeSitemapMethod(instance *Instance, method string, args []interface{}) interface{} {
	switch method {
	case "add":
		if len(args) >= 1 {
			// Check if map/object passed
			if m, ok := args[0].(map[string]interface{}); ok {
				entry := SitemapEntry{
					URL:        fmt.Sprintf("%v", m["url"]),
					LastMod:    "",
					ChangeFreq: "weekly",
					Priority:   0.5,
				}
				if val, exists := m["lastmod"]; exists && val != nil {
					entry.LastMod = fmt.Sprintf("%v", val)
				}
				if val, exists := m["changefreq"]; exists && val != nil {
					entry.ChangeFreq = fmt.Sprintf("%v", val)
				}
				if val, exists := m["priority"]; exists && val != nil {
					switch pv := val.(type) {
					case float64:
						entry.Priority = pv
					case int:
						entry.Priority = float64(pv)
					case int64:
						entry.Priority = float64(pv)
					}
				}
				r.SitemapEntries = append(r.SitemapEntries, entry)
				return nil
			}

			entry := SitemapEntry{
				URL:        fmt.Sprintf("%v", args[0]),
				LastMod:    "",
				ChangeFreq: "weekly",
				Priority:   0.5,
			}
			if len(args) >= 2 && args[1] != nil {
				entry.LastMod = fmt.Sprintf("%v", args[1])
			}
			if len(args) >= 3 && args[2] != nil {
				entry.ChangeFreq = fmt.Sprintf("%v", args[2])
			}
			if len(args) >= 4 && args[3] != nil {
				switch p := args[3].(type) {
				case float64:
					entry.Priority = p
				case int:
					entry.Priority = float64(p)
				case int64:
					entry.Priority = float64(p)
				}
			}
			r.SitemapEntries = append(r.SitemapEntries, entry)
		}
		return nil

	case "provider":
		if len(args) >= 1 {
			switch fn := args[0].(type) {
			case *parser.FunctionLiteral:
				captured := r.captureFunction(fn)
				r.SitemapProviders = append(r.SitemapProviders, captured)
			case *CapturedFunction:
				r.SitemapProviders = append(r.SitemapProviders, fn)
			}
		}
		return nil

	case "exclude":
		if len(args) >= 1 {
			switch v := args[0].(type) {
			case string:
				r.SitemapExclusions = append(r.SitemapExclusions, v)
			case []interface{}:
				for _, item := range v {
					r.SitemapExclusions = append(r.SitemapExclusions, fmt.Sprintf("%v", item))
				}
			case []string:
				r.SitemapExclusions = append(r.SitemapExclusions, v...)
			}
		}
		return nil

	case "xsl":
		return r.GenerateSitemapXSL()

	case "generate":
		baseUrl := ""
		if reqVal, ok := r.Variables["$__request"]; ok {
			if reqInstance, ok := reqVal.(*Instance); ok {
				scheme := "http"
				if s, ok := reqInstance.Fields["_scheme"].(string); ok {
					scheme = s
				}
				host := "localhost"
				if h, ok := reqInstance.Fields["_host"].(string); ok {
					host = h
				}
				baseUrl = scheme + "://" + host
			}
		}
		return r.GenerateSitemapXML(baseUrl)
	}
	return nil
}

// GenerateSitemapXML builds the full sitemap.xml buffer
func (r *Runtime) GenerateSitemapXML(baseUrl string) string {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<?xml-stylesheet type=\"text/xsl\" href=\"/sitemap.xsl\"?>\n")
	sb.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")

	isExcluded := func(p string) bool {
		clean := strings.TrimSpace(p)
		for _, excl := range r.SitemapExclusions {
			excl = strings.TrimSpace(excl)
			if excl == clean {
				return true
			}
			if strings.HasSuffix(excl, "*") {
				prefix := strings.TrimSuffix(excl, "*")
				if strings.HasPrefix(clean, prefix) {
					return true
				}
			}
		}
		// Default system exclusions
		if strings.HasPrefix(clean, "/api/") || strings.HasPrefix(clean, "/admin/") || strings.HasPrefix(clean, "/.") {
			return true
		}
		if clean == "/sitemap" || clean == "/sitemap.xml" || clean == "/sitemap.xsl" || clean == "/ads.txt" {
			return true
		}
		if strings.HasSuffix(clean, ".json") || strings.HasSuffix(clean, ".ps1") || strings.HasSuffix(clean, ".sh") {
			return true
		}
		return false
	}

	normalizePath := func(raw string) string {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			if idx := strings.Index(trimmed, "://"); idx != -1 {
				rest := trimmed[idx+3:]
				if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
					trimmed = rest[slashIdx:]
				} else {
					trimmed = "/"
				}
			}
		}
		if !strings.HasPrefix(trimmed, "/") {
			trimmed = "/" + trimmed
		}
		if len(trimmed) > 1 && strings.HasSuffix(trimmed, "/") {
			trimmed = strings.TrimSuffix(trimmed, "/")
		}
		return trimmed
	}

	visited := make(map[string]bool)

	addEntry := func(urlStr, lastMod, changeFreq string, priority float64) {
		norm := normalizePath(urlStr)
		if isExcluded(norm) || visited[norm] {
			return
		}
		visited[norm] = true
		r.writeSitemapEntry(&sb, urlStr, lastMod, changeFreq, priority, baseUrl)
	}

	// 1. Dynamic Providers (Closures registered via Sitemap::provider(func() { ... }))
	for _, provider := range r.SitemapProviders {
		res := r.callCapturedFunction(provider, []interface{}{})
		if list, ok := res.([]interface{}); ok {
			for _, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					urlStr := fmt.Sprintf("%v", m["url"])
					lastMod := ""
					if val, exists := m["lastmod"]; exists && val != nil {
						lastMod = fmt.Sprintf("%v", val)
					}
					changeFreq := "weekly"
					if val, exists := m["changefreq"]; exists && val != nil {
						changeFreq = fmt.Sprintf("%v", val)
					}
					priority := 0.8
					if val, exists := m["priority"]; exists && val != nil {
						switch pv := val.(type) {
						case float64:
							priority = pv
						case int:
							priority = float64(pv)
						case int64:
							priority = float64(pv)
						}
					}
					addEntry(urlStr, lastMod, changeFreq, priority)
				}
			}
		}
	}

	// 2. Manual Entries (Registered via Sitemap::add)
	for _, entry := range r.SitemapEntries {
		addEntry(entry.URL, entry.LastMod, entry.ChangeFreq, entry.Priority)
	}

	// 3. Automatic Routes from routes.joss (Auto-discovery for any remaining public GET pages)
	if getRoutes, ok := r.Routes["GET"]; ok {
		for path, infoVal := range getRoutes {
			if info, ok := infoVal.(map[string]interface{}); ok {
				source, _ := info["source"].(string)
				middleware, _ := info["middleware"].([]string)

				isRoutesSource := source == "routes" || source == "routes.joss" || strings.HasSuffix(filepath.ToSlash(source), "routes.joss")
				if isRoutesSource && len(middleware) == 0 {
					if !strings.Contains(path, ":") && !strings.Contains(path, "{") {
						p := 0.8
						freq := "weekly"
						if path == "/" {
							p = 1.0
							freq = "daily"
						} else if path == "/blog" || path == "/shop" || path == "/pub" {
							p = 0.9
							freq = "daily"
						}
						addEntry(path, "", freq, p)
					}
				}
			}
		}
	}

	sb.WriteString("</urlset>")
	return sb.String()
}

// GenerateSitemapXSL provides a modern, beautiful, responsive interactive dashboard for the sitemap
func (r *Runtime) GenerateSitemapXSL() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="2.0" 
    xmlns:html="http://www.w3.org/TR/REC-html40"
    xmlns:sitemap="http://www.sitemaps.org/schemas/sitemap/0.9"
    xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="html" version="1.0" encoding="UTF-8" indent="yes"/>
    <xsl:template match="/">
        <html lang="es">
            <head>
                <meta charset="UTF-8"/>
                <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
                <title>XML Sitemap | Joss Engine</title>
                <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.5.1/css/all.min.css"/>
                <style>
                    :root {
                        --bg-gradient: linear-gradient(135deg, #090514 0%, #170d2b 50%, #0c0818 100%);
                        --card-bg: rgba(255, 255, 255, 0.05);
                        --card-border: rgba(255, 255, 255, 0.1);
                        --primary: #ff4f5f;
                        --primary-glow: rgba(255, 79, 95, 0.35);
                        --accent: #1bb7a5;
                        --accent-glow: rgba(27, 183, 165, 0.35);
                        --text: #f8fafc;
                        --text-muted: #94a3b8;
                    }
                    * { box-sizing: border-box; margin: 0; padding: 0; }
                    body {
                        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
                        background: var(--bg-gradient);
                        background-attachment: fixed;
                        color: var(--text);
                        min-height: 100vh;
                        padding: 3rem 1.5rem;
                    }
                    .container {
                        max-width: 1200px;
                        margin: 0 auto;
                    }
                    .header {
                        background: var(--card-bg);
                        backdrop-filter: blur(20px);
                        -webkit-backdrop-filter: blur(20px);
                        border: 1px solid var(--card-border);
                        border-radius: 20px;
                        padding: 2.5rem;
                        margin-bottom: 2rem;
                        box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
                        display: flex;
                        justify-content: space-between;
                        align-items: center;
                        flex-wrap: wrap;
                        gap: 1.5rem;
                    }
                    .brand {
                        display: flex;
                        align-items: center;
                        gap: 1rem;
                    }
                    .logo-icon {
                        width: 54px;
                        height: 54px;
                        background: linear-gradient(135deg, var(--primary), var(--accent));
                        border-radius: 14px;
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        font-size: 1.75rem;
                        color: #ffffff;
                        box-shadow: 0 10px 25px var(--primary-glow);
                    }
                    h1 {
                        font-size: 1.85rem;
                        font-weight: 900;
                        letter-spacing: -0.5px;
                        background: linear-gradient(to right, #ffffff, #cbd5e1);
                        -webkit-background-clip: text;
                        -webkit-text-fill-color: transparent;
                    }
                    .subtitle {
                        color: var(--text-muted);
                        font-size: 0.95rem;
                        margin-top: 0.25rem;
                    }
                    .stats {
                        display: flex;
                        gap: 1rem;
                    }
                    .stat-badge {
                        background: rgba(255, 255, 255, 0.04);
                        border: 1px solid var(--card-border);
                        padding: 0.75rem 1.25rem;
                        border-radius: 12px;
                        text-align: center;
                    }
                    .stat-value {
                        font-size: 1.4rem;
                        font-weight: 800;
                        color: var(--primary);
                    }
                    .stat-label {
                        font-size: 0.75rem;
                        text-transform: uppercase;
                        font-weight: 700;
                        color: var(--text-muted);
                        letter-spacing: 0.5px;
                    }
                    .search-bar {
                        margin-bottom: 1.5rem;
                    }
                    .search-input {
                        width: 100%;
                        background: var(--card-bg);
                        backdrop-filter: blur(16px);
                        border: 1px solid var(--card-border);
                        border-radius: 14px;
                        padding: 1rem 1.25rem 1rem 3rem;
                        color: #ffffff;
                        font-size: 1rem;
                        outline: none;
                        transition: border 0.3s, box-shadow 0.3s;
                    }
                    .search-input:focus {
                        border-color: var(--primary);
                        box-shadow: 0 0 20px var(--primary-glow);
                    }
                    .table-card {
                        background: var(--card-bg);
                        backdrop-filter: blur(20px);
                        -webkit-backdrop-filter: blur(20px);
                        border: 1px solid var(--card-border);
                        border-radius: 20px;
                        overflow: hidden;
                        box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
                    }
                    table {
                        width: 100%;
                        border-collapse: collapse;
                        text-align: left;
                    }
                    th {
                        background: rgba(255, 255, 255, 0.04);
                        padding: 1.2rem 1.5rem;
                        font-size: 0.8rem;
                        font-weight: 800;
                        text-transform: uppercase;
                        letter-spacing: 0.75px;
                        color: var(--text-muted);
                        border-bottom: 1px solid var(--card-border);
                    }
                    td {
                        padding: 1.1rem 1.5rem;
                        font-size: 0.92rem;
                        border-bottom: 1px solid rgba(255, 255, 255, 0.04);
                        transition: background 0.2s ease;
                    }
                    tr:hover td {
                        background: rgba(255, 255, 255, 0.03);
                    }
                    .url-link {
                        color: #60a5fa;
                        text-decoration: none;
                        font-weight: 600;
                        transition: color 0.2s;
                        word-break: break-all;
                    }
                    .url-link:hover {
                        color: var(--primary);
                        text-decoration: underline;
                    }
                    .badge {
                        display: inline-block;
                        padding: 0.25rem 0.65rem;
                        border-radius: 6px;
                        font-size: 0.75rem;
                        font-weight: 800;
                        text-transform: uppercase;
                    }
                    .badge-freq {
                        background: rgba(27, 183, 165, 0.12);
                        color: #2dd4bf;
                        border: 1px solid rgba(27, 183, 165, 0.25);
                    }
                    .badge-priority {
                        background: rgba(255, 79, 95, 0.12);
                        color: #ff4f5f;
                        border: 1px solid rgba(255, 79, 95, 0.25);
                    }
                    .footer {
                        text-align: center;
                        margin-top: 2.5rem;
                        font-size: 0.85rem;
                        color: var(--text-muted);
                    }
                </style>
            </head>
            <body>
                <div class="container">
                    <div class="header">
                        <div class="brand">
                            <div class="logo-icon">
                                <i class="fa-solid fa-sitemap"></i>
                            </div>
                            <div>
                                <h1>Mapa del Sitio XML</h1>
                                <p class="subtitle">Generado en vivo y dinámicamente por el motor de servidor de Joss</p>
                            </div>
                        </div>
                        <div class="stats">
                            <div class="stat-badge">
                                <div class="stat-value">
                                    <xsl:value-of select="count(sitemap:urlset/sitemap:url)"/>
                                </div>
                                <div class="stat-label">URLs Indexadas</div>
                            </div>
                        </div>
                    </div>

                    <div class="table-card">
                        <table>
                            <thead>
                                <tr>
                                    <th style="width: 50%;">Ubicación (URL)</th>
                                    <th>Prioridad</th>
                                    <th>Frecuencia</th>
                                    <th>Última Modificación</th>
                                </tr>
                            </thead>
                            <tbody>
                                <xsl:for-each select="sitemap:urlset/sitemap:url">
                                    <tr>
                                        <td>
                                            <a class="url-link">
                                                <xsl:attribute name="href">
                                                    <xsl:value-of select="sitemap:loc"/>
                                                </xsl:attribute>
                                                <xsl:value-of select="sitemap:loc"/>
                                            </a>
                                        </td>
                                        <td>
                                            <span class="badge badge-priority">
                                                <xsl:value-of select="sitemap:priority"/>
                                            </span>
                                        </td>
                                        <td>
                                            <span class="badge badge-freq">
                                                <xsl:value-of select="sitemap:changefreq"/>
                                            </span>
                                        </td>
                                        <td style="color: #94a3b8; font-size: 0.85rem;">
                                            <xsl:choose>
                                                <xsl:when test="sitemap:lastmod">
                                                    <xsl:value-of select="sitemap:lastmod"/>
                                                </xsl:when>
                                                <xsl:otherwise>
                                                    <span style="opacity: 0.4;">—</span>
                                                </xsl:otherwise>
                                            </xsl:choose>
                                        </td>
                                    </tr>
                                </xsl:for-each>
                            </tbody>
                        </table>
                    </div>

                    <div class="footer">
                        <p>Desarrollado con <i class="fa-solid fa-heart" style="color:#ff4f5f;"></i> por Joss Programming Language Engine</p>
                    </div>
                </div>
            </body>
        </html>
    </xsl:template>
</xsl:stylesheet>`
}

func formatSitemapLastMod(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Try standard date formats
	formats := []string{
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}

	// Fallback: if it starts with YYYY-MM-DD
	if len(raw) >= 10 && raw[4] == '-' && raw[7] == '-' {
		return raw[:10]
	}

	return ""
}

func (r *Runtime) writeSitemapEntry(sb *strings.Builder, urlPath, lastMod, freq string, priority float64, dynamicBase string) {
	appUrl := dynamicBase
	if appUrl == "" {
		if val, ok := r.Env["APP_URL"]; ok {
			appUrl = strings.TrimSuffix(val, "/")
		} else {
			appUrl = "http://localhost"
		}
	}

	fullUrl := appUrl + urlPath
	if !strings.HasPrefix(urlPath, "/") && !strings.HasPrefix(urlPath, "http") {
		fullUrl = appUrl + "/" + urlPath
	} else if strings.HasPrefix(urlPath, "http") {
		fullUrl = urlPath
	}

	formattedDate := formatSitemapLastMod(lastMod)

	sb.WriteString("  <url>\n")
	sb.WriteString(fmt.Sprintf("    <loc>%s</loc>\n", html.EscapeString(fullUrl)))
	if formattedDate != "" {
		sb.WriteString(fmt.Sprintf("    <lastmod>%s</lastmod>\n", html.EscapeString(formattedDate)))
	}
	sb.WriteString(fmt.Sprintf("    <changefreq>%s</changefreq>\n", html.EscapeString(freq)))
	sb.WriteString(fmt.Sprintf("    <priority>%.1f</priority>\n", priority))
	sb.WriteString("  </url>\n")
}
