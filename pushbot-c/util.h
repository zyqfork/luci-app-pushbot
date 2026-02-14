#ifndef PUSHBOT_UTIL_H
#define PUSHBOT_UTIL_H

void ensure_dir(const char *d);
void append_to_file(const char *path, const char *line);
void truncate_log(const char *path, int max_lines);
void app_log(const char *log_path, int debuglevel, const char *msg);
void time_for_humans(int sec, char *out, size_t outsz);
void cut_str(const char *s, int max_len, char *out, size_t outsz);

#endif
