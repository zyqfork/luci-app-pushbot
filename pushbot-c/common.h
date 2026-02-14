#ifndef PUSHBOT_COMMON_H
#define PUSHBOT_COMMON_H

#include <stddef.h>
#include <stdint.h>
#include <time.h>

#define CONFIG_SECTION "pushbot.@pushbot[0]"
#define DIR_TMP       "/tmp/pushbot"
#define CONFIG_DIR    "/usr/bin/pushbot"
#define MAX_DEVICES   256
#define MAX_LINE      1024
#define MAX_STR       512

typedef struct device_info {
	char ip[64];
	char mac[32];
	char name[128];
	char iface[32];
	time_t timestamp;
} device_info_t;

typedef struct config {
	char dir[MAX_STR];
	char config_dir[MAX_STR];
	int pushbot_enable;
	char device_name[MAX_STR];
	int sleeptime;
	int debuglevel;
	char jsonpath[MAX_STR];
	int pushbot_up;
	int pushbot_down;
	int up_timeout;
	int down_timeout;
	int timeout_retry;
	int pushbot_sheep;
	int starttime;
	int endtime;
	int pushbot_ipv4;
	int pushbot_ipv6;
	int cpuload_enable;
	int cpuload;
	int temp_enable;
	int temperature;
	char regular_time[32];
	char regular_time2[32];
	char regular_time3[32];
	char interval_time[32];
	char pushbot_whitelist[MAX_LINE];
	char pushbot_blacklist[MAX_LINE];
	char pushbot_interface[MAX_STR];
	int err_enable;
	char err_device_aliases[MAX_LINE];
	int network_err_event;
	int system_time_event;
	/* push tokens - simplified, only used by template */
	char dd_webhook[MAX_STR];
	char bark_token[MAX_STR];
	char bark_srv[MAX_STR];
} config_t;

#endif
