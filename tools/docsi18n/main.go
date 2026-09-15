// docsi18n keeps the canonical Spanish documentation, its translations and
// the public JosSecurity mirror in sync. Code fences and HTML comments are
// never sent to the translation provider, preserving executable contracts.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	manifestPath = "docs/.i18n-manifest.json"
	publicRoot   = "ejemplos/Joss-Red-JosSecurity/assets/docs"
)

var (
	markdownLink      = regexp.MustCompile(`\[[^]]+\]\(([^)]+)\)`)
	inlineCode        = regexp.MustCompile("`[^`\\r\\n]+`")
	translationSyntax = regexp.MustCompile("`[^`\\r\\n]+`")
	protectedMarkdown = regexp.MustCompile("(?s)<!--.*?-->|```.*?```|~~~.*?~~~")
)

type manifest struct {
	Version      int                          `json:"version"`
	SourceLocale string                       `json:"sourceLocale"`
	Locales      []string                     `json:"locales"`
	Sources      map[string]map[string]string `json:"sources"`
}

type protectedSyntax struct {
	links      []string
	inline     []string
	lineBreaks []string
}

type translateFunc func(string, string) (string, error)

type bingTranslator struct {
	client *http.Client
	ig     string
	iid    string
	key    string
	token  string
}

func main() {
	translate := flag.Bool("translate", false, "translate stale or missing English and Portuguese documents")
	force := flag.Bool("force", false, "translate every document even when its source hash is current")
	fixLinks := flag.Bool("fix-links", false, "normalize translated relative links without calling the translation provider")
	acceptCurrent := flag.Bool("accept-current", false, "accept manually reviewed translations after validating protected Markdown")
	syncPublic := flag.Bool("sync", false, "synchronize all three locales to the JosSecurity public mirror")
	check := flag.Bool("check", false, "verify translation coverage, freshness, links and public mirrors")
	filesFlag := flag.String("files", "", "optional comma-separated canonical Markdown filenames")
	localesFlag := flag.String("locales", "en,pt", "comma-separated target locales: en,pt")
	provider := flag.String("provider", "google", "translation provider: google or bing")
	flag.Parse()

	if !*translate && !*fixLinks && !*acceptCurrent && !*syncPublic && !*check {
		flag.Usage()
		os.Exit(2)
	}

	files := canonicalFiles()
	if *filesFlag != "" {
		files = selectFiles(files, strings.Split(*filesFlag, ","))
	}
	m := readManifest()
	if *translate {
		translator, err := newTranslator(*provider)
		must(err)
		locales := selectLocales(*localesFlag)
		for _, locale := range locales {
			for _, path := range files {
				translateFile(path, locale, m, *force, translator)
				writeManifest(m)
			}
		}
	}
	if *fixLinks {
		fixTranslatedLinks(files)
	}
	if *acceptCurrent {
		acceptManualTranslations(files, selectLocales(*localesFlag), m)
		writeManifest(m)
	}
	if *syncPublic {
		syncMirrors(canonicalFiles())
	}
	if *check {
		if err := checkAll(canonicalFiles(), m); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func selectLocales(value string) []string {
	var locales []string
	seen := map[string]bool{}
	for _, locale := range strings.Split(value, ",") {
		locale = strings.TrimSpace(locale)
		if (locale != "en" && locale != "pt") || seen[locale] {
			must(fmt.Errorf("invalid or duplicate target locale %q", locale))
		}
		seen[locale] = true
		locales = append(locales, locale)
	}
	if len(locales) == 0 {
		must(fmt.Errorf("at least one target locale is required"))
	}
	return locales
}

func canonicalFiles() []string {
	paths, err := filepath.Glob(filepath.Join("docs", "*.md"))
	must(err)
	sort.Strings(paths)
	return paths
}

func selectFiles(all, names []string) []string {
	wanted := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if filepath.Ext(name) == "" {
			name += ".md"
		}
		wanted[name] = true
	}
	var selected []string
	for _, path := range all {
		if wanted[filepath.Base(path)] {
			selected = append(selected, path)
			delete(wanted, filepath.Base(path))
		}
	}
	if len(wanted) != 0 {
		must(fmt.Errorf("unknown documentation files: %v", mapKeys(wanted)))
	}
	return selected
}

func readManifest() *manifest {
	m := &manifest{Version: 1, SourceLocale: "es", Locales: []string{"es", "en", "pt"}, Sources: map[string]map[string]string{"en": {}, "pt": {}}}
	data, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return m
	}
	must(err)
	must(json.Unmarshal(data, m))
	for _, locale := range []string{"en", "pt"} {
		if m.Sources[locale] == nil {
			m.Sources[locale] = map[string]string{}
		}
	}
	return m
}

