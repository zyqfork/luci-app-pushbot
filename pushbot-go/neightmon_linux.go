//go:build linux

package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Linux netlink 邻居表常量（与 kernel uapi 一致）
const (
	_netlinkRoute  = 0
	_rtmNewNeigh   = 28
	_rtmDelNeigh   = 29
	_rtmgrpNeigh   = 4
	_ndaDst        = 1
	_ndaLladdr     = 2
	_ndaIfindex    = 8
	_nudReachable  = 0x02
	_nudStale      = 0x04
	_nudFailed     = 0x20
	_nudNoarp      = 0x40
	_nudPermanent  = 0x80
)

// NeighEvent 邻居表事件
type NeighEvent struct {
	IP    string
	MAC   string
	Dev   string
	IsNew bool // true=上线(REACHABLE等), false=下线(DEL/FAILED)
}

// startNeighMonitor 订阅 Netlink NEIGH，有事件时更新 ipAddress 并触发上下线推送
func startNeighMonitor(app *App) bool {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, _netlinkRoute)
	if err != nil {
		return false
	}
	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: _rtmgrpNeigh,
	}
	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return false
	}
	go runNeighMonitor(app, fd)
	return true
}

func runNeighMonitor(app *App, fd int) {
	defer syscall.Close(fd)
	buf := make([]byte, 1<<20)
	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			return
		}
		msgs, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			continue
		}
		for _, m := range msgs {
			if m.Header.Type != _rtmNewNeigh && m.Header.Type != _rtmDelNeigh {
				continue
			}
			ev := parseNeighMsg(m)
			if ev == nil {
				continue
			}
			if ev.IsNew {
				// 仅处理 IPv4，排除 169.254、全零 MAC
				if !isIPv4Private(ev.IP) || ev.MAC == "00:00:00:00:00:00" {
					continue
				}
			}
			handleNeighEvent(app, ev)
		}
	}
}


func parseNeighMsg(m syscall.NetlinkMessage) *NeighEvent {
	// nlmsghdr + ndmsg: family(1)+pad1(1)+pad2(2)+ifindex(4)+state(2)+flags(1)+type(1)=12
	if len(m.Data) < 12 {
		return nil
	}
	state := binary.NativeEndian.Uint16(m.Data[8:10])
	if m.Header.Type == _rtmDelNeigh {
		ev := parseNeighAttrs(m.Data[12:])
		if ev != nil {
			ev.IsNew = false
			return ev
		}
		return nil
	}
	if m.Header.Type == _rtmNewNeigh {
		ev := parseNeighAttrs(m.Data[12:])
		if ev == nil {
			return nil
		}
		if state == _nudFailed || state&_nudNoarp != 0 {
			ev.IsNew = false
		} else if state&_nudReachable != 0 || state&_nudStale != 0 || state&_nudPermanent != 0 {
			ev.IsNew = true
		} else {
			return nil // PROBE 等暂不处理，等 REACHABLE
		}
		return ev
	}
	return nil
}

func parseNeighAttrs(data []byte) *NeighEvent {
	ev := &NeighEvent{}
	for len(data) >= 4 {
		attrLen := int(binary.NativeEndian.Uint16(data[0:2]))
		attrType := int(data[2])
		if attrLen < 4 || len(data) < attrLen {
			break
		}
		payload := data[4:attrLen]
		switch attrType {
		case _ndaDst:
			if len(payload) == 4 {
				ev.IP = net.IP(payload).To4().String()
			} else if len(payload) == 16 {
				ev.IP = net.IP(payload).String()
			}
		case _ndaLladdr:
			if len(payload) >= 6 {
				ev.MAC = net.HardwareAddr(payload[:6]).String()
			}
		case _ndaIfindex:
			if len(payload) >= 4 {
				idx := binary.NativeEndian.Uint32(payload)
				ev.Dev = ifIndexToName(int(idx))
			}
		}
		data = data[attrLen:]
		if attrLen%4 != 0 {
			data = data[4-attrLen%4:]
		}
	}
	if ev.IP == "" || ev.MAC == "" {
		return nil
	}
	return ev
}

func ifIndexToName(idx int) string {
	// 简单读 /sys/class/net/
	// 不依赖 net 包遍历，直接读目录
	ents, _ := os.ReadDir("/sys/class/net")
	for _, e := range ents {
		b, _ := os.ReadFile("/sys/class/net/" + e.Name() + "/ifindex")
		var i int
		fmt.Sscanf(string(b), "%d", &i)
		if i == idx {
			return e.Name()
		}
	}
	return ""
}

func isIPv4Private(ip string) bool {
	if len(ip) < 7 {
		return false
	}
	// 169.254.x.x 排除
	if ip[:7] == "169.254" {
		return false
	}
	// 仅接受内网 10.x, 172.16-31.x, 192.168.x
	if strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "192.168.") {
		return true
	}
	if strings.HasPrefix(ip, "172.") {
		var a, b, c, d int
		fmt.Sscanf(ip, "%d.%d.%d.%d", &a, &b, &c, &d)
		return b >= 16 && b <= 31
	}
	return false
}

func handleNeighEvent(app *App, ev *NeighEvent) {
	cfg := app.cfg
	ipPath := filepath.Join(cfg.Dir, "ipAddress")
	ipAddrMu.Lock()
	list := readIPAddress(ipPath)
	ipAddrMu.Unlock()

	if ev.IsNew {
		// 已在列表中则忽略（仅刷新）
		for _, d := range list {
			if d.IP == ev.IP {
				return
			}
		}
		if blackwhitelist(cfg, ev.MAC) != 0 {
			return
		}
		name := getName(cfg, ev.IP, ev.MAC)
		inf := ev.Dev
		if inf == "" {
			inf = getInterface(cfg, ev.MAC)
		}
		d := DeviceInfo{
			IP: ev.IP, MAC: ev.MAC, Name: name,
			Timestamp: time.Now().Unix(), Interface: inf,
		}
		ipAddrMu.Lock()
		list = readIPAddress(ipPath)
		list = append(list, d)
		_ = writeIPAddress(ipPath, list)
		ipAddrMu.Unlock()
		if cfg.PushbotUp == 1 {
			app.appendNotify(notifyUp(ev.IP, ev.MAC, name, inf, cfg))
		}
	} else {
		var newList []DeviceInfo
		var found bool
		for _, d := range list {
			if d.IP == ev.IP {
				found = true
				app.tmpDownMu.Lock()
				app.tmpDownList = append(app.tmpDownList, d)
				app.tmpDownMu.Unlock()
				if cfg.PushbotDown == 1 && blackwhitelist(cfg, d.MAC) == 0 {
					onlineTime := timeForHumans(int(time.Now().Unix() - d.Timestamp))
					app.appendNotify(notifyDown(d.IP, d.MAC, d.Name, onlineTime, cfg))
				}
			} else {
				newList = append(newList, d)
			}
		}
		if found {
			ipAddrMu.Lock()
			_ = writeIPAddress(ipPath, newList)
			ipAddrMu.Unlock()
		}
	}
}

var ipAddrMu sync.Mutex
