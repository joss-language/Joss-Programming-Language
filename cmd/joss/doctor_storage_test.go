package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jossecurity/joss/pkg/diagnostics"
)

type fakeProbe struct {
	nsErr     error
	namespace string
	bucketErr error
	buckets   []string
}

func (f fakeProbe) Namespace() (string, error)                 { return f.namespace, f.nsErr }
func (f fakeProbe) BucketExists(ns, b string) error            { return f.bucketErr }
func (f fakeProbe) ListBuckets(ns, c string) ([]string, error) { return f.buckets, nil }

func probeFactory(p fakeProbe) func(map[string]string, []byte) (storageProbe, error) {
	return func(map[string]string, []byte) (storageProbe, error) { return p, nil }
}

func writeTestKey(t *testing.T, dir string) (path, fingerprint string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	path = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fp, encrypted, err := ociKeyFingerprint(data)
	if err != nil || encrypted {
		t.Fatalf("fingerprint: %v encrypted=%v", err, encrypted)
	}
	return path, fp
}

func ociEnv(keyPath, fingerprint string) map[string]string {
	return map[string]string{
		"STORAGE":              "OCI",
		"OCI_NAMESPACE":        "ns1",
		"OCI_BUCKET_NAME":      "bucket1",
		"OCI_TENANCY_ID":       "ocid1.tenancy.oc1..aaaaaaaaexampleexampleexampleexampleexample",
		"OCI_USER_ID":          "ocid1.user.oc1..aaaaaaaaexampleexampleexampleexampleexample",
		"OCI_REGION":           "mx-queretaro-1",
		"OCI_FINGERPRINT":      fingerprint,
		"OCI_PRIVATE_KEY_PATH": keyPath,
	}
}

func codes(items []diagnostics.Diagnostic) []string {
	var out []string
	for _, d := range items {
		if d.Severity == diagnostics.SeverityError {
			out = append(out, d.Code)
		}
	}
	return out
}

func hasCode(items []diagnostics.Diagnostic, code string) bool {
	for _, c := range codes(items) {
		if c == code {
			return true
		}
	}
	return false
}

func TestDoctorStorageLocalWritable(t *testing.T) {
	items := diagnoseStorage(map[string]string{"STORAGE": "local"}, t.TempDir(), nil)
	if got := codes(items); len(got) != 0 {
		t.Fatalf("local válido no debe tener errores: %v", got)
	}
}

