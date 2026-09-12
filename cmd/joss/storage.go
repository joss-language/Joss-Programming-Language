package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jossecurity/joss/pkg/i18n"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
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
	// 1. Update Env to STORAGE=local
	updateEnvVariable("STORAGE", "local")
	fmt.Println(i18n.Tr("storageConfiguredLocal"))

	// 2. Ask if user wants to migrate FROM OCI to Local
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

	// Gather OCI Details
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

	// Ask for migration
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

// --- Migration Logis ---

func getOCIClient() (objectstorage.ObjectStorageClient, context.Context, error) {
	// Read Env again to be sure
	envMap := loadEnvMap()

	// Create configuration provider
	// We need to support reading the private key from file
	privateKey, err := os.ReadFile(envMap["OCI_PRIVATE_KEY_PATH"])
	if err != nil {
		return objectstorage.ObjectStorageClient{}, nil, fmt.Errorf("error leyendo private key: %v", err)
	}

	passphrase := envMap["OCI_PASSPHRASE"]
	confProvider := common.NewRawConfigurationProvider(
		envMap["OCI_TENANCY_ID"],
		envMap["OCI_USER_ID"],
		envMap["OCI_REGION"],
		envMap["OCI_FINGERPRINT"],
		string(privateKey),
		&passphrase,
	)

	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(confProvider)
	if err != nil {
		return objectstorage.ObjectStorageClient{}, nil, err
	}

	return client, context.Background(), nil
}

func migrateToOCI() {
	fmt.Printf("\n%s\n", i18n.Tr("storageStartingMigrationOci"))

	client, ctx, err := getOCIClient()
	if err != nil {
		fmt.Println(i18n.Tr("storageOciInitError", i18n.M{"error": err}))
		return
	}

	envMap := loadEnvMap()
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
				// Convert backslashes to slashes for object storage keys
				objectName := filepath.ToSlash(relPath)

				fmt.Println(i18n.Tr("storageUploading", map[string]interface{}{"source": path, "dest": objectName}))

				file, err := os.Open(path)
				if err != nil {
					fmt.Println(i18n.Tr("storageOpenFileError", i18n.M{"path": path, "error": err}))
					return nil
				}
				defer file.Close()

				stat, _ := file.Stat()

				req := objectstorage.PutObjectRequest{
					NamespaceName: &namespace,
					BucketName:    &bucketName,
					ObjectName:    &objectName,
					PutObjectBody: file,
					ContentLength: common.Int64(stat.Size()),
				}

				_, err = client.PutObject(ctx, req)
				if err != nil {
					fmt.Println(i18n.Tr("storageUploadError", i18n.M{"error": err}))
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

	client, ctx, err := getOCIClient()
	if err != nil {
		fmt.Println(i18n.Tr("storageOciInitError", i18n.M{"error": err}))
		return
	}

	envMap := loadEnvMap()
	namespace := envMap["OCI_NAMESPACE"]
	bucketName := envMap["OCI_BUCKET_NAME"]
	baseDir := "assets/users"

	// List objects
	var start string
	fields := "name,size"
	for {
		req := objectstorage.ListObjectsRequest{
			NamespaceName: &namespace,
			BucketName:    &bucketName,
			Start:         &start,
			Limit:         common.Int(100),
			Fields:        &fields,
		}

		resp, err := client.ListObjects(ctx, req)
		if err != nil {
			fmt.Println(i18n.Tr("storageListObjectsError", i18n.M{"error": err}))
			return
		}

		for _, item := range resp.ListObjects.Objects {
			objectName := *item.Name
			targetPath := filepath.Join(baseDir, objectName)

			fmt.Println(i18n.Tr("storageDownloading", map[string]interface{}{"source": objectName, "dest": targetPath}))

			// Ensure dir
			os.MkdirAll(filepath.Dir(targetPath), 0755)

			// Get content
			getReq := objectstorage.GetObjectRequest{
				NamespaceName: &namespace,
				BucketName:    &bucketName,
				ObjectName:    &objectName,
			}

			getResp, err := client.GetObject(ctx, getReq)
			if err != nil {
				fmt.Println(i18n.Tr("storageDownloadError", i18n.M{"error": err}))
				continue
			}

			outFile, err := os.Create(targetPath)
			if err != nil {
				fmt.Println(i18n.Tr("storageCreateFileError", i18n.M{"error": err}))
				getResp.Content.Close()
				continue
			}

			_, err = io.Copy(outFile, getResp.Content)
			outFile.Close()
			getResp.Content.Close()

			if err != nil {
				fmt.Println(i18n.Tr("storageWriteFileError", i18n.M{"error": err}))
			}
		}

		if resp.NextStartWith == nil {
			break
		}
		start = *resp.NextStartWith
	}

	fmt.Println(i18n.Tr("storageDownloadCompleted"))
}

func loadEnvMap() map[string]string {
	return readEnvFile(GetEnvFile())
}
