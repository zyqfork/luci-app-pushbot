#include "common.h"
#include "config.h"
#include "device.h"
#include "util.h"
#include <ctype.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static int ping_ok(const char *ip, int timeout_sec, int retry, const char *iface) {
	char cmd[384];
	int i;
	if (iface && iface[0]) {
		snprintf(cmd, sizeof(cmd), "arping -I %s -c 3 -w %d %s 2>/dev/null", iface, timeout_sec, ip);
		if (system(cmd) == 0) return 1;
	}
	for (i = 0; i < retry; i++) {
		snprintf(cmd, sizeof(cmd), "ping -c 2 -W %d %s 2>/dev/null", timeout_sec, ip);
		if (system(cmd) == 0) return 1;
		sleep(1);
	}
	return 0;
}

static void arp_get_iface(const char *ip, char *iface_out, size_t sz) {
	FILE *f = fopen("/proc/net/arp", "r");
	if (!f) { if (iface_out && sz) iface_out[0] = '\0'; return; }
	char line[256];
	if (!fgets(line, sizeof(line), f)) { fclose(f); return; }
	while (fgets(line, sizeof(line), f)) {
		char tip[64];
		char iface[32];
		if (sscanf(line, "%63s %*s %*s %*s %*s %31s", tip, iface) >= 2 && strcmp(tip, ip) == 0) {
			strncpy(iface_out, iface, sz - 1);
			iface_out[sz - 1] = '\0';
			fclose(f);
			return;
		}
	}
	fclose(f);
	if (iface_out && sz) iface_out[0] = '\0';
}

static void get_mac_from_arp(const char *ip, char *mac_out, size_t sz) {
	FILE *f = fopen("/proc/net/arp", "r");
	if (!f) { if (mac_out && sz) mac_out[0] = '\0'; return; }
	char line[256];
	fgets(line, sizeof(line), f);
	while (fgets(line, sizeof(line), f)) {
		char tip[64], tmac[32];
		if (sscanf(line, "%63s %*s %*s %31s", tip, tmac) >= 2 && strcmp(tip, ip) == 0) {
			if (strcmp(tmac, "00:00:00:00:00:00") != 0) {
				strncpy(mac_out, tmac, sz - 1);
				mac_out[sz - 1] = '\0';
			}
			break;
		}
	}
	fclose(f);
}

static void get_name_from_dhcp(const char *ip, char *name_out, size_t sz) {
	char cmd[320];
	snprintf(cmd, sizeof(cmd), "grep -F '%s' /tmp/dhcp.leases 2>/dev/null | awk '{print $4}'", ip);
	FILE *f = popen(cmd, "r");
	if (!f) { if (name_out && sz) name_out[0] = '\0'; return; }
	if (fgets(name_out, (int)sz, f)) {
		size_t len = strlen(name_out);
		if (len > 0 && name_out[len-1] == '\n') name_out[len-1] = '\0';
	} else
		name_out[0] = '\0';
	pclose(f);
}

static void oui_lookup(const char *mac, const config_t *cfg, char *name_out, size_t sz) {
	if (strlen(mac) < 8) { if (name_out && sz) name_out[0] = '\0'; return; }
	char oui[16];
	snprintf(oui, sizeof(oui), "%.2s%.2s%.2s", mac, mac+3, mac+6);
	for (int i = 0; oui[i]; i++) oui[i] = (char)tolower((unsigned char)oui[i]);
	const char *dirs[] = { "/usr/share/pushbot", cfg->config_dir, cfg->dir, NULL };
	for (int d = 0; dirs[d]; d++) {
		char path[256];
		snprintf(path, sizeof(path), "%s/oui_base.txt", dirs[d]);
		FILE *f = fopen(path, "r");
		if (!f) { snprintf(path, sizeof(path), "%s/oui.txt", dirs[d]); f = fopen(path, "r"); }
		if (!f) continue;
		char line[512];
		while (fgets(line, sizeof(line), f)) {
			if (!strstr(line, "(base 16)")) continue;
			char *p = strstr(line, oui);
			if (!p || p > line + 16) continue;
			p = strstr(line, "(base 16)");
			if (!p) continue;
			p += 9;
			while (*p == '\t' || *p == ' ') p++;
			char *end = strchr(p, '\n');
			if (end) *end = '\0';
			/* copy and replace space with _ */
			size_t i = 0;
			for (; *p && i < sz - 1; p++) {
				name_out[i++] = (*p == ' ') ? '_' : *p;
			}
			name_out[i] = '\0';
			fclose(f);
			return;
		}
		fclose(f);
	}
	if (name_out && sz) name_out[0] = '\0';
}

