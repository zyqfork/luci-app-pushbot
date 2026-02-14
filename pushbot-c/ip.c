#include "common.h"
#include "config.h"
#include "ip.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

int get_wan_ipv4(const config_t *cfg, char *out, size_t sz) {
	(void)cfg;
	FILE *f = popen("curl -s -m 8 -A 'curl/7.0' 'https://api.ipify.org' 2>/dev/null", "r");
	if (!f) return -1;
	if (!fgets(out, (int)sz, f)) { pclose(f); return -1; }
	pclose(f);
	size_t len = strlen(out);
	if (len > 0 && out[len-1] == '\n') out[len-1] = '\0';
	return 0;
}

int get_wan_ipv6(const config_t *cfg, char *out, size_t sz) {
	(void)cfg;
	FILE *f = popen("curl -s -m 8 -6 -A 'curl/7.0' 'https://api6.ipify.org' 2>/dev/null", "r");
	if (!f) return -1;
	if (!fgets(out, (int)sz, f)) { pclose(f); return -1; }
	pclose(f);
	size_t len = strlen(out);
	if (len > 0 && out[len-1] == '\n') out[len-1] = '\0';
	return 0;
}

void ip_state_load(const char *dir, char *ipv4_out, size_t iv4_sz, char *ipv6_out, size_t iv6_sz) {
	char path[MAX_STR];
	snprintf(path, sizeof(path), "%s/ip", dir);
	FILE *f = fopen(path, "r");
	if (!f) { if (ipv4_out && iv4_sz) ipv4_out[0] = '\0'; if (ipv6_out && iv6_sz) ipv6_out[0] = '\0'; return; }
	if (fgets(ipv4_out, (int)iv4_sz, f)) {
		size_t l = strlen(ipv4_out);
		if (l > 0 && ipv4_out[l-1] == '\n') ipv4_out[l-1] = '\0';
	} else if (ipv4_out && iv4_sz) ipv4_out[0] = '\0';
	if (fgets(ipv6_out, (int)iv6_sz, f)) {
		size_t l = strlen(ipv6_out);
		if (l > 0 && ipv6_out[l-1] == '\n') ipv6_out[l-1] = '\0';
	} else if (ipv6_out && iv6_sz) ipv6_out[0] = '\0';
	fclose(f);
}

void ip_state_save(const char *dir, const char *ipv4, const char *ipv6) {
	char path[MAX_STR];
	snprintf(path, sizeof(path), "%s/ip", dir);
	FILE *f = fopen(path, "w");
	if (!f) return;
	fprintf(f, "%s\n%s\n", ipv4 ? ipv4 : "", ipv6 ? ipv6 : "");
	fclose(f);
}
