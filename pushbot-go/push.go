package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Notify struct {
	Title   string
	Content string
}

// PushTemplate 从 jsonpath 加载的推送模板
type PushTemplate struct {
	URL         string            `json:"url"`
	Data        string            `json:"data"`
	ContentType string            `json:"content_type"`
	Type        map[string]interface{} `json:"type"`
	StrTitleStart string `json:"str_title_start"`
	StrTitleEnd   string `json:"str_title_end"`
	StrLinefeed   string `json:"str_linefeed"`
	StrSplitline  string `json:"str_splitline"`
	StrSpace     string `json:"str_space"`
	StrTab       string `json:"str_tab"`
	FontGreen    string `json:"font_green"`
	FontGreen2   string `json:"font_green2"`
	FontRed      string `json:"font_red"`
	FontBlue     string `json:"font_blue"`
	FontEnd      string `json:"font_end"`
	FontEnd2     string `json:"font_end2"`
	Boldstar     string `json:"boldstar"`
}

var templateRe = regexp.MustCompile(`\$\{([^}]+)\}`)

func loadPushTemplate(jsonPath string) (*PushTemplate, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	var t PushTemplate
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (t *PushTemplate) varsMap(cfg *Config, title, content string) map[string]string {
	nowtime := time.Now().Format("2006-01-02 15:04:05")
	m := map[string]string{
		"1": title, "2": content,
		"nowtime":       nowtime,
		"str_title_start": t.StrTitleStart, "str_title_end": t.StrTitleEnd,
		"str_linefeed":   t.StrLinefeed, "str_splitline": t.StrSplitline,
		"str_space": t.StrSpace, "str_tab": t.StrTab,
		"font_green": t.FontGreen, "font_green2": t.FontGreen2,
		"font_red": t.FontRed, "font_blue": t.FontBlue,
		"font_end": t.FontEnd, "font_end2": t.FontEnd2,
		"boldstar": t.Boldstar,
		"tempjsonpath": cfg.TempJSONPath,
		"dd_webhook":    cfg.DDWebhook, "we_webhook": cfg.WeWebhook,
		"pp_token": cfg.PPToken, "pp_channel": cfg.PPChannel,
		"pp_webhook": cfg.PPWebhook, "pp_topic": cfg.PPTopic,
		"pushdeer_key": cfg.PushdeerKey, "pushdeer_srv": cfg.PushdeerSrv,
		"bark_token": cfg.BarkToken, "bark_srv": cfg.BarkSrv,
		"bark_sound": cfg.BarkSound, "bark_icon": cfg.BarkIcon, "bark_level": cfg.BarkLevel,
		"fs_webhook": cfg.FsWebhook,
		"qywx_corpid": cfg.QywxCorpid, "qywx_agentid": cfg.QywxAgentid,
		"qywx_corpsecret": cfg.QywxCorpsecret, "qywx_touser": cfg.QywxTouser,
		"tg_token": cfg.TgToken, "chat_id": cfg.ChatID,
	}
	return m
}

func substituteVars(s string, m map[string]string) string {
	return templateRe.ReplaceAllStringFunc(s, func(match string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if v, ok := m[key]; ok {
			return v
		}
		return match
	})
}

// substituteMap 递归替换 map 中所有字符串里的变量
func substituteMap(m map[string]interface{}, vars map[string]string) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		out[k] = substituteValue(v, vars)
	}
	return out
}

func substituteValue(v interface{}, vars map[string]string) interface{} {
	switch x := v.(type) {
	case string:
		s := substituteVars(x, vars)
		s = strings.Trim(s, "\"")
		return s
	case map[string]interface{}:
		return substituteMap(x, vars)
	default:
		return v
	}
}

// diySend 根据模板构建 body 并 POST
func diySend(cfg *Config, t *PushTemplate, title, content string) error {
	vars := t.varsMap(cfg, title, content)
	urlStr := substituteVars(t.URL, vars)
	urlStr = strings.Trim(urlStr, "\"")
	if urlStr == "" {
		return fmt.Errorf("empty url")
	}
	var body []byte
	if strings.HasPrefix(t.Data, "@") {
		typeObj := substituteMap(t.Type, vars)
		body, _ = json.Marshal(typeObj)
	} else {
		body = []byte(substituteVars(t.Data, vars))
	}
	req, err := http.NewRequest("POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", t.ContentType)
	resp, err := defaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(bb))
	}
	return nil
}

func notifyUp(ip, mac, name, iface string, cfg *Config) *Notify {
	t := globalTemplate
	if t == nil {
		return &Notify{Title: name + " 连接了你的路由器", Content: ""}
	}
	title := name + " 连接了你的路由器"
	content := t.StrSplitline + t.StrTitleStart + t.FontGreen + " 新设备连接" + t.FontEnd + t.StrTitleEnd
	content += t.StrLinefeed + t.StrTab + "客户端名：" + t.StrSpace + name
	content += t.StrLinefeed + t.StrTab + "客户端IP： " + t.StrSpace + ip
	content += t.StrLinefeed + t.StrTab + "客户端MAC：" + t.StrSpace + mac
	content += t.StrLinefeed + t.StrTab + "网络接口：" + t.StrSpace + iface
	return &Notify{Title: title, Content: content}
}

func notifyDown(ip, mac, name, onlineTime string, cfg *Config) *Notify {
	t := globalTemplate
	if t == nil {
		return &Notify{Title: name + " 断开连接", Content: ""}
	}
	title := name + " 断开连接"
	content := t.StrSplitline + t.StrTitleStart + t.FontRed + " 设备断开连接" + t.FontEnd + t.StrTitleEnd
	content += t.StrLinefeed + t.StrTab + "客户端名：" + t.StrSpace + name
	content += t.StrLinefeed + t.StrTab + "客户端IP： " + t.StrSpace + ip
	content += t.StrLinefeed + t.StrTab + "客户端MAC：" + t.StrSpace + mac
	content += t.StrLinefeed + t.StrTab + "在线时间： " + t.StrSpace + onlineTime
	return &Notify{Title: title, Content: content}
}

var globalTemplate *PushTemplate

var defaultHTTPClient = &http.Client{Timeout: 15 * time.Second}

func loadGlobalTemplate(jsonPath string) {
	t, err := loadPushTemplate(jsonPath)
	if err != nil {
		return
	}
	// 反义 \n
	t.StrLinefeed = strings.ReplaceAll(t.StrLinefeed, "\\n", "\n")
	t.StrSplitline = strings.ReplaceAll(t.StrSplitline, "\\n", "\n")
	globalTemplate = t
}
