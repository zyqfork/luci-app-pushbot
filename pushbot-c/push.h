#ifndef PUSHBOT_PUSH_H
#define PUSHBOT_PUSH_H

#include "common.h"

int push_send(const config_t *cfg, const char *title, const char *content);

#endif
