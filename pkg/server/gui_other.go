//go:build !windows

package server

func maybeLaunchDesktopWindow(port string, isGUI bool) bool {
	return false
}
