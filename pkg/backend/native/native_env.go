package native

// readBuildTimeEnv returns an empty string by default so build-machine or
// developer environment files (.env, env.joss, env.enc) are never accidentally
// embedded into compiled native executables. Compiled binaries load environment
// configuration from the target runtime environment instead.
func readBuildTimeEnv() string {
	return ""
}
