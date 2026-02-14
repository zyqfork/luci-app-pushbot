package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DeviceInfo struct {
	IP        string
	MAC       string
	Name      string
	Timestamp int64
	Interface string
}

func readIPAddress(path string) []DeviceInfo {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var list []DeviceInfo
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			ts, _ := strconv.ParseInt(parts[3], 10, 64)
			inf := ""
			if len(parts) >= 5 {
				inf = parts[4]
			}
			list = append(list, DeviceInfo{
				IP: parts[0], MAC: parts[1], Name: parts[2],
				Timestamp: ts, Interface: inf,
			})
		}
	}
	return list
}

func writeIPAddress(path string, list []DeviceInfo) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, d := range list {
		_, _ = fmt.Fprintf(f, "%s %s %s %d %s\n", d.IP, d.MAC, d.Name, d.Timestamp, d.Interface)
	}
	return nil
}

// getMAC 从 ipAddress、tmp_downlist、dhcp.leases、arp 获取 MAC
func getMAC(cfg *Config, ip string) string {
	dir := cfg.Dir
	// 缓存
	for _, p := range []string{filepath.Join(dir, "ipAddress"), filepath.Join(dir, "tmp_downlist")} {
		if mac := grepField(p, ip, 2); mac != "" {
			return mac
		}
	}
	if mac := grepField("/var/dhcp.leases", ip, 2); mac != "" {
		return mac
	}
	if mac := arpGetMAC(ip); mac != "" {
		return mac
	}
	return "unknown"
}

func grepField(path, matchCol1 string, fieldIndex int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) > fieldIndex && parts[0] == matchCol1 {
			return parts[fieldIndex-1]
		}
	}
	return ""
}

func arpGetMAC(ip string) string {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan() // header
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) >= 6 && parts[0] == ip && (parts[2] == "0x2" || parts[2] == "0x6") {
			mac := parts[3]
			if mac != "00:00:00:00:00:00" {
				return mac
			}
		}
	}
	return ""
}

// getName 从 device_aliases、ipAddress、dhcp、UCI dhcp、OUI 获取名称
func getName(cfg *Config, ip, mac string) string {
	for _, line := range cfg.DeviceAliases {
		idx := strings.Index(line, " ")
		if idx <= 0 {
			continue
		}
		m := strings.TrimSpace(line[:idx])
		if strings.EqualFold(m, mac) {
			return strings.TrimSpace(line[idx+1:])
		}
	}
	dir := cfg.Dir
	for _, p := range []string{filepath.Join(dir, "ipAddress"), filepath.Join(dir, "tmp_downlist")} {
		if n := grepField(p, ip, 3); n != "" {
			return n
		}
	}
	if n := grepField("/var/dhcp.leases", ip, 4); n != "" {
		return n
	}
	// UCI dhcp 可选
	if n := uciDHCPName(ip); n != "" {
		return n
	}
	if mac != "unknown" {
		if n := ouiLookup(mac, cfg); n != "" {
			return n
		}
	}
	return "unknown"
}

func uciDHCPName(ip string) string {
	cmd := exec.Command("uci", "show", "dhcp")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if !strings.Contains(l, ".ip=") && !strings.Contains(l, ".ip='") {
			continue
		}
		if !strings.Contains(l, ip) {
			continue
		}
		i := strings.Index(l, "=")
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(l[:i])
		// key 形如 dhcp.@host[0].ip
		sec := strings.TrimSuffix(key, ".ip")
		if sec == key {
			continue
		}
		name := uciGetFull(sec + ".name")
		if name != "" {
			return name
		}
	}
	return ""
}