func writeManifest(m *manifest) {
	data, err := json.MarshalIndent(m, "", "  ")
	must(err)
	data = append(data, '\n')
	must(os.WriteFile(manifestPath, data, 0644))
}

func translateFile(sourcePath, locale string, m *manifest, force bool, translator translateFunc) {
	source, err := os.ReadFile(sourcePath)
	must(err)
	name := filepath.Base(sourcePath)
	hash := digest(source)
	target := filepath.Join("docs", locale, name)
	if !force && m.Sources[locale][name] == hash {
		if _, err := os.ReadFile(target); err == nil {
			fmt.Printf("current %s/%s\n", locale, name)
			return
		}
	}
	fmt.Printf("translating %s -> %s/%s\n", name, locale, name)
	translated, err := translateMarkdown(string(source), locale, translator)
	must(err)
	must(os.MkdirAll(filepath.Dir(target), 0755))
	translated = restoreInlineCode(string(source), translated, locale)
	translated = restoreLinkTargets(string(source), translated, locale)
	must(os.WriteFile(target, []byte(translated), 0644))
	m.Sources[locale][name] = hash
}

func fixTranslatedLinks(files []string) {
	for _, locale := range []string{"en", "pt"} {
		for _, canonical := range files {
			path := filepath.Join("docs", locale, filepath.Base(canonical))
			source, err := os.ReadFile(canonical)
			must(err)
			data, err := os.ReadFile(path)
			must(err)
			fixed := string(data)
			fixed = restoreProtectedMarkdown(string(source), fixed, locale)
			if len(inlineCode.FindAllStringIndex(string(source), -1)) == len(inlineCode.FindAllStringIndex(fixed, -1)) {
				fixed = restoreInlineCode(string(source), fixed, locale)
			} else {
				fmt.Printf("inline syntax requires retranslation: %s/%s\n", locale, filepath.Base(canonical))
			}
			fixed = restoreLinkTargets(string(source), fixed, locale)
			must(os.WriteFile(path, []byte(fixed), 0644))
		}
	}
}

func restoreProtectedMarkdown(source, translated, locale string) string {
	sourceSpans := protectedMarkdown.FindAllStringIndex(source, -1)
	translatedSpans := protectedMarkdown.FindAllStringIndex(translated, -1)
	if len(sourceSpans) != len(translatedSpans) {
		must(fmt.Errorf("translation changed protected Markdown count for locale %s: got %d, want %d", locale, len(translatedSpans), len(sourceSpans)))
	}
	var output strings.Builder
	position := 0
	for index, translatedSpan := range translatedSpans {
		sourceSpan := sourceSpans[index]
		output.WriteString(translated[position:translatedSpan[0]])
		output.WriteString(source[sourceSpan[0]:sourceSpan[1]])
		position = translatedSpan[1]
	}
	output.WriteString(translated[position:])
	return output.String()
}

