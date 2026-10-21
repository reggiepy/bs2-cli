# BS2 CLI

用 Go 独立控制飞智 BS2 压风式散热器，支持 Windows 蓝牙。无需空间站、飞智 DLL、.NET 或 Python；仍使用 Windows 蓝牙驱动和 WinRT。Go 由 mise 固定为 **1.26.5**。

## 快速使用

先从托盘退出飞智空间站，在项目目录运行：

```powershell
.\dist\bs2.exe devices
.\dist\bs2.exe status
.\dist\bs2.exe levels

# 持续观察，不改变设置；Ctrl+C 退出
.\dist\bs2.exe watch

# 原生九档中的第 7 档：2800 RPM；退出后恢复进入前的模式和目标
.\dist\bs2.exe control --level 7

# 运行 30 秒后自动恢复
.\dist\bs2.exe control --rpm 2800 --duration 30s

# 一次性设置，退出时不恢复
.\dist\bs2.exe set --rpm 2800

# 切回静音/标准/强劲三个物理模式之一（使用设备已有的默认转速）
.\dist\bs2.exe preset --number 3
```

地址选择优先级：`--address` > 保存的配置 > Windows 唯一一台已知 BS2。若存在多台，不会擅自选择。

```powershell
# 记住本机设备地址（其他设备请用 devices 的输出）
.\dist\bs2.exe config --address DE:32:62:6D:30:20
.\dist\bs2.exe config

# 临时指定设备，或使用另一份配置文件
.\dist\bs2.exe status --address DE:32:62:6D:30:20
.\dist\bs2.exe config --config .\my-bs2.json --address DE:32:62:6D:30:20
```

默认配置为 `%APPDATA%\bs2-cli\config.json`。配置损坏会明确报错，不会悄悄改用其他设备。`devices` 是 Windows 已知设备记录，不证明设备在线；`scan --duration 8s` 只列出正在广播的 BS2。已连接设备不广播时，扫描可能为空，仍可按已知地址连接。

## 对应原生界面

从安装版空间站的 `FanSpeedHelper` 核实 BS2 九档如下：

| 组别 | 全局档位 | 组内 1 / 2 / 3 档 RPM |
|---|---|---|
| 静音 silent | 1–3 | 1300 / 1700 / 1900 |
| 标准 standard | 4–6 | 2100 / 2400 / 2700 |
| 强劲 strong | 7–9 | 2800 / 2900 / 3000 |

`set/control --level N` 使用这些档位对应的 RPM，进入软件控制；它**不会重写原生三个模式保存的默认档位**。`--level` 与 `--rpm` 互斥。固定 RPM 范围为 1300–3000。`preset --number` 只开放 BS2 的 1–3 模式，不开放 Pro 的超频第 4 模式。

截图中的另外三个设置已实现协议发送：

```powershell
.\dist\bs2.exe option --name gear-led --value on        # 档位灯：on / off
.\dist\bs2.exe option --name power-on --value on        # 通电自启动：on / off
.\dist\bs2.exe option --name smart-stop --value off     # 智能启停：off / instant / delay
```

这三项基于原程序指令及单元测试实现，**尚未在本轮实机切换验证**。返回 `acknowledged: true` 只表示设备回执接受，`readback_verified: false` 表示没有已确认的查询接口。工具不伪造这些设置的当前状态，也不自动恢复它们。

智能变频不是简单的设备模式编号。原程序读取电脑 **CPU Package / GPU Core** 温度，再计算风速；当前版本尚未实现温度采集和自动曲线。RGB、USB/HID、原生各模式默认档位写入、开机启动和后台 Windows 服务也尚未实现。

## 持续运行与退出

- `watch` 只读；`control` 设置一次固定转速，然后维持连接并监测设备。
- `--interval` 默认 1 秒，允许 500ms–10s。`--duration` 默认 0，表示直到 Ctrl+C。
- `control` 默认恢复运行前的模式与目标；`--restore=false` 关闭恢复。恢复后会读回检查，并输出 `restored` 事件；风扇实际转速需要时间变化，不要求立刻等于目标。
- 检测到物理按键/其他控制器改变模式或目标时，停止控制并保留新设置，不强行覆盖。空间站在运行中启动也会停止监测；此时跳过恢复，避免两个控制器争抢。
- 同一 Windows 会话内，同一设备只能被一个 `bs2` 命令占用。`control/watch` 运行时，另一个 `status` 会报占用，直接查看原进程的状态流即可。
- 普通命令 `--timeout 30s` 为整条命令的限制；`watch/control` 的 timeout 为单次操作停滞期限（至少 15 秒），不限制总运行时间。
- WinRT 系统调用可能无法取消，因此由父进程监控专用子进程。Ctrl+C 通过管道通知子进程，给予最多 12 秒清理；设备恢复本身使用 10 秒上下文。系统调用一直阻塞时才强制结束，错误中会说明恢复未确认。断电、蓝牙丢失、强制终止进程不能保证恢复。
- 不自动重连或重发已执行的写指令。连接发现阶段允许两次额外重试。

`set --hold 8s` 保留验证版行为：保持连接观察 8 秒，然后退出，**不恢复**。断开后设备是否保持设置、断电是否保留，尚未专项确认。

## 输出与构建

普通命令输出 JSON；`watch/control` 每行输出一个 JSON 事件（`initial`、`controlled`、`status`、`restored`），包含时间戳和设备状态。`--debug` 的十六进制通信日志写入 stderr。`firmware_hex` 保留设备原始字节，不擅自解释版本号。

```powershell
mise exec --command "go version"
mise run build
mise exec --command "go test -race ./..."
mise exec --command "go vet ./..."
mise exec --command "golangci-lint run"
```

若 mise 报未信任配置，核对后执行 `mise trust`；固定工具版本缺失时执行 `mise install`。race 测试使用本机已有 MinGW GCC，lint 使用本机已有 golangci-lint。产物在 `dist/bs2.exe`，大小、构建参数及 SHA-256 见 `dist/build-info.json` 和 `dist/SHA256SUMS`。

## 验证范围

2026-10-21，在设备名 `FlyDigi BS2`、固件字节 `00 00 03 05` 上验证：

- 查询状态、无需输入地址的设备选择、保存默认地址、固定 2800 RPM 实际转速读回。
- 第 7 档定时控制及自动恢复原始模式 3、目标 3000 RPM。
- 取消管道触发恢复，以及重复进程访问被互斥锁拒绝。
- 持续状态流。子进程取消和操作超时另有进程级自动化测试。

只验证 Windows/amd64；未宣称其他系统或其他型号兼容。三个开关和完整九档的硬件行为尚未逐项验证。协议来源与原生枚举的差异记录在 [协议证据](evidence/protocol.md)。