func TestDoctorStorageLocalNotWritable(t *testing.T) {
	root := t.TempDir()
	// Un archivo ocupa el lugar del directorio ./storage.
	if err := os.WriteFile(filepath.Join(root, "storage"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	items := diagnoseStorage(map[string]string{}, root, nil)
	if !hasCode(items, diagnostics.CodeStorageLocalNotWritable) {
		t.Fatalf("se esperaba JOSS-STORAGE-008: %v", items)
	}
}

func TestDoctorStorageUnsupportedAndUnknown(t *testing.T) {
	if !hasCode(diagnoseStorage(map[string]string{"STORAGE": "s3"}, t.TempDir(), nil), diagnostics.CodeStorageProviderUnsupported) {
		t.Fatal("s3 debe reportar JOSS-STORAGE-009")
	}
	if !hasCode(diagnoseStorage(map[string]string{"STORAGE": "ftp"}, t.TempDir(), nil), diagnostics.CodeStorageUnknownProvider) {
		t.Fatal("proveedor desconocido debe reportar JOSS-STORAGE-001")
	}
}

func TestDoctorOCIMissingVariable(t *testing.T) {
	env := ociEnv("k.pem", "aa")
	delete(env, "OCI_NAMESPACE")
	if !hasCode(diagnoseStorage(env, t.TempDir(), nil), diagnostics.CodeStorageConfig) {
		t.Fatal("falta OCI_NAMESPACE debe reportar JOSS-STORAGE-004")
	}
}

func TestDoctorOCIFingerprintMismatch(t *testing.T) {
	dir := t.TempDir()
	keyPath, _ := writeTestKey(t, dir)
	env := ociEnv(keyPath, "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99")
	items := diagnoseStorage(env, dir, probeFactory(fakeProbe{}))
	if !hasCode(items, diagnostics.CodeStorageFingerprint) {
		t.Fatalf("se esperaba JOSS-STORAGE-003: %v", items)
	}
}

func TestDoctorOCIKeyErrors(t *testing.T) {
	dir := t.TempDir()
	_, fp := writeTestKey(t, dir)
	env := ociEnv(filepath.Join(dir, "no-existe.pem"), fp)
	if !hasCode(diagnoseStorage(env, dir, nil), diagnostics.CodeStorageKey) {
		t.Fatal("llave inexistente debe reportar JOSS-STORAGE-002")
	}
	bad := filepath.Join(dir, "bad.pem")
	_ = os.WriteFile(bad, []byte("no es pem"), 0o600)
	env["OCI_PRIVATE_KEY_PATH"] = bad
	if !hasCode(diagnoseStorage(env, dir, nil), diagnostics.CodeStorageKey) {
		t.Fatal("PEM inválido debe reportar JOSS-STORAGE-002")
	}
}

func TestDoctorOCIAuthNamespaceBucket(t *testing.T) {
	dir := t.TempDir()
	keyPath, fp := writeTestKey(t, dir)
	env := ociEnv(keyPath, fp)

	items := diagnoseStorage(env, dir, probeFactory(fakeProbe{nsErr: &probeError{Status: 401, Code: "NotAuthenticated"}}))
	if !hasCode(items, diagnostics.CodeStorageAuth) {
		t.Fatalf("401 debe reportar JOSS-STORAGE-005: %v", items)
	}

	items = diagnoseStorage(env, dir, probeFactory(fakeProbe{namespace: "otro-ns"}))
	if !hasCode(items, diagnostics.CodeStorageNamespace) {
		t.Fatalf("namespace distinto debe reportar JOSS-STORAGE-006: %v", items)
	}
	var found bool
	for _, d := range items {
		if d.Code == diagnostics.CodeStorageNamespace && strings.Contains(d.Suggestion, "otro-ns") {
			found = true
		}
	}
	if !found {
		t.Fatal("la sugerencia debe incluir el namespace real")
	}

	items = diagnoseStorage(env, dir, probeFactory(fakeProbe{
		namespace: "ns1",
		bucketErr: &probeError{Status: 404, Code: "BucketNotFound"},
		buckets:   []string{"a", "b"},
	}))
	if !hasCode(items, diagnostics.CodeStorageBucket) {
		t.Fatalf("404 debe reportar JOSS-STORAGE-007: %v", items)
	}
	for _, d := range items {
		if d.Code == diagnostics.CodeStorageBucket && !strings.Contains(d.Suggestion, "a, b") {
			t.Fatalf("la sugerencia debe listar los buckets existentes: %q", d.Suggestion)
		}
	}
}

func TestDoctorOCIHappyPath(t *testing.T) {
	dir := t.TempDir()
	keyPath, fp := writeTestKey(t, dir)
	items := diagnoseStorage(ociEnv(keyPath, fp), dir, probeFactory(fakeProbe{namespace: "ns1"}))
	if got := codes(items); len(got) != 0 {
		t.Fatalf("configuración válida no debe tener errores: %v", got)
	}
}

func TestDoctorNeverPrintsFullOCID(t *testing.T) {
	id := "ocid1.user.oc1..aaaaaaaacdvg4zkpzrtwn4hzovd7yqb6mlo7lh7asgkattir5pzhlhyy6mrq"
	masked := maskOCID(id)
	if strings.Contains(masked, "cdvg4zkpzrtwn4hzovd7") {
		t.Fatalf("OCID sin enmascarar: %s", masked)
	}
}