func acceptManualTranslations(files, locales []string, m *manifest) {
	for _, locale := range locales {
		for _, canonical := range files {
			name := filepath.Base(canonical)
			source, err := os.ReadFile(canonical)
			must(err)
			translated, err := os.ReadFile(filepath.Join("docs", locale, name))
			must(err)
			if !equalBlocks(protectedMarkdown.FindAll(source, -1), protectedMarkdown.FindAll(translated, -1)) {
				must(fmt.Errorf("cannot accept %s/%s: protected Markdown differs", locale, name))
			}
			if !equalBlocks(inlineCode.FindAll(source, -1), inlineCode.FindAll(translated, -1)) {
				must(fmt.Errorf("cannot accept %s/%s: inline code differs", locale, name))
			}
			if len(markdownLink.FindAll(source, -1)) != len(markdownLink.FindAll(translated, -1)) {
				must(fmt.Errorf("cannot accept %s/%s: Markdown link count differs", locale, name))
			}
			m.Sources[locale][name] = digest(source)
		}
	}
}

func restoreInlineCode(source, translated, locale string) string {
	sourceSpans := inlineCode.FindAllStringIndex(source, -1)
	translatedSpans := inlineCode.FindAllStringIndex(translated, -1)
	if len(sourceSpans) != len(translatedSpans) {
		must(fmt.Errorf("translation changed inline code count for locale %s: got %d, want %d", locale, len(translatedSpans), len(sourceSpans)))
	}
	var output strings.Builder
	position := 0
	for index, translatedSpan := range translatedSpans {
		sourceSpan := sourceSpans[index]
		output.WriteString(translated[position:translatedSpan[0]])
		output.WriteString(source[sourceSpan[0]:sourceSpan[1]])
		position = translatedSpan[1]
	}
	output.WriteString(translated[position:])
	return output.String()
}

func restoreLinkTargets(source, translated, locale string) string {
	sourceLinks := markdownLink.FindAllStringSubmatchIndex(source, -1)
	translatedLinks := markdownLink.FindAllStringSubmatchIndex(translated, -1)
	if len(sourceLinks) != len(translatedLinks) {
		must(fmt.Errorf("translation changed Markdown link count for locale %s: got %d, want %d", locale, len(translatedLinks), len(sourceLinks)))
	}
	var output strings.Builder
	position := 0
	for index, translatedLink := range translatedLinks {
		sourceLink := sourceLinks[index]
		output.WriteString(translated[position:translatedLink[2]])
		output.WriteString(localizedTarget(source[sourceLink[2]:sourceLink[3]], locale))
		position = translatedLink[3]
	}
	output.WriteString(translated[position:])
	return output.String()
}

func localizedTarget(target, locale string) string {
	wrapped := strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">")
	clean := strings.Trim(target, "<>")
	if clean == "" || strings.HasPrefix(clean, "#") || strings.Contains(clean, "://") {
		return target
	}
	if strings.HasPrefix(clean, "../") {
		clean = "../" + clean
	} else if strings.HasPrefix(clean, "en/") || strings.HasPrefix(clean, "pt/") {
		parts := strings.SplitN(clean, "/", 2)
		if parts[0] == locale {
			clean = parts[1]
		} else {
			clean = "../" + clean
		}
	}
	if wrapped {
		clean = "<" + clean + ">"
	}
	return clean
}

