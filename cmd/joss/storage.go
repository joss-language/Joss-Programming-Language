package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/i18n"
)

func handleUserStorage(provider string) {
	switch strings.ToLower(provider) {
	case "migrate-oci", "sync-oci":
		migrateToOCI()
	case "migrate-local", "sync-local":
		migrateFromOCI()
	case "local":
		fmt.Println(i18n.Tr("storageConfiguring", map[string]interface{}{"provider": provider}))
		configureLocal()
	case "oci":
		fmt.Println(i18n.Tr("storageConfiguring", map[string]interface{}{"provider": provider}))
		configureOCI()
	case "aws", "azure":
		fmt.Println(i18n.Tr("storageNotSupported", map[string]interface{}{"provider": provider}))
	default:
		fmt.Println(i18n.Tr("storageUnknownProvider", map[string]interface{}{"provider": provider}))
	}
}

func configureLocal() {
	updateEnvVariable("STORAGE", "local")
	fmt.Println(i18n.Tr("storageConfiguredLocal"))

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (s/n): ", i18n.Tr("storagePromptDownloadOci"))
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(strings.ToLower(text))

	if text == "s" || text == "y" || text == "si" || text == "yes" {
		migrateFromOCI()
	}
}

func configureOCI() {
	reader := bufio.NewReader(os.Stdin)
	config := make(map[string]string)

	fmt.Printf("\n%s\n", i18n.Tr("storageOciTitle"))

	fields := []struct {
		Key   string
		Label string
	}{
		{"OCI_NAMESPACE", "Namespace"},
		{"OCI_BUCKET_NAME", "Bucket Name"},
		{"OCI_TENANCY_ID", "Tenancy OCID"},
		{"OCI_USER_ID", "User OCID"},
		{"OCI_REGION", "Region (e.g. mx-queretaro-1)"},
		{"OCI_FINGERPRINT", "Fingerprint"},
		{"OCI_PRIVATE_KEY_PATH", "Private Key Path (e.g. storage/oci_api_key.pem)"},
		{"OCI_PASSPHRASE", "Passphrase (dejar vacío si no tiene)"},
	}

	for _, field := range fields {
		fmt.Printf("%s: ", field.Label)
		val, _ := reader.ReadString('\n')
		val = strings.TrimSpace(val)
		if val != "" {
			config[field.Key] = val
			updateEnvVariable(field.Key, val)
		}
	}

	updateEnvVariable("STORAGE", "OCI")
	fmt.Printf("\n%s\n", i18n.Tr("storageOciSaved"))

	fmt.Printf("%s (s/n): ", i18n.Tr("storagePromptUploadOci"))
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(strings.ToLower(text))

	if text == "s" || text == "y" || text == "si" || text == "yes" {
		migrateToOCI()
	}
}

func updateEnvVariable(key, value string) {
	updateEnvFile(GetEnvFile(), key, value)
}

// --- Native HTTP OCI Helpers (100% Pure Joss / Zero SDK Dependencies) ---

