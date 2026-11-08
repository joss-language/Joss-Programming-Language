package main

import (
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jossecurity/joss/pkg/crypto"
	"github.com/jossecurity/joss/pkg/diagnostics"
)

// `joss doctor storage` valida por capas la configuración de almacenamiento
// declarada en el entorno (STORAGE=local|OCI) y reporta cada hallazgo como
// diagnostics.Diagnostic. Nunca imprime secretos: sólo longitudes, fingerprints
// y OCIDs enmascarados.

var (
	ociFingerprintRe = regexp.MustCompile(`^([0-9a-f]{2}:){15}[0-9a-f]{2}$`)
	ociRegionRe      = regexp.MustCompile(`^[a-z]{2}-[a-z]+-[0-9]+$`)
)

// storageProbe abstrae las llamadas remotas para poder probar el diagnóstico
// sin red. La implementación real usa el SDK de OCI.
type storageProbe interface {
	Namespace() (string, error)
	BucketExists(namespace, bucket string) error
	ListBuckets(namespace, compartment string) ([]string, error)
}

// probeError describe un error remoto con su estado HTTP y código de servicio.
type probeError struct {
	Status  int
	Code    string
	Message string
}

func (e *probeError) Error() string {
	return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message)
}

type ociSDKProbe struct {
	env        map[string]string
	privateKey string
	client     *http.Client
}

func newOCISDKProbe(env map[string]string, privateKey []byte) (storageProbe, error) {
	return &ociSDKProbe{
		env:        env,
		privateKey: string(privateKey),
		client:     &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (p *ociSDKProbe) sendRequest(method, path string) (*http.Response, []byte, error) {
	region := p.env["OCI_REGION"]
	if region == "" {
		region = "mx-queretaro-1"
	}
	host := "objectstorage." + region + ".oraclecloud.com"
	date := time.Now().UTC().Format(http.TimeFormat)
	signingString := fmt.Sprintf("date: %s\n(request-target): %s %s\nhost: %s", date, strings.ToLower(method), path, host)
	sig, err := crypto.SignRSASHA256(signingString, p.privateKey)
	if err != nil {
		return nil, nil, err
	}
	keyId := fmt.Sprintf("%s/%s/%s", p.env["OCI_TENANCY_ID"], p.env["OCI_USER_ID"], p.env["OCI_FINGERPRINT"])
	authHeader := fmt.Sprintf(`Signature version="1",headers="date (request-target) host",keyId="%s",algorithm="rsa-sha256",signature="%s"`, keyId, sig)

	req, err := http.NewRequest(method, "https://"+host+path, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Host", host)
	req.Host = host
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "*/*")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var ociErr struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &ociErr)
		return resp, body, &probeError{
			Status:  resp.StatusCode,
			Code:    ociErr.Code,
			Message: ociErr.Message,
		}
	}
	return resp, body, nil
}

func (p *ociSDKProbe) Namespace() (string, error) {
	_, body, err := p.sendRequest("GET", "/n/")
	if err != nil {
		return "", err
	}
	var res struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &res); err == nil && res.Value != "" {
		return res.Value, nil
	}
	return strings.Trim(strings.TrimSpace(string(body)), `"`), nil
}

func (p *ociSDKProbe) BucketExists(namespace, bucket string) error {
	_, _, err := p.sendRequest("GET", "/n/"+url.PathEscape(namespace)+"/b/"+url.PathEscape(bucket))
	return err
}

func (p *ociSDKProbe) ListBuckets(namespace, compartment string) ([]string, error) {
	_, body, err := p.sendRequest("GET", "/n/"+url.PathEscape(namespace)+"/b?compartmentId="+url.QueryEscape(compartment))
	if err != nil {
		return nil, err
	}
	var res struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &res); err == nil && len(res.Items) > 0 {
		names := make([]string, 0, len(res.Items))
		for _, item := range res.Items {
			if item.Name != "" {
				names = append(names, item.Name)
			}
		}
		sort.Strings(names)
		return names, nil
	}
	var items []struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(body, &items)
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.Name != "" {
			names = append(names, item.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// maskOCID conserva el prefijo del recurso y los últimos caracteres.
func maskOCID(id string) string {
	if len(id) <= 24 {
		return "***"
	}
	return id[:strings.LastIndex(id, ".")+1] + "…" + id[len(id)-6:]
}

// ociKeyFingerprint calcula el fingerprint MD5 que OCI muestra para una API key.
// Devuelve encrypted=true cuando la llave está protegida con passphrase y, por
// lo tanto, no puede evaluarse localmente.
func ociKeyFingerprint(pemData []byte) (fingerprint string, encrypted bool, err error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return "", false, errors.New("el archivo no contiene un bloque PEM válido")
	}
	if block.Type == "ENCRYPTED PRIVATE KEY" || block.Headers["Proc-Type"] != "" {
		return "", true, nil
	}
	var parsed interface{}
	if key, perr := x509.ParsePKCS8PrivateKey(block.Bytes); perr == nil {
		parsed = key
	} else if key, perr := x509.ParsePKCS1PrivateKey(block.Bytes); perr == nil {
		parsed = key
	} else {
		return "", false, errors.New("la llave no es RSA en formato PKCS#1 ni PKCS#8")
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return "", false, errors.New("OCI sólo admite llaves RSA para firmar peticiones")
	}
	der, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return "", false, err
	}
	sum := md5.Sum(der)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":"), false, nil
}

