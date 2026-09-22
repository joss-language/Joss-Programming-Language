package core

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// UserStorage Native Class Implementation
// Usage: UserStorage::put($user_token, "profile.jpg", $file_content)
func (r *Runtime) executeUserStorageMethod(instance *Instance, method string, args []interface{}) interface{} {
	basePath := "assets/users"
	storageType := "local"
	if val, ok := r.Env["STORAGE"]; ok {
		storageType = val
	}

	// Get Prefix and Table Names
	prefix := "js_"
	if val, ok := r.Env["PREFIX"]; ok {
		prefix = val
	}
	storageTable := prefix + "storage"
	usersTable := prefix + "users"

	// Ensure DB tables exist
	r.ensureStorageTable(storageTable)

	// extractToken: JOSS may pass the full user Instance instead of a plain token string.
	// If args[0] is an *Instance, extract the "user_token" field from its Fields map.
	extractToken := func(arg interface{}) string {
		if inst, ok := arg.(*Instance); ok {
			if tok, exists := inst.Fields["user_token"]; exists {
				return fmt.Sprintf("%v", tok)
			}
			// Fallback: try "token" field
			if tok, exists := inst.Fields["token"]; exists {
				return fmt.Sprintf("%v", tok)
			}
		}
		return fmt.Sprintf("%v", arg)
	}

	switch method {
	case "path":
		if len(args) > 0 {
			relPath := fmt.Sprintf("%v", args[0])
			cwd, _ := os.Getwd()
			return filepath.Join(cwd, "storage", relPath)
		}
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, "storage")

	case "put":
		if len(args) < 3 {
			return false
		}
		userToken := extractToken(args[0])
		fileName := fmt.Sprintf("%v", args[1]) // Can be "photos/my_pic.jpg"
		content := fmt.Sprintf("%v", args[2])
		fullPath, pathErr := safeUserStoragePath(basePath, userToken, fileName)
		if pathErr != nil {
			fmt.Printf("[Storage] Ruta rechazada: %v\n", pathErr)
			return false
		}

		// DB Registry Logic (Common for both)
		if r.GetDB() != nil {
			userId := r.getUserIdFromToken(usersTable, userToken)
			if userId > 0 {
				// Check if exists
				var existingId int
				check := fmt.Sprintf("SELECT id FROM %s WHERE user_id = ? AND path = ?", storageTable)
				err := r.databaseExecutor().QueryRow(check, userId, fileName).Scan(&existingId)

				if err == sql.ErrNoRows {
					// Insert
					insert := fmt.Sprintf("INSERT INTO %s (user_id, path, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", storageTable)
					if val, ok := r.Env["DB"]; ok && val == "mysql" {
						insert = fmt.Sprintf("INSERT INTO %s (user_id, path, created_at, updated_at) VALUES (?, ?, NOW(), NOW())", storageTable)
					}
					r.databaseExecutor().Exec(insert, userId, fileName)
				} else {
					// Update timestamp
					update := fmt.Sprintf("UPDATE %s SET updated_at = CURRENT_TIMESTAMP WHERE id = ?", storageTable)
					if val, ok := r.Env["DB"]; ok && val == "mysql" {
						update = fmt.Sprintf("UPDATE %s SET updated_at = NOW() WHERE id = ?", storageTable)
					}
					r.databaseExecutor().Exec(update, existingId)
				}
			}
		}

		if storageType == "OCI" {
			return r.ociPut(userToken, fileName, content)
		} else {
			// LOCAL STORAGE
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("[Storage DEBUG] MkdirAll error: %v\n", err)
				return false
			}

			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				fmt.Printf("[Storage DEBUG] WriteFile error: %v\n", err)
				return false
			}
			return true
		}

	case "get":
		if len(args) < 2 {
			return nil
		}
		userToken := extractToken(args[0])
		fileName := fmt.Sprintf("%v", args[1])
		fullPath, pathErr := safeUserStoragePath(basePath, userToken, fileName)
		if pathErr != nil {
			return nil
		}

		if storageType == "OCI" {
			return r.ociGet(userToken, fileName)
		} else {
			content, err := os.ReadFile(fullPath)
			if err != nil {
				return nil
			}
			return string(content)
		}

	case "getToFile":
		if len(args) < 3 {
			return false
		}
		userToken := extractToken(args[0])
		fileName := fmt.Sprintf("%v", args[1])
		destPath := fmt.Sprintf("%v", args[2])
		srcPath, pathErr := safeUserStoragePath(basePath, userToken, fileName)
		if pathErr != nil {
			return false
		}

		if storageType == "OCI" {
			return r.ociGetToFile(userToken, fileName, destPath)
		} else {
			// Local: just copy the file
			content, err := os.ReadFile(srcPath)
			if err != nil {
				return false
			}
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return false
			}
			return os.WriteFile(destPath, content, 0644) == nil
		}

	case "delete":
		if len(args) < 2 {
			return false
		}
		userToken := extractToken(args[0])
		fileName := fmt.Sprintf("%v", args[1])
		fullPath, pathErr := safeUserStoragePath(basePath, userToken, fileName)
		if pathErr != nil {
			return false
		}

		// DB Registry Delete
		if r.GetDB() != nil {
			userId := r.getUserIdFromToken(usersTable, userToken)
			if userId > 0 {
				query := fmt.Sprintf("DELETE FROM %s WHERE user_id = ? AND path = ?", storageTable)
				r.databaseExecutor().Exec(query, userId, fileName)
			}
		}

		if storageType == "OCI" {
			return r.ociDelete(userToken, fileName)
		} else {
			if err := os.Remove(fullPath); err != nil {
				return false
			}
			return true
		}
	}
	return nil
}