func translateMarkdown(source, locale string, translator translateFunc) (string, error) {
	lines := strings.SplitAfter(source, "\n")
	var output strings.Builder
	var prose strings.Builder
	inFence := false
	inComment := false
	flush := func() error {
		if prose.Len() == 0 {
			return nil
		}
		original := prose.String()
		if strings.TrimSpace(original) == "" {
			output.WriteString(original)
			prose.Reset()
			return nil
		}
		translated, err := translateProse(original, locale, translator)
		if err != nil {
			return err
		}
		translated = restoreBoundaryNewlines(original, translated)
		output.WriteString(translated)
		prose.Reset()
		return nil
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		fence := strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
		commentStart := strings.Contains(line, "<!--")
		protected := inFence || inComment || fence || commentStart
		if protected {
			if err := flush(); err != nil {
				return "", err
			}
			output.WriteString(line)
			if fence {
				inFence = !inFence
			}
			if commentStart && !strings.Contains(line[strings.Index(line, "<!--")+4:], "-->") {
				inComment = true
			}
			if inComment && strings.Contains(line, "-->") {
				inComment = false
			}
			continue
		}
		// Protected UUID markers expand dense Markdown tables considerably. Keep
		// source batches small enough to stay below the provider's 5,000-character
		// request limit after links, inline code and line breaks are masked.
		if prose.Len()+len(line) > 700 {
			if err := flush(); err != nil {
				return "", err
			}
		}
		prose.WriteString(line)
	}
	if err := flush(); err != nil {
		return "", err
	}
	return output.String(), nil
}

func translateProse(original, locale string, translator translateFunc) (string, error) {
	if strings.TrimSpace(original) == "" {
		return original, nil
	}
	masked, protected := maskTranslationSyntax(original)
	requestText := encodeTranslationHTML(masked, protected)
	if len(requestText) > 3500 {
		splitAt := proseSplitIndex(original)
		if splitAt <= 0 || splitAt >= len(original) {
			return "", fmt.Errorf("unable to split oversized translation block")
		}
		left, err := translateProse(original[:splitAt], locale, translator)
		if err != nil {
			return "", err
		}
		right, err := translateProse(original[splitAt:], locale, translator)
		if err != nil {
			return "", err
		}
		return left + right, nil
	}
	translated, err := translator(requestText, locale)
	if err != nil {
		return "", err
	}
	translated, err = decodeTranslationHTML(translated, protected)
	if err != nil {
		return "", err
	}
	return unmaskTranslationSyntax(translated, protected)
}

func proseSplitIndex(text string) int {
	middle := len(text) / 2
	if index := strings.LastIndex(text[:middle], "\n"); index >= 0 {
		return index + 1
	}
	if index := strings.LastIndex(text[:middle], " "); index >= 0 {
		return index + 1
	}
	runes := []rune(text)
	return len(string(runes[:len(runes)/2]))
}

func encodeTranslationHTML(text string, protected protectedSyntax) string {
	text = html.EscapeString(text)
	for index := range protected.links {
		text = strings.ReplaceAll(text, syntaxMarker('S', index), protectedHTMLMarker("link-start", index))
		text = strings.ReplaceAll(text, syntaxMarker('E', index), protectedHTMLMarker("link-end", index))
	}
	for index := range protected.inline {
		text = strings.ReplaceAll(text, syntaxMarker('I', index), protectedHTMLMarker("inline", index))
	}
	for index := range protected.lineBreaks {
		text = strings.ReplaceAll(text, syntaxMarker('B', index), protectedHTMLMarker("break", index))
	}
	return text
}

func protectedHTMLMarker(kind string, index int) string {
	return fmt.Sprintf(`<span translate="no" data-joss-%s="%d">%s</span>`, kind, index, htmlMarkerToken(kind, index))
}

func htmlMarkerToken(kind string, index int) string {
	return fmt.Sprintf("JOSSZXQ%sX%dQXZ", strings.ToUpper(strings.ReplaceAll(kind, "-", "")), index)
}