func uciGetFull(path string) string {
	cmd := exec.Command("uci", "-q", "get", path)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// ouiSearchPaths 查找顺序：/usr/share/pushbot（包安装）→ 配置目录 → 临时目录
var ouiSearchPaths = []string{"/usr/share/pushbot", ""}

func ouiLookup(mac string, cfg *Config) string {
	if len(mac) < 8 {
		return ""
	}
	oui := strings.ToLower(strings.ReplaceAll(mac[:8], ":", ""))
	bases := make([]string, 0, 4)
	for _, p := range ouiSearchPaths {
		if p != "" {
			bases = append(bases, p)
		}
	}
	bases = append(bases, cfg.ConfigDir, cfg.Dir)
	for _, base := range bases {
		for _, name := range []string{"oui_base.txt", "oui.txt"} {
			ouiPath := filepath.Join(base, name)
			f, err := os.Open(ouiPath)
			if err != nil {
				continue
			}
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				line := sc.Text()
				idx := strings.Index(line, "(base 16)")
				if idx <= 0 {
					continue
				}
				// 行首 OUI 可能为 XX-XX-XX 或 XX:XX:XX，统一与 oui 比较
				pre := line
				if idx < len(line) {
					pre = line[:idx]
				}
				prefix := strings.ReplaceAll(strings.ReplaceAll(pre, "-", ""), ":", "")
				prefix = strings.ToLower(strings.TrimSpace(prefix))
				if len(prefix) >= 6 && prefix[:6] == oui {
					rest := line[idx+9:]
					parts := strings.Split(rest, "\t")
					name := strings.TrimSpace(parts[len(parts)-1])
					f.Close()
					return strings.ReplaceAll(name, " ", "_")
				}
			}
			f.Close()
		}
	}
	return ""
}

// getInterface 从 ipAddress、arp、iw 获取接口
func getInterface(cfg *Config, mac string) string {
	dir := cfg.Dir
	if s := grepFieldByMAC(filepath.Join(dir, "ipAddress"), mac, 5); s != "" {
		return s
	}
	if s := grepFieldByMAC(filepath.Join(dir, "tmp_downlist"), mac, 5); s != "" {
		return s
	}
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan()
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) >= 6 && strings.EqualFold(parts[3], mac) {
			return parts[5]
		}
	}
	return ""
}

func grepFieldByMAC(path, mac string, fieldIndex int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) >= fieldIndex && strings.EqualFold(parts[1], mac) {
			return parts[fieldIndex-1]
		}
	}
	return ""
}

// blackwhitelist 返回 0 表示应推送（通过检查），非 0 表示不推送
func blackwhitelist(cfg *Config, mac string) int {
	if cfg.PushbotWhitelist != "" || cfg.PushbotBlacklist != "" || cfg.PushbotInterface != "" ||
		cfg.MACOnlineList != "" || cfg.MACOfflineList != "" {
		// 白名单：仅在名单内才推送
		for _, m := range cfg.PushbotWhitelistLines {
			if strings.TrimSpace(m) != "" && strings.EqualFold(m, mac) {
				return 0
			}
		}
		if cfg.PushbotWhitelist != "" {
			return 1
		}
		// 黑名单：在名单内不推送
		for _, m := range cfg.PushbotBlacklistLines {
			if strings.TrimSpace(m) != "" && strings.EqualFold(m, mac) {
				return 1
			}
		}
		// 接口过滤等可再扩展
	}
	return 0
}

