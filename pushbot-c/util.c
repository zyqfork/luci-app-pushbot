#include "common.h"
#include "util.h"
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

void ensure_dir(const char *d) {
	struct stat st;
	if (stat(d, &st) == 0 && S_ISDIR(st.st_mode))
		return;
	mkdir(d, 0755);
}

void append_to_file(const char *path, const char *line) {
	FILE *f = fopen(path, "a");
	if (!f) return;
	fputs(line, f);
	fclose(f);
}

void truncate_log(const char *path, int max_lines) {
	/* simplified: only truncate if > max_lines; real impl would rewrite file */
	FILE *f = fopen(path, "r");
	if (!f) return;
	int n = 0;
	char buf[256];
	while (fgets(buf, sizeof(buf), f)) n++;
	fclose(f);
	if (n <= max_lines) return;
	/* keep last max_lines: reopen and rewrite - skip first (n - max_lines) */
	f = fopen(path, "r");
	if (!f) return;
	FILE *tmp = fopen("/tmp/pushbot_log_tmp", "w");
	if (!tmp) { fclose(f); return; }
	int skip = n - max_lines;
	int i = 0;
	while (fgets(buf, sizeof(buf), f)) {
		if (i >= skip) fputs(buf, tmp);
		i++;
	}
	fclose(f); fclose(tmp);
	rename("/tmp/pushbot_log_tmp", path);
}

void app_log(const char *log_path, int debuglevel, const char *msg) {
	time_t t = time(NULL);
	struct tm *tm = localtime(&t);
	char ts[32];
	snprintf(ts, sizeof(ts), "%04d-%02d-%02d %02d:%02d:%02d",
		tm->tm_year + 1900, tm->tm_mon + 1, tm->tm_mday,
		tm->tm_hour, tm->tm_min, tm->tm_sec);
	char line[MAX_LINE + 64];
	snprintf(line, sizeof(line), "%s %s\n", ts, msg);
	append_to_file(log_path, line);
	truncate_log(log_path, 500);
	if (debuglevel != 0)
		fprintf(stderr, "%s\n", msg);
}

void time_for_humans(int sec, char *out, size_t outsz) {
	if (sec < 60)
		snprintf(out, outsz, "%d 秒", sec);
	else if (sec < 3600)
		snprintf(out, outsz, "%d 分 %d 秒", sec / 60, sec % 60);
	else if (sec < 86400)
		snprintf(out, outsz, "%d 小时 %d 分", sec / 3600, (sec % 3600) / 60);
	else
		snprintf(out, outsz, "%d 天 %d 小时", sec / 86400, (sec % 86400) / 3600);
}

void cut_str(const char *s, int max_len, char *out, size_t outsz) {
	if (!s || !out || outsz == 0) return;
	size_t len = strlen(s);
	if (len <= (size_t)max_len) {
		strncpy(out, s, outsz - 1);
		out[outsz - 1] = '\0';
		return;
	}
	int copy = max_len > 0 ? max_len : 14;
	if ((size_t)copy >= outsz) copy = (int)outsz - 1;
	memcpy(out, s, copy);
	out[copy] = '\0';
}