func storageDiag(code string, sev diagnostics.Severity, msg, explanation, suggestion string) diagnostics.Diagnostic {
	return diagnostics.Diagnostic{
		Code:        code,
		Severity:    sev,
		Message:     msg,
		Explanation: explanation,
		Suggestion:  suggestion,
	}
}

// diagnoseStorage ejecuta las comprobaciones del proveedor activo. root es el
// directorio del proyecto contra el que se resuelven las rutas relativas.
func diagnoseStorage(env map[string]string, root string, newProbe func(env map[string]string, key []byte) (storageProbe, error)) []diagnostics.Diagnostic {
	provider := strings.ToLower(strings.TrimSpace(env["STORAGE"]))
	switch provider {
	case "", "local":
		return diagnoseLocalStorage(env, root)
	case "oci":
		return diagnoseOCIStorage(env, root, newProbe)
	case "s3", "aws", "azure":
		return []diagnostics.Diagnostic{storageDiag(
			diagnostics.CodeStorageProviderUnsupported, diagnostics.SeverityError,
			fmt.Sprintf("El proveedor STORAGE=%s todavía no tiene driver en el runtime de Joss.", env["STORAGE"]),
			"El runtime sólo implementa almacenamiento local y Oracle Cloud Infrastructure (OCI). Aunque OCI ofrece una API compatible con S3, Joss usa su API nativa.",
			"Use STORAGE=local o STORAGE=OCI.",
		)}
	default:
		return []diagnostics.Diagnostic{storageDiag(
			diagnostics.CodeStorageUnknownProvider, diagnostics.SeverityError,
			fmt.Sprintf("Valor de STORAGE desconocido: %q.", env["STORAGE"]),
			"STORAGE decide dónde guarda UserStorage los archivos.",
			"Use STORAGE=local o STORAGE=OCI.",
		)}
	}
}

func diagnoseLocalStorage(env map[string]string, root string) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	base := filepath.Join(root, "storage")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return append(out, storageDiag(
			diagnostics.CodeStorageLocalNotWritable, diagnostics.SeverityError,
			fmt.Sprintf("No se puede crear el directorio local de almacenamiento %q: %v.", base, err),
			"UserStorage escribe bajo ./storage cuando STORAGE=local.",
			"Revise los permisos del directorio del proyecto.",
		))
	}
	probe, err := os.CreateTemp(base, ".joss-doctor-*")
	if err != nil {
		return append(out, storageDiag(
			diagnostics.CodeStorageLocalNotWritable, diagnostics.SeverityError,
			fmt.Sprintf("El directorio %q no es escribible: %v.", base, err),
			"UserStorage escribe bajo ./storage cuando STORAGE=local.",
			"Revise los permisos o el espacio en disco.",
		))
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	out = append(out, storageDiag(diagnostics.CodeStorageOK, diagnostics.SeverityInfo,
		fmt.Sprintf("Almacenamiento local operativo en %s.", base), "", ""))
	for key := range env {
		if strings.HasPrefix(key, "OCI_") {
			out = append(out, storageDiag(diagnostics.CodeStorageOK, diagnostics.SeverityInfo,
				"Hay variables OCI_* definidas, pero STORAGE=local las ignora.", "",
				"Use STORAGE=OCI para activarlas."))
			break
		}
	}
	return out
}

