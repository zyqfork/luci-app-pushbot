#ifndef PUSHBOT_IP_H
#define PUSHBOT_IP_H

#include "common.h"

int get_wan_ipv4(const config_t *cfg, char *out, size_t sz);
int get_wan_ipv6(const config_t *cfg, char *out, size_t sz);
void ip_state_load(const char *dir, char *ipv4_out, size_t iv4_sz, char *ipv6_out, size_t iv6_sz);
void ip_state_save(const char *dir, const char *ipv4, const char *ipv6);

#endif
