package main

// runEmbeddedRunnerIfSealed is deprecated; Joss compiles applications directly
// to pure standalone native machine code (PE32+, ELF64, Mach-O64) without
// embedding a Go host runner or VFS trailer.
func runEmbeddedRunnerIfSealed() bool {
	return false
}