func diagnoseOCIStorage(env map[string]string, root string, newProbe func(env map[string]string, key []byte) (storageProbe, error)) []diagnostics.Diagnostic {
	var out []diagnostics.Diagnostic
	add := func(d diagnostics.Diagnostic) { out = append(out, d) }

	required := []string{"OCI_NAMESPACE", "OCI_BUCKET_NAME", "OCI_TENANCY_ID", "OCI_USER_ID", "OCI_REGION", "OCI_FINGERPRINT", "OCI_PRIVATE_KEY_PATH"}
	missing := false
	for _, key := range required {
		if strings.TrimSpace(env[key]) == "" {
			missing = true
			add(storageDiag(diagnostics.CodeStorageConfig, diagnostics.SeverityError,
				fmt.Sprintf("Falta la variable %s.", key),
				"STORAGE=OCI necesita todas las variables OCI_* para firmar las peticiones.",
				fmt.Sprintf("Defina %s en el archivo de entorno.", key)))
		}
	}
	if missing {
		return out
	}

	formatOK := true
	checkFormat := func(ok bool, key, hint string) {
		if !ok {
			formatOK = false
			add(storageDiag(diagnostics.CodeStorageConfig, diagnostics.SeverityError,
				fmt.Sprintf("El valor de %s no tiene el formato esperado.", key), hint,
				"Copie el valor exacto desde la consola de OCI (Mi perfil → Claves de API → Ver archivo de configuración)."))
		}
	}
	checkFormat(strings.HasPrefix(env["OCI_USER_ID"], "ocid1.user."), "OCI_USER_ID", "Debe comenzar con ocid1.user.")
	checkFormat(strings.HasPrefix(env["OCI_TENANCY_ID"], "ocid1.tenancy."), "OCI_TENANCY_ID", "Debe comenzar con ocid1.tenancy.")
	checkFormat(ociFingerprintRe.MatchString(env["OCI_FINGERPRINT"]), "OCI_FINGERPRINT", "Debe ser 16 pares hexadecimales en minúscula separados por dos puntos.")
	checkFormat(ociRegionRe.MatchString(env["OCI_REGION"]), "OCI_REGION", "Debe tener la forma mx-queretaro-1.")

	keyPath := env["OCI_PRIVATE_KEY_PATH"]
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(root, keyPath)
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		add(storageDiag(diagnostics.CodeStorageKey, diagnostics.SeverityError,
			fmt.Sprintf("No se puede leer la llave privada %q: %v.", env["OCI_PRIVATE_KEY_PATH"], err),
			"OCI_PRIVATE_KEY_PATH debe apuntar al PEM privado, relativo al proyecto o absoluto.",
			"Corrija la ruta o copie el archivo."))
		return out
	}
	fingerprint, encrypted, ferr := ociKeyFingerprint(keyData)
	switch {
	case ferr != nil:
		add(storageDiag(diagnostics.CodeStorageKey, diagnostics.SeverityError,
			fmt.Sprintf("La llave privada no es utilizable: %v.", ferr),
			"El archivo debe ser el PEM privado de la API key, no la llave pública.",
			"Genere una API key nueva en OCI y descargue su llave privada."))
		return out
	case encrypted && env["OCI_PASSPHRASE"] == "":
		add(storageDiag(diagnostics.CodeStorageKey, diagnostics.SeverityError,
			"La llave privada está cifrada y no se definió OCI_PASSPHRASE.",
			"Sin passphrase el SDK no puede firmar las peticiones.",
			"Defina OCI_PASSPHRASE o use una llave sin cifrar."))
		return out
	case encrypted:
		add(storageDiag(diagnostics.CodeStorageOK, diagnostics.SeverityInfo,
			"La llave está cifrada; se omite la comprobación local del fingerprint.", "", ""))
	case strings.EqualFold(fingerprint, env["OCI_FINGERPRINT"]):
		add(storageDiag(diagnostics.CodeStorageOK, diagnostics.SeverityInfo,
			"El fingerprint de la llave coincide con OCI_FINGERPRINT.", "", ""))
	default:
		add(storageDiag(diagnostics.CodeStorageFingerprint, diagnostics.SeverityError,
			fmt.Sprintf("El fingerprint de la llave local (%s) no coincide con OCI_FINGERPRINT (%s).", fingerprint, env["OCI_FINGERPRINT"]),
			"OCI rechazará con 401 cualquier petición firmada con una llave distinta a la registrada.",
			fmt.Sprintf("Use OCI_FINGERPRINT=%s si esa llave está registrada en el usuario, o reemplace el PEM por el que corresponde al fingerprint configurado.", fingerprint)))
		formatOK = false
	}
	if !formatOK {
		return out
	}

	probe, err := newProbe(env, keyData)
	if err != nil {
		add(storageDiag(diagnostics.CodeStorageAuth, diagnostics.SeverityError,
			fmt.Sprintf("No se pudo crear el cliente de OCI: %v.", err),
			"El SDK rechazó la configuración antes de contactar a OCI.",
			"Revise la llave privada y la passphrase."))
		return out
	}

	namespace, err := probe.Namespace()
	if err != nil {
		var pe *probeError
		if errors.As(err, &pe) && pe.Status == 401 {
			add(storageDiag(diagnostics.CodeStorageAuth, diagnostics.SeverityError,
				fmt.Sprintf("OCI rechazó la autenticación (%s) para el usuario %s en el tenancy %s.", pe.Code, maskOCID(env["OCI_USER_ID"]), maskOCID(env["OCI_TENANCY_ID"])),
				"La llave, el fingerprint, el usuario y el tenancy son coherentes entre sí, pero OCI no encuentra esa API key para ese usuario.",
				"Compruebe que la clave pública esté en Mi perfil → Claves de API del usuario indicado. Si la subió hace menos de dos minutos, espere y repita."))
		} else {
			add(storageDiag(diagnostics.CodeStorageAuth, diagnostics.SeverityError,
				fmt.Sprintf("No se pudo consultar el namespace de OCI: %v.", err),
				"La petición no llegó a autenticarse.",
				"Revise la conexión a Internet, la región y el reloj del equipo."))
		}
		return out
	}
	add(storageDiag(diagnostics.CodeStorageOK, diagnostics.SeverityInfo, "Autenticación con OCI correcta.", "", ""))

	if namespace != env["OCI_NAMESPACE"] {
		add(storageDiag(diagnostics.CodeStorageNamespace, diagnostics.SeverityError,
			fmt.Sprintf("OCI_NAMESPACE=%s no corresponde a este tenancy; el namespace real es %s.", env["OCI_NAMESPACE"], namespace),
			"Cada tenancy tiene su propio namespace; uno ajeno devuelve errores engañosos.",
			fmt.Sprintf("Use OCI_NAMESPACE=%s.", namespace)))
	}

	if err := probe.BucketExists(namespace, env["OCI_BUCKET_NAME"]); err != nil {
		var pe *probeError
		if errors.As(err, &pe) && pe.Status == 404 {
			suggestion := "Cree el bucket o corrija OCI_BUCKET_NAME."
			if names, lerr := probe.ListBuckets(namespace, env["OCI_TENANCY_ID"]); lerr == nil && len(names) > 0 {
				suggestion = "Buckets existentes en el tenancy: " + strings.Join(names, ", ") + "."
			}
			add(storageDiag(diagnostics.CodeStorageBucket, diagnostics.SeverityError,
				fmt.Sprintf("El bucket %q no existe en el namespace %s.", env["OCI_BUCKET_NAME"], namespace),
				"Los nombres de bucket son únicos por namespace; si el bucket está en otra cuenta, necesita las credenciales de esa cuenta.",
				suggestion))
		} else {
			add(storageDiag(diagnostics.CodeStorageBucket, diagnostics.SeverityError,
				fmt.Sprintf("No se pudo acceder al bucket %q: %v.", env["OCI_BUCKET_NAME"], err),
				"El usuario podría no tener políticas de IAM sobre el bucket.",
				"Revise las políticas del grupo del usuario (manage object-family)."))
		}
		return out
	}
	add(storageDiag(diagnostics.CodeStorageOK, diagnostics.SeverityInfo,
		fmt.Sprintf("El bucket %q es accesible.", env["OCI_BUCKET_NAME"]), "", ""))
	return out
}

