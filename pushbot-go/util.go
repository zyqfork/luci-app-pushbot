package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func timeForHumans(sec int) string {
	if sec < 60 {
		return strconv.Itoa(sec) + " 秒"
	}
	if sec < 3600 {
		m := sec / 60
		s := sec - m*60
		return fmt.Sprintf("%d 分 %d 秒", m, s)
	}
	if sec < 86400 {
		h := sec / 3600
		m := (sec - h*3600) / 60
		return fmt.Sprintf("%d 时 %d 分", h, m)
	}
	d := sec / 86400
	h := (sec - d*86400) / 3600
	return fmt.Sprintf("%d 天 %d 时", d, h)
}

var ipV4Re = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)

func ensureDir(d string) {
	_ = os.MkdirAll(d, 0755)
}

func appendToFile(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	if !strings.HasSuffix(line, "\n") {
		_, _ = f.WriteString("\n")
	}
	_ = f.Close()
}

func truncateLog(logPath string, maxLines int) {
	f, err := os.Open(logPath)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) <= maxLines {
		return
	}
	// 保留后 maxLines 条
	keep := lines[len(lines)-maxLines:]
	_ = os.WriteFile(logPath, []byte(strings.Join(keep, "\n")+"\n"), 0644)
}

func cutStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