func decodeTranslationHTML(text string, protected protectedSyntax) (string, error) {
	restore := func(kind byte, name string, count int) error {
		for index := 0; index < count; index++ {
			marker := syntaxMarker(kind, index)
			token := htmlMarkerToken(name, index)
			pattern := regexp.MustCompile(fmt.Sprintf(`(?is)<span[^>]*data-joss-%s=["']?%d["']?[^>]*>.*?</span>`, regexp.QuoteMeta(name), index))
			if pattern.MatchString(text) {
				text = pattern.ReplaceAllStringFunc(text, func(string) string { return marker })
			} else if strings.Contains(text, token) {
				text = strings.ReplaceAll(text, token, marker)
			} else {
				return fmt.Errorf("translation provider changed protected HTML marker %s %d in %q", name, index, text)
			}
		}
		return nil
	}
	if err := restore('S', "link-start", len(protected.links)); err != nil {
		return "", err
	}
	if err := restore('E', "link-end", len(protected.links)); err != nil {
		return "", err
	}
	if err := restore('I', "inline", len(protected.inline)); err != nil {
		return "", err
	}
	if err := restore('B', "break", len(protected.lineBreaks)); err != nil {
		return "", err
	}
	return html.UnescapeString(text), nil
}

func newTranslator(provider string) (translateFunc, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "bing", "microsoft":
		client, err := newBingTranslator()
		if err != nil {
			return nil, err
		}
		return client.translate, nil
	case "google":
		return googleTranslate, nil
	default:
		return nil, fmt.Errorf("unknown translation provider %q", provider)
	}
}

func newBingTranslator() (*bingTranslator, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 45 * time.Second, Jar: jar}
	request, err := http.NewRequest(http.MethodGet, "https://www.bing.com/translator", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Mozilla/5.0")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Microsoft Translator session returned %s", response.Status)
	}
	page, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	extract := func(pattern string) (string, error) {
		match := regexp.MustCompile(pattern).FindSubmatch(page)
		if len(match) != 2 {
			return "", fmt.Errorf("Microsoft Translator session parameter not found")
		}
		return string(match[1]), nil
	}
	ig, err := extract(`IG:"([A-F0-9]+)"`)
	if err != nil {
		return nil, err
	}
	iid, err := extract(`data-iid="(translator\.[0-9]+)"`)
	if err != nil {
		return nil, err
	}
	abuse := regexp.MustCompile(`params_AbusePreventionHelper\s*=\s*\[([0-9]+),"([^"]+)"`).FindSubmatch(page)
	if len(abuse) != 3 {
		return nil, fmt.Errorf("Microsoft Translator anti-abuse parameters not found")
	}
	return &bingTranslator{client: client, ig: ig, iid: iid, key: string(abuse[1]), token: string(abuse[2])}, nil
}

func (translator *bingTranslator) translate(text, locale string) (string, error) {
	endpoint := "https://www.bing.com/ttranslatev3?isVertical=1&IG=" + url.QueryEscape(translator.ig) + "&IID=" + url.QueryEscape(translator.iid) + "&SFX=0"
	form := url.Values{"text": {text}, "fromLang": {"es"}, "to": {locale}, "token": {translator.token}, "key": {translator.key}}
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Referer", "https://www.bing.com/translator")
	request.Header.Set("User-Agent", "Mozilla/5.0")
	response, err := translator.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Microsoft Translator returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var payload []struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload) == 0 || len(payload[0].Translations) == 0 {
		return "", fmt.Errorf("unexpected Microsoft Translator response: %s", strings.TrimSpace(string(body)))
	}
	return payload[0].Translations[0].Text, nil
}

func restoreBoundaryNewlines(source, translated string) string {
	leading := len(source) - len(strings.TrimLeft(source, "\r\n"))
	trailing := len(source) - len(strings.TrimRight(source, "\r\n"))
	translated = strings.Trim(translated, "\r\n")
	return source[:leading] + translated + source[len(source)-trailing:]
}

