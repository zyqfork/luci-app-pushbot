package main

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ipFetchClient = &http.Client{Timeout: 8 * time.Second}

func getWanIPv4(cfg *Config) string {
	if cfg.IPv4Interface != "" {
		return getInterfaceIPv4(cfg.IPv4Interface)
	}
	return getFirstIPv4()
}

func getInterfaceIPv4(iface string) string {
	// ip addr show eth0 或读 /sys/class/net/eth0/address 等，简单用 net 包
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		if i.Name != iface {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getFirstIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getHostIPv4(cfg *Config) string {
	if len(cfg.IPv4URLList) == 0 {
		return ""
	}
	// 随机选一个 URL
	idx := time.Now().UnixNano() % int64(len(cfg.IPv4URLList))
	urlStr := strings.TrimSpace(cfg.IPv4URLList[idx])
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "curl/7.0")
	resp, err := ipFetchClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return ipV4Re.FindString(string(b))
}

func getWanIPv6(cfg *Config) string {
	if cfg.IPv6Interface != "" {
		return getInterfaceIPv6(cfg.IPv6Interface)
	}
	return getFirstIPv6()
}

func getInterfaceIPv6(iface string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		if i.Name != iface {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To16() != nil && ipnet.IP.To4() == nil && !ipnet.IP.IsLoopback() {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getFirstIPv6() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To16() != nil && ipnet.IP.To4() == nil && !ipnet.IP.IsLoopback() {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getHostIPv6(cfg *Config) string {
	if len(cfg.IPv6URLList) == 0 {
		return ""
	}
	idx := time.Now().UnixNano() % int64(len(cfg.IPv6URLList))
	urlStr := strings.TrimSpace(cfg.IPv6URLList[idx])
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", "curl/7.0")
	resp, err := ipFetchClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	// 简单取第一个 IPv6
	s := string(b)
	for _, r := range []string{"\n", " ", "\t"} {
		s = strings.ReplaceAll(s, r, " ")
	}
	parts := strings.Fields(s)
	for _, p := range parts {
		if strings.Contains(p, ":") && len(p) >= 4 {
			return p
		}
	}
	return ""
}

type ipState struct {
	IPv4 string
	IPv6 string
}

func loadIPState(dir string) *ipState {
	f, err := os.Open(filepath.Join(dir, "ip"))
	if err != nil {
		return nil
	}
	defer f.Close()
	b, _ := io.ReadAll(f)
	s := string(b)
	st := &ipState{}
	lines := strings.Split(s, "\n")
	for _, l := range lines {
		parts := strings.Fields(l)
		if len(parts) >= 2 {
			if parts[0] == "IPv4" {
				st.IPv4 = parts[1]
			} else if parts[0] == "IPv6" {
				st.IPv6 = parts[1]
			}
		}
	}
	return st
}

func saveIPState(dir, ipv4, ipv6 string) {
	_ = os.WriteFile(filepath.Join(dir, "ip"), []byte("IPv4 "+ipv4+"\nIPv6 "+ipv6+"\n"), 0644)
}

// checkIPChanges 检测公网 IP 变化，若有则追加通知并写回 ip 文件
func checkIPChanges(app *App) {
	cfg := app.cfg
	dir := cfg.Dir
	var ipv4, ipv6 string
	if cfg.PushbotIPv4 == 1 {
		ipv4 = getWanIPv4(cfg)
	} else if cfg.PushbotIPv4 == 2 {
		ipv4 = getHostIPv4(cfg)
	}
	if cfg.PushbotIPv6 == 1 {
		ipv6 = getWanIPv6(cfg)
	} else if cfg.PushbotIPv6 == 2 {
		ipv6 = getHostIPv6(cfg)
	}
	old := loadIPState(dir)
	if old == nil {
		saveIPState(dir, ipv4, ipv6)
		// 首次：可发“路由器已启动”
		if ipv4 != "" || ipv6 != "" {
			t := globalTemplate
			if t != nil {
				content := t.StrSplitline + t.StrTitleStart + t.FontGreen + " 路由器重新启动" + t.FontEnd + t.StrTitleEnd
				if ipv4 != "" {
					content += t.StrLinefeed + t.StrTab + "当前IP：" + ipv4
				}
				if ipv6 != "" {
					content += t.StrLinefeed + t.StrTab + "当前IPv6：" + ipv6
				}
				app.appendNotify(&Notify{Title: "路由器重新启动", Content: content})
			}
		}
		return
	}
	if cfg.PushbotIPv4 != 0 && ipv4 != "" && ipv4 != old.IPv4 {
		t := globalTemplate
		if t != nil {
			content := t.StrSplitline + t.StrTitleStart + t.FontGreen + " IP 地址变化" + t.FontEnd + t.StrTitleEnd + t.StrLinefeed + t.StrTab + "当前 IP：" + ipv4
			app.appendNotify(&Notify{Title: "IP 地址变化", Content: content})
		}
	}
	if cfg.PushbotIPv6 != 0 && ipv6 != "" && ipv6 != old.IPv6 {
		t := globalTemplate
		if t != nil {
			content := t.StrSplitline + t.StrTitleStart + t.FontGreen + " IPv6 地址变化" + t.FontEnd + t.StrTitleEnd + t.StrLinefeed + t.StrTab + "当前 IPv6：" + ipv6
			app.appendNotify(&Notify{Title: "IPv6 地址变化", Content: content})
		}
	}
	saveIPState(dir, ipv4, ipv6)
}
