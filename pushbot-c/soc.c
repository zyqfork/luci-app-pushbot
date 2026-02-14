#include "common.h"
#include "soc.h"
#include "util.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <dirent.h>
#include <unistd.h>

void soc_run(void) {
	ensure_dir(DIR_TMP);
	char out[32];
	out[0] = '\0';
	/* thermal_zone */
	DIR *d = opendir("/sys/class/thermal");
	if (d) {
		struct dirent *e;
		int max_c = 0;
		while ((e = readdir(d)) != NULL) {
			if (strncmp(e->d_name, "thermal_zone", 12) != 0) continue;
			char path[256];
			snprintf(path, sizeof(path), "/sys/class/thermal/%s/temp", e->d_name);
			FILE *f = fopen(path, "r");
			if (!f) continue;
			int v;
			if (fscanf(f, "%d", &v) == 1 && v > max_c) max_c = v;
			fclose(f);
		}
		closedir(d);
		if (max_c > 0) snprintf(out, sizeof(out), "%d", max_c / 1000);
	}
	if (out[0] == '\0') {
		FILE *f = popen("sensors 2>/dev/null | grep -oE '+[0-9.]+°C' | head -1", "r");
		if (f && fgets(out, sizeof(out), f)) {
			size_t len = strlen(out);
			if (len > 0 && out[len-1] == '\n') out[len-1] = '\0';
		}
		pclose(f);
	}
	if (out[0] == '\0') return;
	char path[MAX_STR];
	snprintf(path, sizeof(path), "%s/soc_tmp", DIR_TMP);
	FILE *fp = fopen(path, "w");
	if (fp) { fputs(out, fp); fclose(fp); }
}
