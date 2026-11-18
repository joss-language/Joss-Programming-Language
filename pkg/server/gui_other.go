//go:build !windows || nogui

package server

func maybeLaunchDesktopWindow(port string, isGUI bool) bool {
	return false
}
