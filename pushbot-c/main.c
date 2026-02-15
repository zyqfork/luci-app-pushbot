/*
 * Pushbot C - 设备上下线推送，与 LuCI 配置兼容；无 Go 依赖，OpenWrt 下 C 工具链即可编译。
 */
#include "common.h"
#include "config.h"
#include "device.h"
#include "ip.h"
#include "push.h"
#include "soc.h"
#include "util.h"
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static device_info_t g_current[MAX_DEVICES];
static int g_n_current;
static char g_log_path[MAX_STR];
static char g_ip_path[MAX_STR];
static char g_pending_title[MAX_LINE];
static char g_pending_content[MAX_LINE * 2];
static int g_pending_has;

static void on_up_cb(void *ctx, const device_info_t *d, const char *extra) {
	(void)extra;
	config_t *cfg = (config_t *)ctx;
	snprintf(g_pending_title, sizeof(g_pending_title), "%s 连接了你的路由器", d->name);
	snprintf(g_pending_content, sizeof(g_pending_content),
		"新设备连接\n客户端名： %s\n客户端IP： %s\n客户端MAC：%s\n网络接口：%s",
		d->name, d->ip, d->mac, d->iface);
	g_pending_has = 1;
}

static void on_down_cb(void *ctx, const device_info_t *d, const char *extra) {
	(void)ctx;
	(void)extra;
	char dur[64];
	time_for_humans((int)(time(NULL) - d->timestamp), dur, sizeof(dur));
	snprintf(g_pending_title, sizeof(g_pending_title), "%s 断开连接", d->name);
	snprintf(g_pending_content, sizeof(g_pending_content),
		"设备断开连接\n客户端名： %s\n客户端IP： %s\n客户端MAC：%s\n在线时间： %s",
		d->name, d->ip, d->mac, dur);
	g_pending_has = 1;
}

static void run_cycle(config_t *cfg) {
	device_info_t new_list[MAX_DEVICES];
	int n_new = 0;
	pushbot_first(cfg, g_ip_path, g_log_path,
		g_current, g_n_current,
		new_list, &n_new,
		on_up_cb, on_down_cb, cfg);
	memcpy(g_current, new_list, (size_t)n_new * sizeof(device_info_t));
	g_n_current = n_new;
	device_write_list(g_ip_path, g_current, g_n_current);

	if (g_pending_has && g_pending_title[0] && g_pending_content[0]) {
		char title[MAX_LINE + 64];
		if (cfg->device_name[0])
			snprintf(title, sizeof(title), "【%s】%s", cfg->device_name, g_pending_title);
		else
			strncpy(title, g_pending_title, sizeof(title) - 1);
		for (int i = 0; i < 3; i++) {
			if (push_send(cfg, title, g_pending_content) == 0) break;
			sleep(2);
		}
		g_pending_has = 0;
	}
}

static void do_send_once(config_t *cfg) {
	g_pending_has = 1;
	snprintf(g_pending_title, sizeof(g_pending_title), "定时推送");
	snprintf(g_pending_content, sizeof(g_pending_content), "设备运行状态");
	run_cycle(cfg);
}

static void output_client_list(config_t *cfg) {
	device_info_t list[MAX_DEVICES];
	int n = 0;
	device_read_list(g_ip_path, list, MAX_DEVICES, &n);
	printf("<table border='1'><tr><th>设备名</th><th>MAC</th><th>IP</th><th>在线时长</th></tr>");
	for (int i = 0; i < n; i++) {
		char dur[64];
		time_for_humans((int)(time(NULL) - list[i].timestamp), dur, sizeof(dur));
		printf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			list[i].name, list[i].mac, list[i].ip, dur);
	}
	printf("</table>");
}

int main(int argc, char **argv) {
	int do_send = 0, do_client = 0, do_test = 0, do_soc = 0;
	if (argc > 1) {
		if (strcmp(argv[1], "send") == 0) do_send = 1;
		else if (strcmp(argv[1], "client") == 0) do_client = 1;
		else if (strcmp(argv[1], "test") == 0) do_test = 1;
		else if (strcmp(argv[1], "soc") == 0) { soc_run(); return 0; }
	}
	if (argc > 1 && strcmp(argv[1], "-send") == 0) do_send = 1;
	if (argc > 1 && strcmp(argv[1], "-client") == 0) do_client = 1;
	if (argc > 1 && strcmp(argv[1], "-test") == 0) do_test = 1;

	config_t cfg;
	if (load_config(&cfg) != 0) return 1;
	if (cfg.pushbot_enable != 1 && !do_send && !do_client) return 0;

	ensure_dir(cfg.dir);
	snprintf(g_log_path, sizeof(g_log_path), "%s/pushbot.log", cfg.dir);
	snprintf(g_ip_path, sizeof(g_ip_path), "%s/ipAddress", cfg.dir);

	char send_lock[MAX_STR];
	snprintf(send_lock, sizeof(send_lock), "%s/send_enable.lock", cfg.dir);
	FILE *f = fopen(send_lock, "w");
	if (f) fclose(f);

	if (do_send || do_test) {
		do_send_once(&cfg);
		return 0;
	}
	if (do_client) {
		device_read_list(g_ip_path, g_current, MAX_DEVICES, &g_n_current);
		output_client_list(&cfg);
		return 0;
	}

	/* 守护进程化：脱离启动它的 shell，避免收到 SIGHUP 被杀死 */
	if (daemon(0, 0) != 0)
		return 1;

	/* Daemon: init device list then loop */
	g_n_current = 0;
	device_read_list(g_ip_path, g_current, MAX_DEVICES, &g_n_current);
	device_info_t new_list[MAX_DEVICES];
	int n_new = 0;
	pushbot_first(&cfg, g_ip_path, g_log_path,
		g_current, g_n_current, new_list, &n_new,
		on_up_cb, on_down_cb, &cfg);
	memcpy(g_current, new_list, (size_t)n_new * sizeof(device_info_t));
	g_n_current = n_new;
	device_write_list(g_ip_path, g_current, g_n_current);
	app_log(g_log_path, cfg.debuglevel, "初始化完成");

	signal(SIGPIPE, SIG_IGN);
	for (;;) {
		if (uci_get_enable() != 1) return 0;
		run_cycle(&cfg);
		sleep((unsigned)cfg.sleeptime);
	}
	return 0;
}