func maskTranslationSyntax(text string) (string, protectedSyntax) {
	protected := protectedSyntax{}
	protectInline := func(value string) string {
		placeholder := syntaxMarker('I', len(protected.inline))
		protected.inline = append(protected.inline, value)
		return placeholder
	}
	masked := markdownLink.ReplaceAllStringFunc(text, func(link string) string {
		separator := strings.LastIndex(link, "](")
		if separator < 1 || !strings.HasSuffix(link, ")") {
			return link
		}
		index := len(protected.links)
		protected.links = append(protected.links, link[separator+2:len(link)-1])
		start := syntaxMarker('S', index)
		end := syntaxMarker('E', index)
		return start + " " + link[1:separator] + " " + end
	})
	masked = translationSyntax.ReplaceAllStringFunc(masked, protectInline)
	lineBreak := regexp.MustCompile(`(?:\r\n|\n)+`)
	masked = lineBreak.ReplaceAllStringFunc(masked, func(value string) string {
		placeholder := syntaxMarker('B', len(protected.lineBreaks))
		protected.lineBreaks = append(protected.lineBreaks, value)
		return " " + placeholder + " "
	})
	return masked, protected
}

func syntaxMarker(kind byte, index int) string {
	base := 0xE000
	switch kind {
	case 'S':
		base = 0xE400
	case 'E':
		base = 0xE800
	case 'B':
		base = 0xEC00
	}
	return string(rune(base + index))
}

func unmaskTranslationSyntax(text string, protected protectedSyntax) (string, error) {
	for index, value := range protected.links {
		start := syntaxMarker('S', index)
		end := syntaxMarker('E', index)
		upperText := strings.ToUpper(text)
		startAt := strings.Index(upperText, strings.ToUpper(start))
		endAt := strings.Index(upperText, strings.ToUpper(end))
		if startAt < 0 || endAt < startAt {
			return "", fmt.Errorf("translation provider changed protected Markdown link %q in %q", value, text)
		}
		label := strings.TrimSpace(text[startAt+len(start) : endAt])
		label = strings.NewReplacer("[", "", "]", "").Replace(label)
		if strings.TrimSpace(label) == "" {
			label = value
		}
		text = text[:startAt] + "[" + label + "](" + value + ")" + text[endAt+len(end):]
	}
	for index, value := range protected.inline {
		placeholder := syntaxMarker('I', index)
		pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(placeholder))
		if !pattern.MatchString(text) {
			return "", fmt.Errorf("translation provider changed protected Markdown syntax %q", value)
		}
		text = pattern.ReplaceAllStringFunc(text, func(string) string { return value })
	}
	for index, value := range protected.lineBreaks {
		placeholder := syntaxMarker('B', index)
		pattern := regexp.MustCompile(`(?i)[ \t]*` + regexp.QuoteMeta(placeholder) + `[ \t]*`)
		if !pattern.MatchString(text) {
			return "", fmt.Errorf("translation provider changed protected line break %d in %q", index, text)
		}
		text = pattern.ReplaceAllString(text, value)
	}
	return text, nil
}

func googleTranslate(text, locale string) (string, error) {
	endpoint := "https://translate.googleapis.com/translate_a/single?client=gtx&sl=es&tl=" + url.QueryEscape(locale) + "&dt=t&format=html"
	client := &http.Client{Timeout: 45 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(url.Values{"q": {text}}.Encode()))
		if err != nil {
			return "", err
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := client.Do(request)
		if err == nil && response.StatusCode == http.StatusOK {
			body, readErr := io.ReadAll(response.Body)
			response.Body.Close()
			var payload []any
			if readErr == nil {
				err = json.Unmarshal(body, &payload)
				if err == nil {
					var translated string
					translated, err = translatedText(payload)
					if err == nil {
						return translated, nil
					}
				}
			} else {
				err = readErr
			}
			if err != nil {
				err = fmt.Errorf("unexpected translation response: %w: %s", err, strings.TrimSpace(string(body[:min(len(body), 512)])))
			}
		} else if response != nil {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
			response.Body.Close()
			err = fmt.Errorf("translation service returned %s: %s", response.Status, strings.TrimSpace(string(body)))
		}
		lastErr = err
		if response != nil && response.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(attempt+1) * 15 * time.Second)
		} else {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	return "", lastErr
}

