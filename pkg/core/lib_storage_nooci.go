//go:build nooci

package core

import "fmt"

func (r *Runtime) ociPut(userToken, fileName, content string) bool {
	fmt.Println("[OCI] Soporte OCI Object Storage omitido en este binario optimizado (nooci).")
	return false
}

func (r *Runtime) ociGet(userToken, fileName string) interface{} {
	fmt.Println("[OCI] Soporte OCI Object Storage omitido en este binario optimizado (nooci).")
	return nil
}

func (r *Runtime) ociGetToFile(userToken, fileName, destPath string) bool {
	fmt.Println("[OCI] Soporte OCI Object Storage omitido en este binario optimizado (nooci).")
	return false
}

func (r *Runtime) ociDelete(userToken, fileName string) bool {
	fmt.Println("[OCI] Soporte OCI Object Storage omitido en este binario optimizado (nooci).")
	return false
}
