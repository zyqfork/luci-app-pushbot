//go:build !linux

package main

// startNeighMonitor 非 Linux 平台不支持 Netlink NEIGH，返回 false
func startNeighMonitor(app *App) bool {
	return false
}
