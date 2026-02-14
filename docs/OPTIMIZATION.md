# PushBot 脚本优化建议

当前主脚本约 **1884 行**，单文件维护成本高。以下建议按「投入小 → 投入大」排序，可逐步实施。

---

## 一、快速见效（不拆文件）

### 1. 抽取「消息块」拼接函数

多处重复模式：用 `str_splitline`、`str_tab`、`str_linefeed` 拼一段 content。可统一成：

```sh
# 用法: append_msg "标题文字" "行1" "行2" ...
append_msg() {
    local title_text="$1"
    shift
    local line
    content="${content}${str_splitline}${str_title_start}${title_text}${str_title_end}"
    for line in "$@"; do
        content="${content}${str_linefeed}${str_tab}${line}"
    done
}
```

在**设备断开**、**登录成功/失败**、**当前设备列表**等处复用，可减少几十行重复。

### 2. 登录通知合并为统一逻辑

`login_send` 内 Web 成功 / SSH 成功 / Web 失败 / SSH 失败 四块结构几乎相同，可改为：

- 公共函数：`login_append_notification type ip time mode`（type=web_ok/ssh_ok/web_fail/ssh_fail）
- 根据 type 选 title 模板和 content 模板，再调用 `append_msg` 或上述拼接函数

这样 `login_send` 从 ~110 行可压到 ~40 行以内，逻辑更清晰。

### 3. 去掉未使用/冗余代码

- 搜索 `smart_device_scan`：若从未被调用，可删除整段（约 40 行）。
- `pushbot_first` 里对 `total_count` 的赋值若只用于注释或未使用，可删减。
- 重复的 `echo "date ..." >> ${logfile}` 可封装成 `log_msg "xxx"`（若与现有 log 函数不重复）。

---

## 二、按功能拆成多个脚本（模块化）

OpenWrt 下 sh 多为 ash，支持 `. file`  sourced 子脚本。建议目录：

```
/usr/bin/pushbot/
  pushbot              # 主入口，只做：读配置、source 各 lib、主循环
  lib/
    config.sh           # 配置读取、load_configs、read_config、get_config
    util.sh             # 日志、deltemp、getmac/getname/getip、time_for_humans、cut_str 等
    network.sh          # getinterfacelist、ip_changes、rand_geturl、网络检测与恢复
    device.sh           # up、down、pushbot_first、blackwhitelist、get_client、current_device
    notify.sh           # diy_send、append_msg、down_send、login_send、send（定时）
    cron.sh             # pushbot_cron、pushbot_disturb、geterrdevicealiases、unattended
    system.sh           # cpu_load、soc_temp、get_syslog、add_ip_black、network_restart
```

主脚本 `pushbot` 只保留：

- 全局变量与常量
- `read_config`（或改为调用 config.sh 内函数）
- 依次 `. /usr/bin/pushbot/lib/config.sh`、`util.sh`、… 
- 主流程：`pushbot_init` → 启动参数分支 → `while` 主循环

这样主脚本可控制在 **150 行以内**，其余按模块阅读和修改。

**注意**：子脚本里用到的变量（如 `dir`、`jsonpath`、`str_*`）需在主脚本或 config 中先定义再 source，否则子脚本无法使用。

---

## 三、结构与逻辑优化

### 1. 主循环里「发送锁」判断重复

多处出现 `[ ! -f "${dir}send_enable.lock" ] && ...`，可改为：

```sh
send_locked() { [ -f "${dir}send_enable.lock" ]; }

# 主循环里
if ! send_locked; then
    pushbot_first
    down_send
    current_device
    unattended
    cpu_load
    login_send
    # ...
fi
```

减少重复判断，逻辑更集中。

### 2. 长函数拆成「步骤函数」

例如 `send()`（定时发送）：

- `send_build_title`
- `send_build_device_list`
- `send_build_status`（CPU/温度等）
- `send_do_push`

主函数 `send` 只按顺序调用这几步，每步约 20～40 行，便于单测和阅读。

### 3. 配置项集中

- 把 `config_vars`、`get_config` 的 key 列表放到脚本顶部或单独 `CONFIG_KEYS="..."`，避免散落在 `load_configs` 与 `read_config` 两处。
- 若有未使用的配置项，可在此集中标注或删除，避免「幽灵配置」。

---

## 四、实施顺序建议

1. **先做 一.1、一.2、一.3**：不改变执行路径，只减重复、缩长度，风险小。
2. **再考虑 二**：模块化时先只拆出 `util.sh`、`config.sh`，确认运行正常后再拆 `device.sh`、`notify.sh` 等。
3. **最后做 三**：在模块稳定后，再重构主循环和长函数，便于后续加功能或修 bug。

---

## 五、行数大致预期

| 项目           | 当前约 | 优化后约 |
|----------------|--------|----------|
| 主脚本         | 1884   | 150～300（若拆模块则更短） |
| 重复 content 拼接 | 分散   | 集中为 1～2 个函数 |
| login_send     | ~110   | ~40     |
| 单文件最大     | 1884   | 建议单文件 < 400 行 |

按上述步骤做完，总行数可能略增（多出函数定义和空行），但**单文件行数**和**重复度**会明显下降，后续维护和扩展会更轻松。
