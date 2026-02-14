#ifndef PUSHBOT_DEVICE_H
#define PUSHBOT_DEVICE_H

#include "common.h"

typedef void (*notify_cb)(void *ctx, const device_info_t *d, const char *extra);

void get_name(const config_t *cfg, const char *ip, const char *mac, char *name_out, size_t sz);
int blackwhitelist(const config_t *cfg, const char *mac);
int device_read_list(const char *path, device_info_t *list, int max_n, int *n);
void device_write_list(const char *path, const device_info_t *list, int n);
void pushbot_first(config_t *cfg, const char *ip_path, const char *log_path,
	device_info_t *current, int n_current,
	device_info_t *new_list, int *n_new,
	notify_cb on_up, notify_cb on_down, void *cb_ctx);
int device_ping_ok(const char *ip, int timeout_sec, int retry);

#endif
