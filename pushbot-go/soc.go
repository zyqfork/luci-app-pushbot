package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const socTmpPath = "/tmp/pushbot/soc_tmp"

// doSoc 读取 SoC 温度并写入 /tmp/pushbot/soc_tmp，供 LuCI 高级设置「测试温度命令」显示
func doSoc() {
	ensureDir("/tmp/pushbot")
	out := readSocTemp()
	_ = os.WriteFile(socTmpPath, []byte(out), 0644)
}

// readSocTemp 优先读 sysfs thermal_zone（无 exec、OpenWrt 常见），再回退到 sensors
func readSocTemp() string {
	if s := readThermalZone(); s != "" {
		return s
	}
	if b, err := exec.Command("sensors").Output(); err == nil {
		if s := parseSensorsOutput(string(b)); s != "" {
			return s
		}
	}
	return ""
}

// readThermalZone 从 /sys/class/thermal/thermal_zone* 读温度，优先 CPU/SOC 类型，取最大值（毫摄氏度）
func readThermalZone() string {
	matches, err := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if err != nil {
		return ""
	}
	var cpuMax, anyMax int
	for _, dir := range matches {
		tempPath := filepath.Join(dir, "temp")
		data, err := os.ReadFile(tempPath)
		if err != nil {
			continue
		}
		v, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if v <= 0 {
			continue
		}
		if v > anyMax {
			anyMax = v
		}
		// 优先采纳类型为 cpu / soc / x86 的 zone
		if typeData, err := os.ReadFile(filepath.Join(dir, "type")); err == nil {
			t := strings.ToLower(strings.TrimSpace(string(typeData)))
			if strings.Contains(t, "cpu") || strings.Contains(t, "soc") || strings.Contains(t, "x86") {
				if v > cpuMax {
					cpuMax = v
				}
			}
		}
	}
	if cpuMax > 0 {
		return formatMilliCelsius(cpuMax)
	}
	if anyMax > 0 {
		return formatMilliCelsius(anyMax)
	}
	return ""
}

func formatMilliCelsius(millideg int) string {
	if millideg%1000 == 0 {
		return strconv.Itoa(millideg / 1000)
	}
	return strconv.FormatFloat(float64(millideg)/1000, 'f', 1, 64)
}

// 匹配 +45.0°C 或 +45.0 C 等常见 sensors 输出
var sensorsTempRe = regexp.MustCompile(`\+([\d.]+)\s*°?C`)

func parseSensorsOutput(s string) string {
	sc := bufio.NewScanner(strings.NewReader(s))
	var max float64
	for sc.Scan() {
		line := sc.Text()
		ms := sensorsTempRe.FindStringSubmatch(line)
		if len(ms) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(ms[1], 64)
		if err != nil {
			continue
		}
		if v > max {
			max = v
		}
	}
	if max > 0 {
		return strconv.FormatFloat(max, 'f', 1, 64)
	}
	return ""
}