void get_name(const config_t *cfg, const char *ip, const char *mac, char *name_out, size_t sz) {
	get_name_from_dhcp(ip, name_out, sz);
	if (name_out[0]) return;
	oui_lookup(mac, cfg, name_out, sz);
	if (name_out[0]) return;
	snprintf(name_out, sz, "%s", mac);
}

static int in_whitelist(const config_t *cfg, const char *mac) {
	if (!cfg->pushbot_whitelist[0]) return 0;
	char list[MAX_LINE];
	strncpy(list, cfg->pushbot_whitelist, sizeof(list)-1);
	list[sizeof(list)-1] = '\0';
	for (char *p = strtok(list, " \n\t"); p; p = strtok(NULL, " \n\t"))
		if (strcasecmp(p, mac) == 0) return 1;
	return 0;
}

static int in_blacklist(const config_t *cfg, const char *mac) {
	if (!cfg->pushbot_blacklist[0]) return 0;
	char list[MAX_LINE];
	strncpy(list, cfg->pushbot_blacklist, sizeof(list)-1);
	list[sizeof(list)-1] = '\0';
	for (char *p = strtok(list, " \n\t"); p; p = strtok(NULL, " \n\t"))
		if (strcasecmp(p, mac) == 0) return 1;
	return 0;
}

int blackwhitelist(const config_t *cfg, const char *mac) {
	if (cfg->pushbot_whitelist[0]) return in_whitelist(cfg, mac) ? 0 : 1;
	if (in_blacklist(cfg, mac)) return 1;
	return 0;
}

int device_read_list(const char *path, device_info_t *list, int max_n, int *n) {
	*n = 0;
	FILE *f = fopen(path, "r");
	if (!f) return 0;
	char line[512];
	while (fgets(line, sizeof(line), f) && *n < max_n) {
		char ip[64], mac[32], name[128], ts[32], iface[32];
		if (sscanf(line, "%63s %31s %127[^\t\n] %31s %31s", ip, mac, name, ts, iface) >= 2) {
			device_info_t *d = &list[*n];
			strncpy(d->ip, ip, sizeof(d->ip)-1);
			strncpy(d->mac, mac, sizeof(d->mac)-1);
			strncpy(d->name, name, sizeof(d->name)-1);
			strncpy(d->iface, iface, sizeof(d->iface)-1);
			d->timestamp = (time_t)atoll(ts);
			(*n)++;
		}
	}
	fclose(f);
	return 1;
}

void device_write_list(const char *path, const device_info_t *list, int n) {
	FILE *f = fopen(path, "w");
	if (!f) return;
	for (int i = 0; i < n; i++) {
		fprintf(f, "%s\t%s\t%s\t%ld\t%s\n",
			list[i].ip, list[i].mac, list[i].name,
			(long)list[i].timestamp, list[i].iface);
	}
	fclose(f);
}