func getOCIPrivateKey(envMap map[string]string) (string, error) {
	keyPath := envMap["OCI_PRIVATE_KEY_PATH"]
	if keyPath == "" {
		keyPath = "assets/oci/private_key.pem"
	}
	candidates := []string{keyPath, "./" + keyPath, "../" + keyPath, "../../" + keyPath}
	for _, cand := range candidates {
		if data, err := os.ReadFile(cand); err == nil && len(data) > 0 {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("error leyendo private key en %s", keyPath)
}

func ociSendRequest(envMap map[string]string, pemKey, method, path string, bodyBytes []byte, headers map[string]string) (*http.Response, []byte, error) {
	region := envMap["OCI_REGION"]
	if region == "" {
		region = "mx-queretaro-1"
	}
	host := "objectstorage." + region + ".oraclecloud.com"
	date := time.Now().UTC().Format(http.TimeFormat)

	var signingString, authHeader string
	keyId := fmt.Sprintf("%s/%s/%s", envMap["OCI_TENANCY_ID"], envMap["OCI_USER_ID"], envMap["OCI_FINGERPRINT"])

	if strings.ToUpper(method) == "PUT" {
		h := sha256.Sum256(bodyBytes)
		shaB64 := base64.StdEncoding.EncodeToString(h[:])
		cLen := fmt.Sprintf("%d", len(bodyBytes))
		cType := "application/octet-stream"

		signingString = fmt.Sprintf("date: %s\n(request-target): put %s\nhost: %s\nx-content-sha256: %s\ncontent-type: %s\ncontent-length: %s",
			date, path, host, shaB64, cType, cLen)
		sig, err := crypto.SignRSASHA256(signingString, pemKey)
		if err != nil {
			return nil, nil, err
		}
		authHeader = fmt.Sprintf(`Signature version="1",headers="date (request-target) host x-content-sha256 content-type content-length",keyId="%s",algorithm="rsa-sha256",signature="%s"`, keyId, sig)

		if headers == nil {
			headers = make(map[string]string)
		}
		headers["x-content-sha256"] = shaB64
		headers["Content-Type"] = cType
		headers["Content-Length"] = cLen
	} else {
		signingString = fmt.Sprintf("date: %s\n(request-target): %s %s\nhost: %s", date, strings.ToLower(method), path, host)
		sig, err := crypto.SignRSASHA256(signingString, pemKey)
		if err != nil {
			return nil, nil, err
		}
		authHeader = fmt.Sprintf(`Signature version="1",headers="date (request-target) host",keyId="%s",algorithm="rsa-sha256",signature="%s"`, keyId, sig)
	}

	var reader io.Reader
	if len(bodyBytes) > 0 {
		reader = strings.NewReader(string(bodyBytes))
	}

	req, err := http.NewRequest(method, "https://"+host+path, reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Host", host)
	req.Host = host
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "*/*")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp, data, err
}

func migrateToOCI() {
	fmt.Printf("\n%s\n", i18n.Tr("storageStartingMigrationOci"))
	envMap := loadEnvMap()
	pemKey, err := getOCIPrivateKey(envMap)
	if err != nil {
		fmt.Println(i18n.Tr("storageOciInitError", i18n.M{"error": err}))
		return
	}

	namespace := envMap["OCI_NAMESPACE"]
	bucketName := envMap["OCI_BUCKET_NAME"]
	baseDirs := []string{"assets/users", "storage/user_storage"}

	for _, baseDir := range baseDirs {
		if _, statErr := os.Stat(baseDir); statErr != nil {
			continue
		}
		err = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				relPath, _ := filepath.Rel(baseDir, path)
				objectName := filepath.ToSlash(relPath)
				fmt.Println(i18n.Tr("storageUploading", map[string]interface{}{"source": path, "dest": objectName}))

				data, err := os.ReadFile(path)
				if err != nil {
					fmt.Println(i18n.Tr("storageOpenFileError", i18n.M{"path": path, "error": err}))
					return nil
				}

				targetPath := "/n/" + url.PathEscape(namespace) + "/b/" + url.PathEscape(bucketName) + "/o/" + objectName
				resp, body, err := ociSendRequest(envMap, pemKey, "PUT", targetPath, data, nil)
				if err != nil || (resp.StatusCode != 200 && resp.StatusCode != 201) {
					fmt.Println(i18n.Tr("storageUploadError", i18n.M{"error": fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))}))
				}
			}
			return nil
		})
	}

	if err != nil {
		fmt.Println(i18n.Tr("storageWalkDirError", i18n.M{"error": err}))
	} else {
		fmt.Println(i18n.Tr("storageMigrationOciCompleted"))
	}
}

func migrateFromOCI() {
	fmt.Printf("\n%s\n", i18n.Tr("storageStartingDownloadOci"))
	envMap := loadEnvMap()
	pemKey, err := getOCIPrivateKey(envMap)
	if err != nil {
		fmt.Println(i18n.Tr("storageOciInitError", i18n.M{"error": err}))
		return
	}

	namespace := envMap["OCI_NAMESPACE"]
	bucketName := envMap["OCI_BUCKET_NAME"]
	baseDir := "assets/users"

	start := ""
	for {
		listPath := "/n/" + url.PathEscape(namespace) + "/b/" + url.PathEscape(bucketName) + "/o?limit=100"
		if start != "" {
			listPath += "&start=" + url.QueryEscape(start)
		}

		resp, body, err := ociSendRequest(envMap, pemKey, "GET", listPath, nil, nil)
		if err != nil || resp.StatusCode != 200 {
			fmt.Println(i18n.Tr("storageListObjectsError", i18n.M{"error": fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))}))
			return
		}

		var listResult struct {
			Objects []struct {
				Name string `json:"name"`
			} `json:"objects"`
			NextStartWith *string `json:"nextStartWith"`
		}
		if err := json.Unmarshal(body, &listResult); err != nil {
			fmt.Println(i18n.Tr("storageListObjectsError", i18n.M{"error": err}))
			return
		}

		for _, item := range listResult.Objects {
			objectName := item.Name
			targetPath := filepath.Join(baseDir, objectName)
			fmt.Println(i18n.Tr("storageDownloading", map[string]interface{}{"source": objectName, "dest": targetPath}))

			_ = os.MkdirAll(filepath.Dir(targetPath), 0755)

			objPath := "/n/" + url.PathEscape(namespace) + "/b/" + url.PathEscape(bucketName) + "/o/" + objectName
			getResp, getData, err := ociSendRequest(envMap, pemKey, "GET", objPath, nil, nil)
			if err != nil || getResp.StatusCode != 200 {
				fmt.Println(i18n.Tr("storageDownloadError", i18n.M{"error": fmt.Sprintf("status %d", getResp.StatusCode)}))
				continue
			}

			if err := os.WriteFile(targetPath, getData, 0644); err != nil {
				fmt.Println(i18n.Tr("storageWriteFileError", i18n.M{"error": err}))
			}
		}

		if listResult.NextStartWith == nil || *listResult.NextStartWith == "" {
			break
		}
		start = *listResult.NextStartWith
	}

	fmt.Println(i18n.Tr("storageDownloadCompleted"))
}

func loadEnvMap() map[string]string {
	return readEnvFile(GetEnvFile())
}
