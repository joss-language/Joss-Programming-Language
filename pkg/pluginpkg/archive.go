package pluginpkg

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	FormatVersion  = 2
	MetadataPath   = "META-INF/joss-plugin.json"
	SymbolsPath    = "META-INF/joss-symbols.json"
	MaxArchiveSize = 256 << 20
	MaxFileSize    = 128 << 20
)

type Metadata struct {
	Format             int                          `json:"format"`
	Name               string                       `json:"name"`
	Version            string                       `json:"version"`
	Language           string                       `json:"language,omitempty"`
	Bytecode           string                       `json:"bytecode"`
	Exports            []string                     `json:"exports,omitempty"`
	Permissions        []string                     `json:"permissions,omitempty"`
	Dependencies       map[string]string            `json:"dependencies,omitempty"`
	Native             map[string]string            `json:"native,omitempty"` // os-arch -> executable path
	ABI                map[string]string            `json:"abi,omitempty"`    // os-arch -> shared library path
	Protocol           string                       `json:"protocol,omitempty"`
	Symbols            string                       `json:"symbols,omitempty"`
	SignatureAlgorithm string                       `json:"signature_algorithm,omitempty"`
	PublicKey          string                       `json:"public_key,omitempty"`
	KeyID              string                       `json:"key_id,omitempty"`
	Signature          string                       `json:"signature,omitempty"`
	Commands           map[string]CommandDefinition `json:"commands,omitempty"`
}

type Archive struct {
	Metadata Metadata
	Files    map[string][]byte
}

func Build(metadata Metadata, files map[string][]byte) ([]byte, error) {
	return build(metadata, files)
}

// BuildSigned creates a reproducible JP v2 archive signed with Ed25519.
func BuildSigned(metadata Metadata, files map[string][]byte, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("jp: llave privada Ed25519 invalida")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	metadata.Format = FormatVersion
	keyHash := sha256.Sum256(publicKey)
	metadata.SignatureAlgorithm = "Ed25519"
	metadata.PublicKey = base64.StdEncoding.EncodeToString(publicKey)
	metadata.KeyID = hexKeyID(keyHash[:])
	metadata.Signature = ""
	digest, err := signatureDigest(metadata, files)
	if err != nil {
		return nil, err
	}
	metadata.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, digest))
	return build(metadata, files)
}

func build(metadata Metadata, files map[string][]byte) ([]byte, error) {
	metadata.Format = FormatVersion
	if metadata.Name == "" || metadata.Version == "" || metadata.Bytecode == "" {
		return nil, fmt.Errorf("jp: metadata requiere name, version y bytecode")
	}
	if _, ok := files[metadata.Bytecode]; !ok {
		return nil, fmt.Errorf("jp: falta bytecode %q", metadata.Bytecode)
	}
	if metadata.Symbols != "" {
		if _, ok := files[metadata.Symbols]; !ok {
			return nil, fmt.Errorf("jp: falta indice de simbolos %q", metadata.Symbols)
		}
	}
	if err := validateNativePayloads(metadata, files); err != nil {
		return nil, err
	}
	metaJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, err
	}
	all := make(map[string][]byte, len(files)+1)
	for name, content := range files {
		clean, err := cleanArchivePath(name)
		if err != nil {
			return nil, err
		}
		all[clean] = content
	}
	all[MetadataPath] = metaJSON

	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})
	fixedTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range names {
		content := all[name]
		if len(content) > MaxFileSize {
			_ = zw.Close()
			return nil, fmt.Errorf("jp: %s excede %d MiB", name, MaxFileSize>>20)
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: fixedTime}
		header.SetMode(0644)
		if isNativeTarget(metadata.Native, name) {
			header.SetMode(0755)
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := writer.Write(content); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if buffer.Len() > MaxArchiveSize {
		return nil, fmt.Errorf("jp: el paquete excede %d MiB", MaxArchiveSize>>20)
	}
	return buffer.Bytes(), nil
}

func Read(data []byte) (*Archive, error) {
	if len(data) > MaxArchiveSize {
		return nil, fmt.Errorf("jp: el paquete excede %d MiB", MaxArchiveSize>>20)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("jp: ZIP v2 invalido: %w", err)
	}
	files := make(map[string][]byte, len(zr.File))
	total := 0
	for _, file := range zr.File {
		clean, err := cleanArchivePath(file.Name)
		if err != nil {
			return nil, err
		}
		if _, duplicate := files[clean]; duplicate {
			return nil, fmt.Errorf("jp: entrada duplicada %q", clean)
		}
		if file.UncompressedSize64 > MaxFileSize {
			return nil, fmt.Errorf("jp: %s excede %d MiB", clean, MaxFileSize>>20)
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, MaxFileSize+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(content) > MaxFileSize {
			return nil, fmt.Errorf("jp: %s excede %d MiB", clean, MaxFileSize>>20)
		}
		total += len(content)
		if total > MaxArchiveSize {
			return nil, fmt.Errorf("jp: contenido expandido excede %d MiB", MaxArchiveSize>>20)
		}
		files[clean] = content
	}
	metaData, ok := files[MetadataPath]
	if !ok {
		return nil, fmt.Errorf("jp: falta %s", MetadataPath)
	}
	var metadata Metadata
	if err := json.Unmarshal(metaData, &metadata); err != nil {
		return nil, fmt.Errorf("jp: metadata invalida: %w", err)
	}
	if metadata.Format != FormatVersion {
		return nil, fmt.Errorf("jp: version de formato %d no soportada", metadata.Format)
	}
	if metadata.Name == "" || metadata.Version == "" || metadata.Bytecode == "" {
		return nil, fmt.Errorf("jp: metadata incompleta")
	}
	if _, ok := files[metadata.Bytecode]; !ok {
		return nil, fmt.Errorf("jp: falta bytecode %q", metadata.Bytecode)
	}
	if metadata.Symbols != "" {
		if _, ok := files[metadata.Symbols]; !ok {
			return nil, fmt.Errorf("jp: falta indice de simbolos %q", metadata.Symbols)
		}
	}
	for target, executable := range metadata.Native {
		if strings.TrimSpace(target) == "" {
			return nil, fmt.Errorf("jp: target nativo vacio")
		}
		// Remote URLs are downloaded at runtime — skip file presence check
		if strings.HasPrefix(executable, "https://") || strings.HasPrefix(executable, "http://") {
			continue
		}
		if _, ok := files[executable]; !ok {
			return nil, fmt.Errorf("jp: target %s apunta a %q inexistente", target, executable)
		}
	}
	for target, library := range metadata.ABI {
		if strings.TrimSpace(target) == "" {
			return nil, fmt.Errorf("jp: target ABI vacio")
		}
		if _, ok := files[library]; !ok {
			return nil, fmt.Errorf("jp: target ABI %s apunta a %q inexistente", target, library)
		}
	}
	if err := validateNativePayloads(metadata, files); err != nil {
		return nil, err
	}
	if metadata.Signature != "" {
		if err := verifySignature(metadata, files); err != nil {
			return nil, err
		}
	}
	return &Archive{Metadata: metadata, Files: files}, nil
}

// ReadVerified rejects unsigned JP archives and verifies signed archives.
func ReadVerified(data []byte) (*Archive, error) {
	archive, err := Read(data)
	if err != nil {
		return nil, err
	}
	if archive.Metadata.Signature == "" {
		return nil, fmt.Errorf("jp: paquete sin firma de autor Ed25519")
	}
	return archive, nil
}

func verifySignature(metadata Metadata, files map[string][]byte) error {
	if metadata.SignatureAlgorithm != "Ed25519" {
		return fmt.Errorf("jp: algoritmo de firma %q no soportado", metadata.SignatureAlgorithm)
	}
	publicKey, err := base64.StdEncoding.DecodeString(metadata.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("jp: llave publica Ed25519 invalida")
	}
	keyHash := sha256.Sum256(publicKey)
	if metadata.KeyID != hexKeyID(keyHash[:]) {
		return fmt.Errorf("jp: key_id no coincide con la llave publica")
	}
	signature, err := base64.StdEncoding.DecodeString(metadata.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("jp: firma Ed25519 invalida")
	}
	digest, err := signatureDigest(metadata, files)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), digest, signature) {
		return fmt.Errorf("jp: la firma no corresponde al contenido del paquete")
	}
	return nil
}

