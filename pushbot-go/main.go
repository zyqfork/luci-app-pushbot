package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// 非设备检测的轮询间隔，降低 CPU/网络占用
const (
	intervalIP         = 5 * time.Minute // 公网 IP 检测间隔
	intervalCPULoad    = 5 * time.Minute // CPU 负载检测间隔
	intervalUnattended = 5 * time.Minute // 无人值守检查间隔
)

type App struct {
	cfg             *Config
	template        *PushTemplate
	pendingTitle    string
	pendingContent  string
	tmpDownList     []DeviceInfo
	tmpDownMu       sync.Mutex
	logPath         string
	sendLockPath    string
	useNeighMonitor bool // 已启用 Netlink 邻居表事件，不再定时全量扫描

	wakeCh      chan struct{} // 有待发送通知时唤醒主循环，避免空转轮询
	cachedEnable int          // 每轮刷新，避免主循环多次 uci get

	// 各任务下次执行时间，到点才执行，减少无效轮询
	nextIPCheck      time.Time
	nextCronCheck    time.Time
	nextCPULoadCheck time.Time
	nextUnattended   time.Time
	nextDeviceScan   time.Time // 未用邻居表时，按 Sleeptime 全量扫描
}

func main() {
	sendCmd := flag.Bool("send", false, "trigger scheduled send once")
	clientCmd := flag.Bool("client", false, "output client list HTML (for LuCI)")
	testCmd := flag.Bool("test", false, "test push")
	flag.Parse()

	// 兼容 LuCI/cron 的无横杠子命令：pushbot send / client / test / soc
	if !*sendCmd && !*clientCmd && !*testCmd && len(os.Args) > 1 {
		switch os.Args[1] {
		case "send":
			*sendCmd = true
		case "client":
			*clientCmd = true
		case "test":
			*testCmd = true
		case "soc":
			doSoc()
			return
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.PushbotEnable != 1 && !*sendCmd && !*clientCmd {
		os.Exit(0)
	}

	ensureDir(cfg.Dir)
	app := &App{
		cfg:          cfg,
		logPath:      filepath.Join(cfg.Dir, "pushbot.log"),
		sendLockPath: filepath.Join(cfg.Dir, "send_enable.lock"),
		wakeCh:       make(chan struct{}, 1),
	}

	if *sendCmd {
		doSend(app)
		return
	}
	if *clientCmd {
		outputClientList(app)
		return
	}
	if *testCmd {
		doSend(app)
		return
	}

	// 加载推送模板
	loadGlobalTemplate(cfg.JSONPath)
	app.template = globalTemplate
	if app.template == nil {
		appLog(app, "未加载到推送模板: "+cfg.JSONPath)
	}

	// 初始化：首次用 ARP+ping 扫一次，填满 ipAddress
	_ = os.WriteFile(app.sendLockPath, nil, 0644)
	pushbotFirst(app)
	deltemp(app)

	// 若内核支持 Netlink NEIGH，改为事件驱动，不再每轮全量 ping
	if startNeighMonitor(app) {
		app.useNeighMonitor = true
		appLog(app, "已启用 Netlink 邻居表事件检测")
	}
	// 各任务首次立即执行
	now := time.Now()
	app.nextIPCheck = now
	app.nextCronCheck = now
	app.nextCPULoadCheck = now
	app.nextUnattended = now
	app.nextDeviceScan = now
	app.cachedEnable = cfg.PushbotEnable
	appLog(app, "初始化完成")

	maxSleep := time.Duration(cfg.Sleeptime) * time.Second
	sig := sigCh()
	for {
		until := nextWake(app, maxSleep)
		select {
		case <-time.After(until):
			if app.cachedEnable != 1 {
				os.Exit(0)
			}
			runCycle(app)
		case <-app.wakeCh:
			if app.cachedEnable != 1 {
				os.Exit(0)
			}
			runCycle(app)
		case s := <-sig:
			if s == syscall.SIGTERM || s == syscall.SIGINT {
				os.Exit(0)
			}
		}
	}
}

// nextWake 返回距离下次应唤醒的时长，上限 maxSleep
func nextWake(app *App, maxSleep time.Duration) time.Duration {
	now := time.Now()
	next := now.Add(maxSleep)
	if app.nextIPCheck.Before(next) {
		next = app.nextIPCheck
	}
	if app.nextCronCheck.Before(next) {
		next = app.nextCronCheck
	}
	if app.nextCPULoadCheck.Before(next) {
		next = app.nextCPULoadCheck
	}
	if app.nextUnattended.Before(next) {
		next = app.nextUnattended
	}
	if !app.useNeighMonitor && app.nextDeviceScan.Before(next) {
		next = app.nextDeviceScan
	}
	d := time.Until(next)
	if d <= 0 {
		return 0
	}
	if d > maxSleep {
		d = maxSleep
	}
	return d
}

func sigCh() <-chan os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	return ch
}

// runCycle 由主循环在「到点」或「被 wakeCh 唤醒」时调用；仅到点的任务才执行，避免无效轮询。
func runCycle(app *App) {
	cfg := app.cfg
	app.cachedEnable = uciGetInt("pushbot_enable", 0)
	now := time.Now()
	deltemp(app)

	if inDisturb(cfg) {
		return
	}

	// 公网 IP：仅到点执行（默认 5 分钟一次）
	if !now.Before(app.nextIPCheck) {
		checkIPChanges(app)
		app.nextIPCheck = now.Add(intervalIP)
	}

	// 设备列表：未启用邻居表时，按 Sleeptime 到点全量扫描
	if !app.useNeighMonitor && !now.Before(app.nextDeviceScan) {
		title, content := app.consumeNotify()
		if title != "" {
			_ = os.WriteFile(filepath.Join(cfg.Dir, "title"), []byte(title), 0644)
			appendToFile(filepath.Join(cfg.Dir, "content"), content)
		}
		pushbotFirst(app)
		title2, _ := os.ReadFile(filepath.Join(cfg.Dir, "title"))
		content2, _ := os.ReadFile(filepath.Join(cfg.Dir, "content"))
		if len(title2) > 0 {
			app.appendNotify(&Notify{Title: string(title2), Content: string(content2)})
		}
		app.nextDeviceScan = now.Add(time.Duration(cfg.Sleeptime) * time.Second)
	}

	downSend(app)
	currentDevice(app)

	// 无人值守：仅到点执行（默认 5 分钟一次）
	if !now.Before(app.nextUnattended) {
		unattended(app)
		app.nextUnattended = now.Add(intervalUnattended)
	}

	// CPU 负载：仅到点执行（默认 5 分钟一次）
	if !now.Before(app.nextCPULoadCheck) {
		cpuLoad(app)
		app.nextCPULoadCheck = now.Add(intervalCPULoad)
	}

	// 发送：有内容就发
	title, content := app.consumeNotify()
	if title != "" && content != "" && app.template != nil {
		if cfg.DeviceName != "" {
			title = "【" + cfg.DeviceName + "】" + title
		}
		for i := 0; i < 3; i++ {
			if diySend(cfg, app.template, title, content) == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
	}

	// 定时推送：仅到点检查，并计算下次 cron 时刻
	if !now.Before(app.nextCronCheck) {
		checkCronSend(app)
		app.nextCronCheck = nextCronTime(cfg)
	}
}

// nextCronTime 返回下次应检查定时推送的时间（整点或间隔整点）
func nextCronTime(cfg *Config) time.Time {
	now := time.Now()
	var next time.Time
	regular := cfg.RegularTime
	if cfg.RegularTime2 != "" {
		regular += "," + cfg.RegularTime2
	}
	if cfg.RegularTime3 != "" {
		regular += "," + cfg.RegularTime3
	}
	if regular != "" {
		for _, s := range strings.Split(regular, ",") {
			var h int
			fmt.Sscanf(strings.TrimSpace(s), "%d", &h)
			t := time.Date(now.Year(), now.Month(), now.Day(), h, 0, 0, 0, now.Location())
			if t.After(now) && (next.IsZero() || t.Before(next)) {
				next = t
			}
			t = t.Add(24 * time.Hour)
			if t.After(now) && (next.IsZero() || t.Before(next)) {
				next = t
			}
		}
	}
	if cfg.IntervalTime != "" {
		var interval int
		fmt.Sscanf(cfg.IntervalTime, "%d", &interval)
		if interval > 0 {
			h := now.Hour()
			nextHour := (h/interval+1)*interval
			if nextHour >= 24 {
				nextHour = 0
			}
			t := time.Date(now.Year(), now.Month(), now.Day(), nextHour, 0, 0, 0, now.Location())
			if nextHour == 0 {
				t = t.Add(24 * time.Hour)
			}
			if t.After(now) && (next.IsZero() || t.Before(next)) {
				next = t
			}
		}
	}
	if next.IsZero() {
		return now.Add(intervalIP) // 未配置定时则按间隔兜底
	}
	return next
}

func deltemp(app *App) {
	app.pendingTitle = ""
	app.pendingContent = ""
	os.Remove(filepath.Join(app.cfg.Dir, "title"))
	os.Remove(filepath.Join(app.cfg.Dir, "content"))
	os.Remove(filepath.Join(app.cfg.Dir, "tmp_downlist"))
	os.Remove(app.cfg.TempJSONPath)
}

func appLog(app *App, msg string) {
	t := time.Now().Format("2006-01-02 15:04:05")
	line := t + " " + msg + "\n"
	appendToFile(app.logPath, line)
	if app.cfg.Debuglevel != 0 {
		log.Print(msg)
	}
	truncateLog(app.logPath, 500)
}

func currentDevice(app *App) {
	cfg := app.cfg
	if strings.Contains(cfg.LiteEnable, "device") || strings.Contains(cfg.LiteEnable, "content") {
		return
	}
	ipPath := filepath.Join(cfg.Dir, "ipAddress")
	list := readIPAddress(ipPath)
	if len(list) == 0 {
		return
	}
	t := globalTemplate
	if t == nil {
		return
	}
	content := t.StrSplitline + t.StrTitleStart + t.FontBlue + fmt.Sprintf(" 现有在线设备 %d 台，具体如下", len(list)) + t.FontEnd + t.StrTitleEnd
	content += t.StrLinefeed + t.StrTab + "IP 地址" + t.StrSpace + t.StrSpace + t.Boldstar + "客户端名" + t.Boldstar
	for _, d := range list {
		content += t.StrLinefeed + t.StrTab + d.IP + t.Boldstar + t.FontGreen2 + cutStr(d.Name, 14) + t.FontEnd2 + t.Boldstar
	}
	app.appendNotify(&Notify{Title: "", Content: content})
}

// inDisturb 是否在免打扰时段
func inDisturb(cfg *Config) bool {
	if cfg.PushbotSheep == 0 || cfg.Starttime == 0 && cfg.Endtime == 0 {
		return false
	}
	now := time.Now().Hour()
	start, end := cfg.Starttime, cfg.Endtime
	if start < end {
		return now >= end || now < start
	}
	return now >= end && now < start
}

func checkCronSend(app *App) {
	cfg := app.cfg
	now := time.Now()
	h := now.Hour()
	// regular_time 格式如 "8" 或 "8,12,18"
	regular := cfg.RegularTime
	if cfg.RegularTime2 != "" {
		regular += "," + cfg.RegularTime2
	}
	if cfg.RegularTime3 != "" {
		regular += "," + cfg.RegularTime3
	}
	if regular != "" {
		for _, s := range strings.Split(regular, ",") {
			var hi int
			fmt.Sscanf(strings.TrimSpace(s), "%d", &hi)
			if hi == h {
				go doSend(app)
				return
			}
		}
	}
	if cfg.IntervalTime != "" {
		var interval int
		fmt.Sscanf(cfg.IntervalTime, "%d", &interval)
		if interval > 0 && h%interval == 0 && now.Minute() < 2 {
			go doSend(app)
		}
	}
}

func doSend(app *App) {
	cfg := app.cfg
	ipPath := filepath.Join(cfg.Dir, "ipAddress")
	list := readIPAddress(ipPath)
	t := globalTemplate
	if t == nil {
		return
	}
	sendTitle := "定时推送"
	sendContent := t.StrSplitline + t.StrTitleStart + t.FontBlue + " 设备状态" + t.FontEnd + t.StrTitleEnd
	if len(list) == 0 {
		sendContent += t.StrLinefeed + t.StrTab + "当前无在线设备"
	} else {
		sendContent += t.StrLinefeed + t.StrTab + fmt.Sprintf("现有在线设备 %d 台", len(list))
		for _, d := range list {
			onlineTime := timeForHumans(int(time.Now().Unix() - d.Timestamp))
			sendContent += t.StrLinefeed + t.StrTab + t.FontGreen2 + "【" + cutStr(d.Name, 18) + "】" + t.FontEnd2 + " " + d.IP
			sendContent += t.StrLinefeed + t.StrTab + "在线 " + onlineTime
		}
	}
	if cfg.DeviceName != "" {
		sendTitle = "【" + cfg.DeviceName + "】" + sendTitle
	}
	_ = diySend(cfg, t, sendTitle, sendContent)
}

func outputClientList(app *App) {
	ipPath := filepath.Join(app.cfg.Dir, "ipAddress")
	list := readIPAddress(ipPath)
	fmt.Println("<h2>在线设备列表</h2>")
	fmt.Println("<table style='width:100%; border-collapse: collapse;'>")
	fmt.Println("<tr><th>客户端名</th><th>MAC</th><th>IP</th><th>在线时间</th></tr>")
	for _, d := range list {
		onlineTime := timeForHumans(int(time.Now().Unix() - d.Timestamp))
		fmt.Printf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", d.Name, d.MAC, d.IP, onlineTime)
	}
	fmt.Println("</table>")
}

func cpuLoad(app *App) {
	cfg := app.cfg
	if cfg.CpuloadEnable != 1 {
		return
	}
	// 读 /proc/loadavg
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return
	}
	var load1, load5, load15 float64
	fmt.Sscanf(string(b), "%f %f %f", &load1, &load5, &load15)
	if load1 > float64(cfg.Cpuload) {
		// 可加 5 分钟冷却
		t := globalTemplate
		if t != nil {
			app.appendNotify(&Notify{
				Title:   "CPU 负载过高",
				Content: t.StrSplitline + t.StrTitleStart + t.FontRed + " 当前负载: " + fmt.Sprintf("%.2f", load1) + t.FontEnd + t.StrTitleEnd,
			})
		}
	}
}

func unattended(app *App) {
	cfg := app.cfg
	if cfg.ErrEnable != 1 {
		return
	}
	ipPath := filepath.Join(cfg.Dir, "ipAddress")
	current := readIPAddress(ipPath)
	currentMAC := make(map[string]bool)
	for _, d := range current {
		currentMAC[strings.ToLower(d.MAC)] = true
	}
	for _, mac := range cfg.ErrDeviceAliasesLines {
		mac = strings.TrimSpace(strings.Split(mac, " ")[0])
		if mac == "" {
			continue
		}
		if currentMAC[strings.ToLower(mac)] {
			return // 有关注设备在线，不执行
		}
	}
	// 免打扰 1 小时后允许执行
	if cfg.ErrSheepEnable == 1 && inDisturb(cfg) {
		// 简化：不维护 sheep_starttime，直接不执行
		return
	}
	// 执行无人值守动作
	switch cfg.NetworkErrEvent {
	case 1:
		// reboot
		appLog(app, "无人值守：重启路由器")
		exec.Command("reboot").Run()
	case 2:
		appLog(app, "无人值守：重新拨号")
		exec.Command("/etc/init.d/network", "restart").Run()
	}
}
