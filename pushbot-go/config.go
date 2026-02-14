package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const configSection = "pushbot.@pushbot[0]"

// Config 从 UCI 读取的配置（与 LuCI 一致）
type Config struct {
	Dir       string // /tmp/pushbot
	ConfigDir string // /usr/bin/pushbot

	PushbotEnable int
	LiteEnable    string
	DeviceName    string
	Sleeptime     int
	Debuglevel    int
	JSONPath      string
	TempJSONPath  string

	PushbotIPv4     int
	IPv4Interface  string
	PushbotIPv6     int
	IPv6Interface  string
	PushbotUp      int
	PushbotDown    int
	CpuloadEnable  int
	Cpuload        int
	TempEnable     int
	Temperature    int
	RegularTime    string
	RegularTime2   string
	RegularTime3   string
	IntervalTime   string
	UpTimeout      int
	DownTimeout    int
	TimeoutRetry   int
	PushbotSheep   int
	Starttime      int
	Endtime        int
	PushbotWhitelist string
	PushbotBlacklist string
	PushbotInterface string
	MACOnlineList  string
	MACOfflineList string
	WebLogged      int
	SSHLogged      int
	WebLoginFailed int
	SSHLoginFailed int
	LoginMaxNum    int
	WebLoginBlack  int
	IPBlackTimeout int
	ErrEnable      int
	ErrSheepEnable int
	ErrDeviceAliases string
	NetworkErrEvent  int
	SystemTimeEvent  int
	AutorebootTime   int
	NetworkRestartTime int
	PublicIPEvent    int
	PublicIPRetryCount int

	// 推送相关 token（按 jsonpath 选用）
	DDWebhook     string
	QywxCorpid    string
	QywxAgentid   string
	QywxCorpsecret string
	QywxTouser    string
	WeWebhook     string
	PPToken       string
	PPChannel     string
	PPWebhook     string
	PPTopicEnable int
	PPTopic       string
	FsWebhook     string
	PushdeerKey   string
	PushdeerSrvEnable int
	PushdeerSrv   string
	BarkSrvEnable int
	BarkSrv       string
	BarkToken     string
	BarkSound     string
	BarkIcon      string
	BarkIconEnable int
	BarkLevel     string
	// DIY
	TgToken string
	ChatID  string

	DeviceAliases  []string // MAC-名称
	IPv4URLList    []string
	IPv6URLList    []string
	PushbotBlacklistLines []string
	PushbotWhitelistLines []string
	ErrDeviceAliasesLines []string
	MarkMacList    []string
}