typedef struct arp_entry { char ip[64]; char mac[32]; } arp_entry_t;
static int read_arp_once(arp_entry_t *out, int max_n) {
	FILE *f = fopen("/proc/net/arp", "r");
	if (!f) return 0;
	char line[256];
	int n = 0;
	fgets(line, sizeof(line), f);
	while (fgets(line, sizeof(line), f) && n < max_n) {
		char ip[64], mac[32], flags[8];
		if (sscanf(line, "%63s %*s %*s %31s %*s %*s", ip, mac, flags) < 2) continue;
		if (strncmp(ip, "169.254.", 8) == 0) continue;
		if (strcmp(mac, "00:00:00:00:00:00") == 0) continue;
		if (strcmp(flags, "0x2") != 0 && strcmp(flags, "0x6") != 0) continue;
		strncpy(out[n].ip, ip, sizeof(out[n].ip)-1);
		strncpy(out[n].mac, mac, sizeof(out[n].mac)-1);
		n++;
	}
	fclose(f);
	return n;
}

static void get_interface_for_mac(const config_t *cfg, const char *mac, char *iface_out, size_t sz) {
	(void)cfg;
	FILE *f = fopen("/proc/net/arp", "r");
	if (!f) { if (iface_out && sz) iface_out[0] = '\0'; return; }
	char line[256];
	fgets(line, sizeof(line), f);
	while (fgets(line, sizeof(line), f)) {
		char tmac[32], iface[32];
		if (sscanf(line, "%*s %*s %*s %31s %*s %31s", tmac, iface) >= 2 && strcasecmp(tmac, mac) == 0) {
			strncpy(iface_out, iface, sz - 1);
			iface_out[sz - 1] = '\0';
			fclose(f);
			return;
		}
	}
	fclose(f);
	if (iface_out && sz) iface_out[0] = '\0';
}

void pushbot_first(config_t *cfg, const char *ip_path, const char *log_path,
	device_info_t *current, int n_current,
	device_info_t *new_list, int *n_new,
	notify_cb on_up, notify_cb on_down, void *cb_ctx) {
	arp_entry_t arp[128];
	int n_arp = read_arp_once(arp, 128);
	device_info_t still[MAX_DEVICES];
	int n_still = 0;
	int seen[128];
	memset(seen, 0, sizeof(seen));

	for (int i = 0; i < n_current; i++) {
		char iface[32];
		arp_get_iface(current[i].ip, iface, sizeof(iface));
		if (ping_ok(current[i].ip, cfg->down_timeout, cfg->timeout_retry, iface)) {
			still[n_still++] = current[i];
		} else {
			if (on_down) on_down(cb_ctx, &current[i], NULL);
		}
	}

	for (int i = 0; i < n_still; i++)
		new_list[*n_new++] = still[i];

	for (int a = 0; a < n_arp && *n_new < MAX_DEVICES; a++) {
		int j;
		for (j = 0; j < *n_new; j++)
			if (strcmp(new_list[j].ip, arp[a].ip) == 0) break;
		if (j < *n_new) continue;
		char iface[32];
		arp_get_iface(arp[a].ip, iface, sizeof(iface));
		if (!ping_ok(arp[a].ip, cfg->up_timeout, cfg->timeout_retry, iface)) continue;
		if (blackwhitelist(cfg, arp[a].mac)) continue;
		device_info_t d;
		strncpy(d.ip, arp[a].ip, sizeof(d.ip)-1);
		strncpy(d.mac, arp[a].mac, sizeof(d.mac)-1);
		get_name(cfg, d.ip, d.mac, d.name, sizeof(d.name));
		get_interface_for_mac(cfg, d.mac, d.iface, sizeof(d.iface));
		d.timestamp = time(NULL);
		new_list[*n_new] = d;
		(*n_new)++;
		if (cfg->pushbot_up && on_up) on_up(cb_ctx, &d, NULL);
	}
}

int device_ping_ok(const char *ip, int timeout_sec, int retry) {
	char iface[32];
	arp_get_iface(ip, iface, sizeof(iface));
	return ping_ok(ip, timeout_sec, retry, iface);
}