// printStorageDiagnostics imprime los hallazgos y devuelve true si no hubo errores.
func printStorageDiagnostics(items []diagnostics.Diagnostic) bool {
	ok := true
	for _, d := range items {
		if d.Severity == diagnostics.SeverityError {
			ok = false
		}
		prefix := "  ✓"
		if d.Severity == diagnostics.SeverityError {
			prefix = "  ✗"
		} else if d.Severity == diagnostics.SeverityWarning {
			prefix = "  !"
		}
		if d.Severity == diagnostics.SeverityInfo {
			fmt.Printf("%s %s\n", prefix, d.Message)
		} else {
			fmt.Printf("%s %s\n", prefix, d.String())
		}
		if d.Explanation != "" {
			fmt.Printf("      Por qué: %s\n", d.Explanation)
		}
		if d.Suggestion != "" {
			fmt.Printf("      Sugerencia: %s\n", d.Suggestion)
		}
	}
	return ok
}

// handleDoctor despacha `joss doctor [storage]`.
func handleDoctor(args []string) int {
	target := "storage"
	if len(args) > 0 {
		target = strings.ToLower(args[0])
	}
	if target != "storage" {
		fmt.Println("Uso: joss doctor [storage]")
		return 2
	}
	root, _ := os.Getwd()
	env := loadEnvMap()
	fmt.Printf("Diagnóstico de almacenamiento (STORAGE=%s)\n", strings.TrimSpace(env["STORAGE"]))
	if printStorageDiagnostics(diagnoseStorage(env, root, newOCISDKProbe)) {
		return 0
	}
	return 1
}