func safeUserStoragePath(basePath, userToken, fileName string) (string, error) {
	for _, value := range []string{userToken, fileName} {
		clean := filepath.Clean(value)
		if clean == "." || clean == ".." || filepath.IsAbs(value) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("token o nombre de archivo fuera del almacenamiento")
		}
	}
	root, err := filepath.Abs(basePath)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(root, userToken, fileName))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("token o nombre de archivo fuera del almacenamiento")
	}
	return target, nil
}

// --- OCI Helpers ---

func (r *Runtime) getOCIClient() (objectstorage.ObjectStorageClient, context.Context, error) {
	privateKeyPath := r.Env["OCI_PRIVATE_KEY_PATH"]
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return objectstorage.ObjectStorageClient{}, nil, err
	}

	passphrase := r.Env["OCI_PASSPHRASE"]
	confProvider := common.NewRawConfigurationProvider(
		r.Env["OCI_TENANCY_ID"],
		r.Env["OCI_USER_ID"],
		r.Env["OCI_REGION"],
		r.Env["OCI_FINGERPRINT"],
		string(privateKey),
		&passphrase,
	)

	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(confProvider)
	return client, context.Background(), err
}

func (r *Runtime) ociPut(userToken, fileName, content string) bool {
	fmt.Println("[OCI Debug] Initializing Client...")
	client, ctx, err := r.getOCIClient()
	if err != nil {
		fmt.Printf("[OCI Error] Client Init: %v\n", err)
		return false
	}

	// Add Timeout
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	namespace := r.Env["OCI_NAMESPACE"]
	bucketName := r.Env["OCI_BUCKET_NAME"]
	objectName := userToken + "/" + fileName // Use forward slash for OCI

	req := objectstorage.PutObjectRequest{
		NamespaceName: &namespace,
		BucketName:    &bucketName,
		ObjectName:    &objectName,
		PutObjectBody: io.NopCloser(strings.NewReader(content)),
		ContentLength: common.Int64(int64(len(content))),
	}

	fmt.Printf("[OCI Debug] Uploading %s (%d bytes)...\n", objectName, len(content))
	_, err = client.PutObject(ctx, req)
	if err != nil {
		fmt.Printf("[OCI Error] PutObject: %v\n", err)
		return false
	}
	fmt.Println("[OCI Debug] Upload Success!")
	return true
}

