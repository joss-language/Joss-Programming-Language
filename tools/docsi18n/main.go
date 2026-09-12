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
	"io"
	"net/http"
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

var markdownLink = regexp.MustCompile(`\[[^]]+\]\(([^)]+)\)`)

type manifest struct {
	Version      int                          `json:"version"`
	SourceLocale string                       `json:"sourceLocale"`
	Locales      []string                     `json:"locales"`
	Sources      map[string]map[string]string `json:"sources"`
}

func main() {
	translate := flag.Bool("translate", false, "translate stale or missing English and Portuguese documents")
	force := flag.Bool("force", false, "translate every document even when its source hash is current")
	syncPublic := flag.Bool("sync", false, "synchronize all three locales to the JosSecurity public mirror")
	check := flag.Bool("check", false, "verify translation coverage, freshness, links and public mirrors")
	filesFlag := flag.String("files", "", "optional comma-separated canonical Markdown filenames")
	flag.Parse()

	if !*translate && !*syncPublic && !*check {
		flag.Usage()
		os.Exit(2)
	}

	files := canonicalFiles()
	if *filesFlag != "" {
		files = selectFiles(files, strings.Split(*filesFlag, ","))
	}
	m := readManifest()
	if *translate {
		for _, locale := range []string{"en", "pt"} {
			for _, path := range files {
				translateFile(path, locale, m, *force)
			}
		}
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

func translateFile(sourcePath, locale string, m *manifest, force bool) {
	source, err := os.ReadFile(sourcePath)
	must(err)
	name := filepath.Base(sourcePath)
	hash := digest(source)
	target := filepath.Join("docs", locale, name)
	if !force && m.Sources[locale][name] == hash {
		if _, err := os.Stat(target); err == nil {
			fmt.Printf("current %s/%s\n", locale, name)
			return
		}
	}
	fmt.Printf("translating %s -> %s/%s\n", name, locale, name)
	translated, err := translateMarkdown(string(source), locale)
	must(err)
	must(os.MkdirAll(filepath.Dir(target), 0755))
	must(os.WriteFile(target, []byte(translated), 0644))
	m.Sources[locale][name] = hash
}

func translateMarkdown(source, locale string) (string, error) {
	lines := strings.SplitAfter(source, "\n")
	var output strings.Builder
	var prose strings.Builder
	inFence := false
	inComment := false
	flush := func() error {
		if prose.Len() == 0 {
			return nil
		}
		translated, err := googleTranslate(prose.String(), locale)
		if err != nil {
			return err
		}
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
		if prose.Len()+len(line) > 3200 {
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

func googleTranslate(text, locale string) (string, error) {
	endpoint := "https://translate.googleapis.com/translate_a/single?client=gtx&sl=es&tl=" + url.QueryEscape(locale) + "&dt=t&q=" + url.QueryEscape(text)
	client := &http.Client{Timeout: 45 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		response, err := client.Get(endpoint)
		if err == nil && response.StatusCode == http.StatusOK {
			var payload []any
			err = json.NewDecoder(response.Body).Decode(&payload)
			response.Body.Close()
			if err == nil {
				return translatedText(payload)
			}
		} else if response != nil {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
			response.Body.Close()
			err = fmt.Errorf("translation service returned %s: %s", response.Status, strings.TrimSpace(string(body)))
		}
		lastErr = err
		time.Sleep(time.Duration(attempt+1) * time.Second)
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
