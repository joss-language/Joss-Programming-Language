package nativeruntime

import _ "embed"

// SourceC contiene el código fuente C del runtime nativo mínimo de Joss.
//
//go:embed csrc/joss_rt.c
var SourceC []byte

// HeaderH contiene la cabecera C con las declaraciones ABI del runtime.
//
//go:embed csrc/joss_rt.h
var HeaderH []byte