// uciShowSection 一次 exec 拉取整段配置，解析为 key->value；失败时返回 nil。
func uciShowSection() map[string]string {
	cmd := exec.Command("uci", "-q", "show", configSection)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	m := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.LastIndex(line, ".")
		if i < 0 {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 || eq <= i {
			continue
		}
		key := strings.TrimSpace(line[i+1 : eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, "'\"")
		m[key] = val
	}
	return m
}

func uciGet(key string) string {
	cmd := exec.Command("uci", "-q", "get", configSection+"."+key)
	cmd.Env = os.Environ()
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func uciMapInt(m map[string]string, key string, def int) int {
	s := m[key]
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func uciGetInt(key string, defaultVal int) int {
	s := uciGet(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func loadListFromUCI(key string) []string {
	s := uciGet(key)
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, " ", "\n"), "\n")
}

func loadConfig() (*Config, error) {
	c := &Config{}
	c.Dir = "/tmp/pushbot"
	c.ConfigDir = "/usr/bin/pushbot"
	c.TempJSONPath = filepath.Join(c.Dir, "temp.json")

	m := uciShowSection()
	if m == nil {
		return loadConfigFallback(c)
	}

	c.PushbotEnable = uciMapInt(m, "pushbot_enable", 0)
	c.LiteEnable = m["lite_enable"]
	c.DeviceName = m["device_name"]
	if c.DeviceName == "" {
		c.DeviceName = "OpenWrt"
	}
	c.Sleeptime = uciMapInt(m, "sleeptime", 30)
	c.Debuglevel = uciMapInt(m, "debuglevel", 1)
	c.JSONPath = m["jsonpath"]
	if c.JSONPath == "" {
		c.JSONPath = "/usr/bin/pushbot/api/dingding.json"
	}
	c.PushbotIPv4 = uciMapInt(m, "pushbot_ipv4", 0)
	c.IPv4Interface = m["ipv4_interface"]
	c.PushbotIPv6 = uciMapInt(m, "pushbot_ipv6", 0)
	c.IPv6Interface = m["ipv6_interface"]
	c.PushbotUp = uciMapInt(m, "pushbot_up", 1)
	c.PushbotDown = uciMapInt(m, "pushbot_down", 1)
	c.CpuloadEnable = uciMapInt(m, "cpuload_enable", 0)
	c.Cpuload = uciMapInt(m, "cpuload", 80)
	c.TempEnable = uciMapInt(m, "temperature_enable", 0)
	c.Temperature = uciMapInt(m, "temperature", 70)
	c.RegularTime = m["regular_time"]
	c.RegularTime2 = m["regular_time_2"]
	c.RegularTime3 = m["regular_time_3"]
	c.IntervalTime = m["interval_time"]
	c.UpTimeout = uciMapInt(m, "up_timeout", 2)
	c.DownTimeout = uciMapInt(m, "down_timeout", 20)
	if c.DownTimeout > 0 {
		c.DownTimeout = c.DownTimeout/2 + 1
	}
	c.TimeoutRetry = uciMapInt(m, "timeout_retry_count", 2)
	if c.TimeoutRetry == 0 {
		c.TimeoutRetry = 1
	}
	c.PushbotSheep = uciMapInt(m, "pushbot_sheep", 0)
	c.Starttime = uciMapInt(m, "starttime", 0)
	c.Endtime = uciMapInt(m, "endtime", 0)
	c.PushbotWhitelist = m["pushbot_whitelist"]
	c.PushbotBlacklist = m["pushbot_blacklist"]
	c.PushbotInterface = m["pushbot_interface"]
	c.MACOnlineList = m["MAC_online_list"]
	c.MACOfflineList = m["MAC_offline_list"]
	c.WebLogged = uciMapInt(m, "web_logged", 0)
	c.SSHLogged = uciMapInt(m, "ssh_logged", 0)
	c.WebLoginFailed = uciMapInt(m, "web_login_failed", 0)
	c.SSHLoginFailed = uciMapInt(m, "ssh_login_failed", 0)
	c.LoginMaxNum = uciMapInt(m, "login_max_num", 5)
	c.WebLoginBlack = uciMapInt(m, "web_login_black", 1)
	c.IPBlackTimeout = uciMapInt(m, "ip_black_timeout", 86400)
	c.ErrEnable = uciMapInt(m, "err_enable", 0)
	c.ErrSheepEnable = uciMapInt(m, "err_sheep_enable", 0)
	c.ErrDeviceAliases = m["err_device_aliases"]
	c.NetworkErrEvent = uciMapInt(m, "network_err_event", 0)
	c.SystemTimeEvent = uciMapInt(m, "system_time_event", 0)
	c.AutorebootTime = uciMapInt(m, "autoreboot_time", 0)
	c.NetworkRestartTime = uciMapInt(m, "network_restart_time", 0)
	c.PublicIPEvent = uciMapInt(m, "public_ip_event", 0)
	c.PublicIPRetryCount = uciMapInt(m, "public_ip_retry_count", 0)
	c.DDWebhook = m["dd_webhook"]
	c.QywxCorpid = m["qywx_corpid"]
	c.QywxAgentid = m["qywx_agentid"]
	c.QywxCorpsecret = m["qywx_corpsecret"]
	c.QywxTouser = m["qywx_touser"]
	c.WeWebhook = m["we_webhook"]
	c.PPToken = m["pp_token"]
	c.PPChannel = m["pp_channel"]
	c.PPWebhook = m["pp_webhook"]
	c.PPTopicEnable = uciMapInt(m, "pp_topic_enable", 0)
	c.PPTopic = m["pp_topic"]
	c.FsWebhook = m["fs_webhook"]
	c.PushdeerKey = m["pushdeer_key"]
	c.PushdeerSrvEnable = uciMapInt(m, "pushdeer_srv_enable", 0)
	c.PushdeerSrv = m["pushdeer_srv"]
	c.BarkSrvEnable = uciMapInt(m, "bark_srv_enable", 0)
	c.BarkSrv = m["bark_srv"]
	c.BarkToken = m["bark_token"]
	c.BarkSound = m["bark_sound"]
	c.BarkIcon = m["bark_icon"]
	c.BarkIconEnable = uciMapInt(m, "bark_icon_enable", 0)
	c.BarkLevel = m["bark_level"]
	c.TgToken = m["tg_token"]
	c.ChatID = m["chat_id"]

	c.DeviceAliases = listFromStr(m["device_aliases"])
	c.PushbotBlacklistLines = listFromStr(m["pushbot_blacklist"])
	c.PushbotWhitelistLines = listFromStr(m["pushbot_whitelist"])
	c.ErrDeviceAliasesLines = listFromStr(m["err_device_aliases"])
	c.MarkMacList = listFromStr(m["MAC_online_list"])
	c.MarkMacList = append(c.MarkMacList, listFromStr(m["MAC_offline_list"])...)

	c.IPv4URLList = readLines(filepath.Join(c.ConfigDir, "api", "ipv4.list"))
	c.IPv6URLList = readLines(filepath.Join(c.ConfigDir, "api", "ipv6.list"))

	if c.BarkToken != "" && c.BarkSrv == "" {
		c.BarkSrv = "https://api.day.app"
	}
	if c.PushdeerKey != "" && c.PushdeerSrv == "" {
		c.PushdeerSrv = "https://api2.pushdeer.com"
	}
	return c, nil
}

func listFromStr(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, " ", "\n"), "\n")
}

// loadConfigFallback 当 uci show 失败时逐项 uci get（兼容异常环境）
func loadConfigFallback(c *Config) (*Config, error) {
	c.PushbotEnable = uciGetInt("pushbot_enable", 0)
	c.LiteEnable = uciGet("lite_enable")
	c.DeviceName = uciGet("device_name")
	if c.DeviceName == "" {
		c.DeviceName = "OpenWrt"
	}
	c.Sleeptime = uciGetInt("sleeptime", 30)
	c.Debuglevel = uciGetInt("debuglevel", 1)
	c.JSONPath = uciGet("jsonpath")
	if c.JSONPath == "" {
		c.JSONPath = "/usr/bin/pushbot/api/dingding.json"
	}
	c.PushbotIPv4 = uciGetInt("pushbot_ipv4", 0)
	c.IPv4Interface = uciGet("ipv4_interface")
	c.PushbotIPv6 = uciGetInt("pushbot_ipv6", 0)
	c.IPv6Interface = uciGet("ipv6_interface")
	c.PushbotUp = uciGetInt("pushbot_up", 1)
	c.PushbotDown = uciGetInt("pushbot_down", 1)
	c.CpuloadEnable = uciGetInt("cpuload_enable", 0)
	c.Cpuload = uciGetInt("cpuload", 80)
	c.TempEnable = uciGetInt("temperature_enable", 0)
	c.Temperature = uciGetInt("temperature", 70)
	c.RegularTime = uciGet("regular_time")
	c.RegularTime2 = uciGet("regular_time_2")
	c.RegularTime3 = uciGet("regular_time_3")
	c.IntervalTime = uciGet("interval_time")
	c.UpTimeout = uciGetInt("up_timeout", 2)
	c.DownTimeout = uciGetInt("down_timeout", 20)
	if c.DownTimeout > 0 {
		c.DownTimeout = c.DownTimeout/2 + 1
	}
	c.TimeoutRetry = uciGetInt("timeout_retry_count", 2)
	if c.TimeoutRetry == 0 {
		c.TimeoutRetry = 1
	}
	c.PushbotSheep = uciGetInt("pushbot_sheep", 0)
	c.Starttime = uciGetInt("starttime", 0)
	c.Endtime = uciGetInt("endtime", 0)
	c.PushbotWhitelist = uciGet("pushbot_whitelist")
	c.PushbotBlacklist = uciGet("pushbot_blacklist")
	c.PushbotInterface = uciGet("pushbot_interface")
	c.MACOnlineList = uciGet("MAC_online_list")
	c.MACOfflineList = uciGet("MAC_offline_list")
	c.WebLogged = uciGetInt("web_logged", 0)
	c.SSHLogged = uciGetInt("ssh_logged", 0)
	c.WebLoginFailed = uciGetInt("web_login_failed", 0)
	c.SSHLoginFailed = uciGetInt("ssh_login_failed", 0)
	c.LoginMaxNum = uciGetInt("login_max_num", 5)
	c.WebLoginBlack = uciGetInt("web_login_black", 1)
	c.IPBlackTimeout = uciGetInt("ip_black_timeout", 86400)
	c.ErrEnable = uciGetInt("err_enable", 0)
	c.ErrSheepEnable = uciGetInt("err_sheep_enable", 0)
	c.ErrDeviceAliases = uciGet("err_device_aliases")
	c.NetworkErrEvent = uciGetInt("network_err_event", 0)
	c.SystemTimeEvent = uciGetInt("system_time_event", 0)
	c.AutorebootTime = uciGetInt("autoreboot_time", 0)
	c.NetworkRestartTime = uciGetInt("network_restart_time", 0)
	c.PublicIPEvent = uciGetInt("public_ip_event", 0)
	c.PublicIPRetryCount = uciGetInt("public_ip_retry_count", 0)
	c.DDWebhook = uciGet("dd_webhook")
	c.QywxCorpid = uciGet("qywx_corpid")
	c.QywxAgentid = uciGet("qywx_agentid")
	c.QywxCorpsecret = uciGet("qywx_corpsecret")
	c.QywxTouser = uciGet("qywx_touser")
	c.WeWebhook = uciGet("we_webhook")
	c.PPToken = uciGet("pp_token")
	c.PPChannel = uciGet("pp_channel")
	c.PPWebhook = uciGet("pp_webhook")
	c.PPTopicEnable = uciGetInt("pp_topic_enable", 0)
	c.PPTopic = uciGet("pp_topic")
	c.FsWebhook = uciGet("fs_webhook")
	c.PushdeerKey = uciGet("pushdeer_key")
	c.PushdeerSrvEnable = uciGetInt("pushdeer_srv_enable", 0)
	c.PushdeerSrv = uciGet("pushdeer_srv")
	c.BarkSrvEnable = uciGetInt("bark_srv_enable", 0)
	c.BarkSrv = uciGet("bark_srv")
	c.BarkToken = uciGet("bark_token")
	c.BarkSound = uciGet("bark_sound")
	c.BarkIcon = uciGet("bark_icon")
	c.BarkIconEnable = uciGetInt("bark_icon_enable", 0)
	c.BarkLevel = uciGet("bark_level")
	c.TgToken = uciGet("tg_token")
	c.ChatID = uciGet("chat_id")
	c.DeviceAliases = loadListFromUCI("device_aliases")
	c.PushbotBlacklistLines = loadListFromUCI("pushbot_blacklist")
	c.PushbotWhitelistLines = loadListFromUCI("pushbot_whitelist")
	c.ErrDeviceAliasesLines = loadListFromUCI("err_device_aliases")
	c.MarkMacList = loadListFromUCI("MAC_online_list")
	c.MarkMacList = append(c.MarkMacList, loadListFromUCI("MAC_offline_list")...)
	c.IPv4URLList = readLines(filepath.Join(c.ConfigDir, "api", "ipv4.list"))
	c.IPv6URLList = readLines(filepath.Join(c.ConfigDir, "api", "ipv6.list"))
	if c.BarkToken != "" && c.BarkSrv == "" {
		c.BarkSrv = "https://api.day.app"
	}
	if c.PushdeerKey != "" && c.PushdeerSrv == "" {
		c.PushdeerSrv = "https://api2.pushdeer.com"
	}
	return c, nil
}

func readLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(sc.Text(), "\r"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