// pingOK 检测 IP 是否在线（arping 或 ping）
func pingOK(ip string, timeoutSec, retry int) bool {
	// 先尝试 arping
	if iface := arpGetIface(ip); iface != "" {
		cmd := exec.Command("arping", "-I", iface, "-c", "3", "-w", strconv.Itoa(timeoutSec), ip)
		cmd.Run()
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0 {
			return true
		}
	}
	for i := 0; i < retry; i++ {
		cmd := exec.Command("ping", "-c", "2", "-W", strconv.Itoa(timeoutSec), ip)
		if cmd.Run() == nil {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func arpGetIface(ip string) string {
	f, _ := os.Open("/proc/net/arp")
	if f == nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan()
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) >= 6 && parts[0] == ip {
			return parts[5]
		}
	}
	return ""
}

// 扫描当前在线设备并更新 ipAddress；发送上线通知
func pushbotFirst(app *App) {
	cfg := app.cfg
	dir := cfg.Dir
	ipPath := filepath.Join(dir, "ipAddress")
	current := readIPAddress(ipPath)

	// 一次读取 ARP，得到候选 IP 与 ip->mac，避免每 IP 再读一次
	arpIPs, ipToMac := readArpOnce()
	var stillOnline []DeviceInfo
	for _, d := range current {
		if pingOK(d.IP, cfg.DownTimeout, cfg.TimeoutRetry) {
			stillOnline = append(stillOnline, d)
		} else {
			app.tmpDownMu.Lock()
			app.tmpDownList = append(app.tmpDownList, d)
			app.tmpDownMu.Unlock()
		}
	}
	var newList []DeviceInfo
	seenIP := make(map[string]bool)
	for _, d := range stillOnline {
		seenIP[d.IP] = true
		newList = append(newList, d)
	}
	for _, ip := range arpIPs {
		if seenIP[ip] {
			continue
		}
		if !pingOK(ip, cfg.UpTimeout, cfg.TimeoutRetry) {
			continue
		}
		mac := ipToMac[ip]
		if mac == "" {
			mac = getMAC(cfg, ip)
		}
		name := getName(cfg, ip, mac)
		iface := getInterface(cfg, mac)
		if blackwhitelist(cfg, mac) != 0 {
			continue
		}
		seenIP[ip] = true
		newList = append(newList, DeviceInfo{
			IP: ip, MAC: mac, Name: name,
			Timestamp: time.Now().Unix(), Interface: iface,
		})
		if cfg.PushbotUp == 1 {
			app.appendNotify(notifyUp(ip, mac, name, iface, cfg))
		}
	}
	_ = writeIPAddress(ipPath, newList)
}

// readArpOnce 一次读取 /proc/net/arp，返回候选 IP 列表和 ip->mac 映射，避免多次打开文件
func readArpOnce() (ips []string, ipToMac map[string]string) {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	ipToMac = make(map[string]string)
	sc := bufio.NewScanner(f)
	sc.Scan()
	for sc.Scan() {
		parts := strings.Fields(sc.Text())
		if len(parts) < 6 || (parts[2] != "0x2" && parts[2] != "0x6") {
			continue
		}
		ip, mac := parts[0], parts[3]
		if strings.HasPrefix(ip, "169.254.") || mac == "00:00:00:00:00:00" {
			continue
		}
		ips = append(ips, ip)
		ipToMac[ip] = mac
	}
	return ips, ipToMac
}

func arpIPList() []string {
	ips, _ := readArpOnce()
	return ips
}

// 处理离线列表并追加下线通知
func downSend(app *App) {
	app.tmpDownMu.Lock()
	list := app.tmpDownList
	app.tmpDownList = nil
	app.tmpDownMu.Unlock()
	cfg := app.cfg
	for _, d := range list {
		if blackwhitelist(cfg, d.MAC) != 0 {
			continue
		}
		if cfg.PushbotDown != 1 {
			continue
		}
		onlineSec := time.Now().Unix() - d.Timestamp
		app.appendNotify(notifyDown(d.IP, d.MAC, d.Name, timeForHumans(int(onlineSec)), cfg))
	}
}

var appNotifyMu sync.Mutex

func (a *App) appendNotify(n *Notify) {
	if n == nil {
		return
	}
	appNotifyMu.Lock()
	if a.pendingTitle == "" {
		a.pendingTitle = n.Title
	}
	a.pendingContent += n.Content
	appNotifyMu.Unlock()
	// 有待发通知时唤醒主循环，避免空转轮询
	if a.wakeCh != nil {
		select {
		case a.wakeCh <- struct{}{}:
		default:
		}
	}
}

func (a *App) consumeNotify() (title, content string) {
	appNotifyMu.Lock()
	title, content = a.pendingTitle, a.pendingContent
	a.pendingTitle = ""
	a.pendingContent = ""
	appNotifyMu.Unlock()
	return title, content
}