func signatureDigest(metadata Metadata, files map[string][]byte) ([]byte, error) {
	metadata.Signature = ""
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	writeDigestPart(hash, "metadata", metaJSON)
	normalized := make(map[string][]byte, len(files))
	names := make([]string, 0, len(files))
	for name, content := range files {
		clean, err := cleanArchivePath(name)
		if err != nil {
			return nil, err
		}
		if clean != MetadataPath {
			if _, duplicate := normalized[clean]; duplicate {
				return nil, fmt.Errorf("jp: entrada duplicada %q", clean)
			}
			normalized[clean] = content
			names = append(names, clean)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		writeDigestPart(hash, name, normalized[name])
	}
	return hash.Sum(nil), nil
}

func writeDigestPart(writer io.Writer, name string, content []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(name)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write([]byte(name))
	binary.BigEndian.PutUint64(length[:], uint64(len(content)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(content)
}

func hexKeyID(value []byte) string {
	const alphabet = "0123456789abcdef"
	result := make([]byte, 24)
	for index := 0; index < 12; index++ {
		result[index*2] = alphabet[value[index]>>4]
		result[index*2+1] = alphabet[value[index]&0x0f]
	}
	return string(result)
}

func IsV2(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K'
}

func cleanArchivePath(name string) (string, error) {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if clean == "." || clean == ".." || path.IsAbs(clean) || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("jp: ruta insegura %q", name)
	}
	return clean, nil
}

func isNativeTarget(targets map[string]string, name string) bool {
	for _, executable := range targets {
		if executable == name {
			return true
		}
	}
	return false
}

// LoadOrCreateSigningKey retrieves or creates an Ed25519 author signing key.
func LoadOrCreateSigningKey(pluginName string) (ed25519.PrivateKey, string, error) {
	configured := strings.TrimSpace(os.Getenv("JOSS_PLUGIN_SIGNING_KEY"))
	keyPath := configured
	if keyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", err
		}
		safeName := regexp.MustCompile(`[^A-Za-z0-9_.-]+`).ReplaceAllString(pluginName, "_")
		if safeName == "" {
			safeName = "default"
		}
		keyPath = filepath.Join(home, ".joss", "keys", safeName+".ed25519")
	}
	if content, err := os.ReadFile(keyPath); err == nil {
		raw := strings.TrimSpace(string(content))
		decoded, decodeErr := base64.StdEncoding.DecodeString(raw)
		if decodeErr == nil && len(decoded) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(decoded), keyPath, nil
		}
		if len(content) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(content), keyPath, nil
		}
		return nil, keyPath, fmt.Errorf("llave Ed25519 invalida en %s", keyPath)
	} else if !os.IsNotExist(err) {
		return nil, keyPath, err
	}
	if configured != "" {
		return nil, keyPath, fmt.Errorf("JOSS_PLUGIN_SIGNING_KEY no existe: %s", keyPath)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, keyPath, err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, keyPath, err
	}
	if err := os.WriteFile(keyPath, []byte(base64.StdEncoding.EncodeToString(privateKey)+"\n"), 0600); err != nil {
		return nil, keyPath, err
	}
	return privateKey, keyPath, nil
}
