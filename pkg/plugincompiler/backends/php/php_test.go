package php

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jossecurity/joss/pkg/plugincompiler/optimizer"
)

func TestPHPBackendTreeShaking(t *testing.T) {
	tempDir := t.TempDir()
	phpFile := filepath.Join(tempDir, "mailer.php")

	// Simular una biblioteca PHP con muchas funciones pero solo unas pocas usadas
	phpCode := `<?php
function sendMail($to, $subject, $body) {
    validateEmail($to);
    formatHeader($subject);
    return true;
}

function validateEmail($email) {
    return true;
}

function formatHeader($hdr) {
    return "Header: " . $hdr;
}

function unusedPop3Client() {
    return false;
}

function unusedImapSync() {
    return false;
}

function unusedAttachmentEncoder() {
    return "base64";
}

function unusedOauthProvider() {
    return null;
}
`
	if err := os.WriteFile(phpFile, []byte(phpCode), 0644); err != nil {
		t.Fatalf("failed to write php fixture: %v", err)
	}

	backend := NewPHPBackend()
	module, err := backend.Compile(phpFile, "phpmailer_lite", "1.0.0", []string{"sendMail"}, []string{"network"})
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Comprobar que inicialmente se extrajeron todas las funciones
	if len(module.Functions) != 7 {
		t.Fatalf("expected 7 original functions, got %d", len(module.Functions))
	}

	// Ejecutar Tree Shaking
	res, err := optimizer.TreeShake(module)
	if err != nil {
		t.Fatalf("unexpected tree shaking error: %v", err)
	}

	// Debe conservar solo sendMail, validateEmail y formatHeader (3 funciones)
	if res.OptimizedFuncs != 3 {
		t.Fatalf("expected 3 optimized functions (sendMail, validateEmail, formatHeader), got %d (removed %d)",
			res.OptimizedFuncs, res.RemovedFuncs)
	}

	if _, ok := res.Module.Functions["sendMail"]; !ok {
		t.Errorf("sendMail should be preserved")
	}
	if _, ok := res.Module.Functions["validateEmail"]; !ok {
		t.Errorf("validateEmail should be preserved")
	}
	if _, ok := res.Module.Functions["formatHeader"]; !ok {
		t.Errorf("formatHeader should be preserved")
	}
	if _, ok := res.Module.Functions["unusedPop3Client"]; ok {
		t.Errorf("unusedPop3Client should have been pruned by tree shaking")
	}
}