func (r *Runtime) ociGet(userToken, fileName string) interface{} {
	fmt.Printf("[OCI Debug] Getting object: userToken=%s, file=%s\n", userToken, fileName)
	client, ctx, err := r.getOCIClient()
	if err != nil {
		fmt.Printf("[OCI Error] Client init failed: %v\n", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	namespace := r.Env["OCI_NAMESPACE"]
	bucketName := r.Env["OCI_BUCKET_NAME"]
	objectName := userToken + "/" + fileName
	fmt.Printf("[OCI Debug] Fetching from bucket=%s, object=%s\n", bucketName, objectName)

	req := objectstorage.GetObjectRequest{
		NamespaceName: &namespace,
		BucketName:    &bucketName,
		ObjectName:    &objectName,
	}

	resp, err := client.GetObject(ctx, req)
	if err != nil {
		fmt.Printf("[OCI Error] GetObject failed: %v\n", err)
		return nil
	}
	defer resp.Content.Close()

	content, err := io.ReadAll(resp.Content)
	if err != nil {
		fmt.Printf("[OCI Error] ReadAll failed: %v\n", err)
		return nil
	}
	fmt.Printf("[OCI Debug] Downloaded %d bytes\n", len(content))
	return string(content)
}

// ociGetToFile downloads an OCI object directly to a local file path (binary-safe)
func (r *Runtime) ociGetToFile(userToken, fileName, destPath string) bool {
	fmt.Printf("[OCI Debug] GetToFile: userToken=%s, file=%s -> %s\n", userToken, fileName, destPath)
	client, ctx, err := r.getOCIClient()
	if err != nil {
		fmt.Printf("[OCI Error] Client init failed: %v\n", err)
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	namespace := r.Env["OCI_NAMESPACE"]
	bucketName := r.Env["OCI_BUCKET_NAME"]
	objectName := userToken + "/" + fileName
	fmt.Printf("[OCI Debug] Fetching from bucket=%s, object=%s\n", bucketName, objectName)

	req := objectstorage.GetObjectRequest{
		NamespaceName: &namespace,
		BucketName:    &bucketName,
		ObjectName:    &objectName,
	}

	resp, err := client.GetObject(ctx, req)
	if err != nil {
		fmt.Printf("[OCI Error] GetObject failed: %v\n", err)
		return false
	}
	defer resp.Content.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		fmt.Printf("[OCI Error] MkdirAll failed: %v\n", err)
		return false
	}

	// Write directly to file in binary mode
	out, err := os.Create(destPath)
	if err != nil {
		fmt.Printf("[OCI Error] File create failed: %v\n", err)
		return false
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Content)
	if err != nil {
		fmt.Printf("[OCI Error] File write failed: %v\n", err)
		return false
	}
	fmt.Printf("[OCI Debug] Wrote %d bytes to %s\n", written, destPath)
	return true
}

func (r *Runtime) ociDelete(userToken, fileName string) bool {
	client, ctx, err := r.getOCIClient()
	if err != nil {
		return false
	}

	namespace := r.Env["OCI_NAMESPACE"]
	bucketName := r.Env["OCI_BUCKET_NAME"]
	objectName := userToken + "/" + fileName

	req := objectstorage.DeleteObjectRequest{
		NamespaceName: &namespace,
		BucketName:    &bucketName,
		ObjectName:    &objectName,
	}

	_, err = client.DeleteObject(ctx, req)
	return err == nil
}

// Helper to get User ID
func (r *Runtime) getUserIdFromToken(usersTable, token string) int {
	if r.GetDB() == nil {
		return 0
	}
	var id int
	query := fmt.Sprintf("SELECT id FROM %s WHERE user_token = ? LIMIT 1", usersTable)
	err := r.databaseExecutor().QueryRow(query, token).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

var storageTablesEnsured sync.Map

func (r *Runtime) ensureStorageTable(tableName string) {
	db := r.GetDB()
	if db == nil {
		return
	}
	ensureKey := fmt.Sprintf("%p:%s", db, tableName)
	if _, exists := storageTablesEnsured.Load(ensureKey); exists {
		return
	}

	createCtx := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		path VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`, tableName)

	if val, ok := r.Env["DB"]; ok && val == "mysql" {
		createCtx = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT NOT NULL,
			path VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`, tableName)
	} else if normalizeDatabaseDriver(r.Env["DB"]) == "postgres" {
		createCtx = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			path VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`, tableName)
	}

	r.databaseExecutor().Exec(createCtx)
	storageTablesEnsured.Store(ensureKey, true)
}
