#include "common.h"
#include "config.h"
#include "push.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netdb.h>
#include <unistd.h>

/* Minimal HTTP POST via socket; no TLS - for simple webhooks use http:// or rely on curl fallback */
static int http_post(const char *url, const char *body, size_t body_len, const char *content_type) {
	char host[256], path[512], scheme[16];
	int port = 80;
	if (sscanf(url, "%15[^:]://%255[^:/]%*[:]%d%511s", scheme, host, &port, path) < 2) {
		if (sscanf(url, "%15[^:]://%255[^/]%511s", scheme, host, path) < 2)
			return -1;
		path[0] = '/'; path[1] = '\0';
	}
	if (path[0] == '\0') { path[0] = '/'; path[1] = '\0'; }
	if (port <= 0) port = 80;

	struct hostent *he = gethostbyname(host);
	if (!he) return -1;
	int fd = socket(AF_INET, SOCK_STREAM, 0);
	if (fd < 0) return -1;
	struct sockaddr_in addr;
	memset(&addr, 0, sizeof(addr));
	addr.sin_family = AF_INET;
	addr.sin_port = htons((uint16_t)port);
	memcpy(&addr.sin_addr, he->h_addr_list[0], (size_t)he->h_length);
	if (connect(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) { close(fd); return -1; }

	char req[4096];
	int hlen = snprintf(req, sizeof(req),
		"POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: %s\r\nContent-Length: %zu\r\nConnection: close\r\n\r\n",
		path, host, content_type ? content_type : "application/json", body_len);
	if (hlen >= (int)sizeof(req) || hlen + (int)body_len >= (int)sizeof(req)) { close(fd); return -1; }
	memcpy(req + hlen, body, body_len);
	size_t total = (size_t)hlen + body_len;
	ssize_t sent = send(fd, req, total, 0);
	close(fd);
	return (sent == (ssize_t)total) ? 0 : -1;
}

static void substitute(const char *in, const char *var, const char *val, char *out, size_t outsz) {
	const char *p = strstr(in, var);
	if (!p) { strncpy(out, in, outsz - 1); out[outsz - 1] = '\0'; return; }
	size_t pre = (size_t)(p - in);
	if (pre >= outsz) pre = outsz - 1;
	memcpy(out, in, pre);
	out[pre] = '\0';
	size_t l = strlen(out);
	if (val) {
		strncat(out, val, outsz - l - 1);
		l = strlen(out);
	}
	p += strlen(var);
	strncat(out, p, outsz - l - 1);
}

static int load_template_fields(const char *jsonpath, char *url_out, size_t url_sz,
	char *data_out, size_t data_sz, char *ct_out, size_t ct_sz) {
	FILE *f = fopen(jsonpath, "r");
	if (!f) return -1;
	char buf[8192];
	size_t n = fread(buf, 1, sizeof(buf) - 1, f);
	fclose(f);
	buf[n] = '\0';

	const char *p;
	p = strstr(buf, "\"url\"");
	if (p) { p = strchr(p, ':'); if (p) { p = strchr(p, '"'); if (p) { p++; const char *e = strchr(p, '"'); if (e) { size_t len = (size_t)(e - p); if (len >= url_sz) len = url_sz - 1; memcpy(url_out, p, len); url_out[len] = '\0'; } } } }
	p = strstr(buf, "\"content_type\"");
	if (p) { p = strchr(p, ':'); if (p) { p = strchr(p, '"'); if (p) { p++; const char *e = strchr(p, '"'); if (e) { size_t len = (size_t)(e - p); if (len >= ct_sz) len = ct_sz - 1; memcpy(ct_out, p, len); ct_out[len] = '\0'; } } } }
	p = strstr(buf, "\"data\"");
	if (p) { p = strchr(p, ':'); if (p) { p = strchr(p, '"'); if (p) { p++; const char *e = strchr(p, '"'); if (e) { size_t len = (size_t)(e - p); if (len >= data_sz) len = data_sz - 1; memcpy(data_out, p, len); data_out[len] = '\0'; } } } }
	return 0;
}

int push_send(const config_t *cfg, const char *title, const char *content) {
	char url[MAX_STR], data_buf[2048], ct[128];
	if (load_template_fields(cfg->jsonpath, url, sizeof(url), data_buf, sizeof(data_buf), ct, sizeof(ct)) != 0)
		return -1;
	/* trim quotes from url */
	if (url[0] == '"') memmove(url, url + 1, strlen(url));
	if (url[strlen(url)-1] == '"') url[strlen(url)-1] = '\0';
	/* substitute ${dd_webhook} etc. */
	char tmp[MAX_STR];
	substitute(url, "${dd_webhook}", cfg->dd_webhook, tmp, sizeof(tmp));
	strncpy(url, tmp, sizeof(url)-1);
	/* body: if data is @path, write temp.json then read; else substitute in data_buf */
	char body[4096];
	if (data_buf[0] == '@') {
		snprintf(tmp, sizeof(tmp), "%s/temp.json", cfg->dir);
		FILE *f = fopen(tmp, "w");
		if (f) {
			/* minimal markdown body for dingtalk-style */
			fprintf(f, "{\"msgtype\":\"markdown\",\"markdown\":{\"title\":\"%s\",\"text\":\"%s\n\n%s\"}}",
				title ? title : "", title ? title : "", content ? content : "");
			fclose(f);
		}
		f = fopen(tmp, "r");
		if (!f) return -1;
		size_t n = fread(body, 1, sizeof(body) - 1, f);
		fclose(f);
		body[n] = '\0';
	} else {
		substitute(data_buf, "${1}", title, tmp, sizeof(tmp));
		substitute(tmp, "${2}", content, body, sizeof(body));
	}
	size_t body_len = strlen(body);
	if (strncmp(url, "https://", 8) == 0) {
		/* TLS not implemented; fallback to curl */
		char cmd[4096];
		snprintf(cmd, sizeof(cmd), "curl -s -X POST -H \"Content-Type: %s\" -d '%s' '%s' 2>/dev/null",
			ct[0] ? ct : "application/json", body, url);
		return system(cmd) == 0 ? 0 : -1;
	}
	return http_post(url, body, body_len, ct[0] ? ct : "application/json");
}
