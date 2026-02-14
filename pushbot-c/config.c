#include "common.h"
#include "config.h"
#include "util.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static int uci_get_int(const char *key, int def) {
	char cmd[256];
	snprintf(cmd, sizeof(cmd), "uci -q get \"%s.%s\" 2>/dev/null", CONFIG_SECTION, key);
	FILE *f = popen(cmd, "r");
	if (!f) return def;
	char buf[64];
	if (!fgets(buf, sizeof(buf), f)) { pclose(f); return def; }
	pclose(f);
	return atoi(buf);
}

static void uci_get_str(const char *key, char *out, size_t outsz) {
	char cmd[256];
	snprintf(cmd, sizeof(cmd), "uci -q get \"%s.%s\" 2>/dev/null", CONFIG_SECTION, key);
	FILE *f = popen(cmd, "r");
	if (!f) { if (out && outsz) out[0] = '\0'; return; }
	if (!fgets(out, (int)outsz, f)) { pclose(f); if (out && outsz) out[0] = '\0'; return; }
	pclose(f);
	/* trim newline */
	size_t len = strlen(out);
	if (len > 0 && out[len - 1] == '\n') out[len - 1] = '\0';
}

int load_config(config_t *c) {
	memset(c, 0, sizeof(*c));
	strncpy(c->dir, DIR_TMP, sizeof(c->dir) - 1);
	strncpy(c->config_dir, CONFIG_DIR, sizeof(c->config_dir) - 1);

	c->pushbot_enable = uci_get_int("pushbot_enable", 0);
	uci_get_str("device_name", c->device_name, sizeof(c->device_name));
	if (!c->device_name[0]) strcpy(c->device_name, "OpenWrt");
	c->sleeptime = uci_get_int("sleeptime", 30);
	c->debuglevel = uci_get_int("debuglevel", 1);
	uci_get_str("jsonpath", c->jsonpath, sizeof(c->jsonpath));
	if (!c->jsonpath[0]) snprintf(c->jsonpath, sizeof(c->jsonpath), "%s/api/dingding.json", CONFIG_DIR);

	c->pushbot_up = uci_get_int("pushbot_up", 1);
	c->pushbot_down = uci_get_int("pushbot_down", 1);
	c->up_timeout = uci_get_int("up_timeout", 2);
	c->down_timeout = uci_get_int("down_timeout", 20);
	if (c->down_timeout > 0) c->down_timeout = c->down_timeout / 2 + 1;
	c->timeout_retry = uci_get_int("timeout_retry_count", 2);
	if (c->timeout_retry == 0) c->timeout_retry = 1;

	c->pushbot_sheep = uci_get_int("pushbot_sheep", 0);
	c->starttime = uci_get_int("starttime", 0);
	c->endtime = uci_get_int("endtime", 0);
	c->pushbot_ipv4 = uci_get_int("pushbot_ipv4", 0);
	c->pushbot_ipv6 = uci_get_int("pushbot_ipv6", 0);
	c->cpuload_enable = uci_get_int("cpuload_enable", 0);
	c->cpuload = uci_get_int("cpuload", 80);
	c->temp_enable = uci_get_int("temperature_enable", 0);
	c->temperature = uci_get_int("temperature", 70);
	uci_get_str("regular_time", c->regular_time, sizeof(c->regular_time));
	uci_get_str("regular_time_2", c->regular_time2, sizeof(c->regular_time2));
	uci_get_str("regular_time_3", c->regular_time3, sizeof(c->regular_time3));
	uci_get_str("interval_time", c->interval_time, sizeof(c->interval_time));
	uci_get_str("pushbot_whitelist", c->pushbot_whitelist, sizeof(c->pushbot_whitelist));
	uci_get_str("pushbot_blacklist", c->pushbot_blacklist, sizeof(c->pushbot_blacklist));
	uci_get_str("pushbot_interface", c->pushbot_interface, sizeof(c->pushbot_interface));
	c->err_enable = uci_get_int("err_enable", 0);
	uci_get_str("err_device_aliases", c->err_device_aliases, sizeof(c->err_device_aliases));
	c->network_err_event = uci_get_int("network_err_event", 0);
	c->system_time_event = uci_get_int("system_time_event", 0);
	uci_get_str("dd_webhook", c->dd_webhook, sizeof(c->dd_webhook));
	uci_get_str("bark_token", c->bark_token, sizeof(c->bark_token));
	uci_get_str("bark_srv", c->bark_srv, sizeof(c->bark_srv));
	return 0;
}

int uci_get_enable(void) {
	return uci_get_int("pushbot_enable", 0);
}