func translatedText(payload []any) (string, error) {
	if len(payload) == 0 {
		return "", fmt.Errorf("empty translation response")
	}
	segments, ok := payload[0].([]any)
	if !ok {
		return "", fmt.Errorf("unexpected translation response")
	}
	var output strings.Builder
	for _, raw := range segments {
		segment, ok := raw.([]any)
		if !ok || len(segment) == 0 {
			continue
		}
		text, _ := segment[0].(string)
		output.WriteString(text)
	}
	return output.String(), nil
}

func syncMirrors(files []string) {
	for _, locale := range []string{"es", "en", "pt"} {
		dir := filepath.Join(publicRoot, locale)
		must(os.MkdirAll(dir, 0755))
		for _, canonical := range files {
			source := canonical
			if locale != "es" {
				source = filepath.Join("docs", locale, filepath.Base(canonical))
			}
			data, err := os.ReadFile(source)
			must(err)
			must(os.WriteFile(filepath.Join(dir, filepath.Base(canonical)), data, 0644))
		}
	}
}

func checkAll(files []string, m *manifest) error {
	var problems []string
	names := map[string]bool{}
	for _, path := range files {
		names[filepath.Base(path)] = true
	}
	for _, locale := range []string{"es", "en", "pt"} {
		dir := "docs"
		if locale != "es" {
			dir = filepath.Join("docs", locale)
		}
		localeFiles, _ := filepath.Glob(filepath.Join(dir, "*.md"))
		if len(localeFiles) != len(files) {
			problems = append(problems, fmt.Sprintf("%s has %d documents; want %d", locale, len(localeFiles), len(files)))
		}
		for _, canonical := range files {
			name := filepath.Base(canonical)
			source := filepath.Join(dir, name)
			data, err := os.ReadFile(source)
			if err != nil {
				problems = append(problems, fmt.Sprintf("missing %s/%s", locale, name))
				continue
			}
			if locale != "es" {
				canonicalData, _ := os.ReadFile(canonical)
				if m.Sources[locale][name] != digest(canonicalData) {
					problems = append(problems, fmt.Sprintf("stale translation %s/%s", locale, name))
				}
				canonicalProtected := protectedMarkdown.FindAll(canonicalData, -1)
				translatedProtected := protectedMarkdown.FindAll(data, -1)
				if !equalBlocks(canonicalProtected, translatedProtected) {
					problems = append(problems, fmt.Sprintf("code fences or contracts changed in %s/%s", locale, name))
				}
				if !equalBlocks(inlineCode.FindAll(canonicalData, -1), inlineCode.FindAll(data, -1)) {
					problems = append(problems, fmt.Sprintf("inline code changed in %s/%s", locale, name))
				}
			}
			checkLinks(source, data, names, &problems)
			published, err := os.ReadFile(filepath.Join(publicRoot, locale, name))
			if err != nil || !bytes.Equal(data, published) {
				problems = append(problems, fmt.Sprintf("public mirror differs or is missing: %s/%s", locale, name))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("documentation i18n check failed:\n- %s", strings.Join(problems, "\n- "))
	}
	fmt.Printf("documentation i18n is current: %d files x 3 locales\n", len(files))
	return nil
}

func equalBlocks(left, right [][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !bytes.Equal(left[index], right[index]) {
			return false
		}
	}
	return true
}

func checkLinks(path string, data []byte, names map[string]bool, problems *[]string) {
	for _, match := range markdownLink.FindAllStringSubmatch(string(data), -1) {
		target := strings.TrimSpace(strings.Trim(match[1], "<>"))
		if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "../") {
			continue
		}
		target = strings.SplitN(strings.SplitN(target, "#", 2)[0], "?", 2)[0]
		if strings.EqualFold(filepath.Ext(target), ".md") && !names[filepath.Base(filepath.FromSlash(target))] {
			*problems = append(*problems, fmt.Sprintf("%s has broken local link %q", filepath.ToSlash(path), match[1]))
		}
	}
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func mapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
