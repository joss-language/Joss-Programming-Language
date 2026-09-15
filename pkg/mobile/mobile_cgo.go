//go:build cgo && !windows

package mobile

/*
#include <stdlib.h>
*/
import "C"
import "unsafe"

//export JossRun
func JossRun(source *C.char, timeoutMs C.int) *C.char {
	goSource := C.GoString(source)
	res := Run(goSource, int(timeoutMs))
	return C.CString(res)
}

//export JossAnalyze
func JossAnalyze(source *C.char) *C.char {
	goSource := C.GoString(source)
	res := Analyze(goSource)
	return C.CString(res)
}

//export JossVersion
func JossVersion() *C.char {
	return C.CString(Version())
}

//export JossFreeString
func JossFreeString(str *C.char) {
	if str != nil {
		C.free(unsafe.Pointer(str))
	}
}
