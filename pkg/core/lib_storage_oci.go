//go:build !nooci

package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

func (r *Runtime) getOCIClient() (objectstorage.ObjectStorageClient, context.Context, error) {
	privateKeyPath := r.Env["OCI_PRIVATE_KEY_PATH"]
	privateKey, err := ReadFileBytes(privateKeyPath)
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

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	namespace := r.Env["OCI_NAMESPACE"]
	bucketName := r.Env["OCI_BUCKET_NAME"]
	objectName := userToken + "/" + fileName

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

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		fmt.Printf("[OCI Error] MkdirAll failed: %v\n", err)
		return false
	}

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
