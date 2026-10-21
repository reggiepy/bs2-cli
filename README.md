# BS2 CLI

飞智 BS2 压风式散热器的命令行工具。通过蓝牙查询状态、调整转速和切换档位，可独立于飞智空间站运行。

使用 Go 编写，目前支持 Windows x64。已实测固定转速控制、持续监测和退出恢复；自动温控、RGB 灯效和 USB 连接暂不支持。

## 构建

项目使用 [mise](https://mise.jdx.dev/) 管理 Go，版本固定在 `mise.toml` 中。

```powershell
git clone https://github.com/reggiepy/bs2-cli.git
cd bs2-cli
mise trust
mise install
mise run build
```

生成的程序位于 `dist\bs2.exe`，运行时无需安装 Go 或飞智空间站。

## 开始使用

打开电脑蓝牙，接通散热器电源，并从系统托盘退出飞智空间站。

```powershell
# 查看 Windows 中已知的 BS2 设备
.\dist\bs2.exe devices

# 查询当前模式、实际转速和目标转速
.\dist\bs2.exe status

# 持续查看状态，按 Ctrl+C 结束
.\dist\bs2.exe watch
```

只有一台已知 BS2 时会自动选择。多台设备可通过 `--address` 指定，也可以保存默认地址。将下面的地址替换为 `devices` 返回的地址：

```powershell
.\dist\bs2.exe config --address AA:BB:CC:DD:EE:FF
.\dist\bs2.exe config
```

配置保存在 `%APPDATA%\bs2-cli\config.json`。地址按 `--address`、配置文件、唯一已知设备的顺序选择。使用 `--config 文件路径` 可指定其他配置文件。

## 调整转速

### 持续控制

```powershell
# 固定为 2800 RPM，按 Ctrl+C 退出并恢复原来的模式和目标转速
.\dist\bs2.exe control --rpm 2800

# 使用第 7 档，运行 30 秒后恢复
.\dist\bs2.exe control --level 7 --duration 30s

# 退出时保留设置，不执行恢复
.\dist\bs2.exe control --rpm 2800 --restore=false
```

`control` 设置转速后保持蓝牙连接，持续读取设备状态。如果通过物理按键或其他程序改变了模式、目标转速，命令会停止，并保留新的设置。

正常退出时会尝试恢复进入前的模式和目标转速，成功后输出 `restored` 事件。断电、蓝牙断开或强制结束进程时，可能无法恢复。

### 一次性设置

```powershell
# 设置后退出，不恢复原设置
.\dist\bs2.exe set --rpm 2800

# 设置后观察 8 秒再退出，同样不恢复
.\dist\bs2.exe set --level 7 --hold 8s

# 切回强劲模式，使用设备中保存的默认转速
.\dist\bs2.exe preset --number 3
```

`preset --number` 对应静音（1）、标准（2）、强劲（3）。断开连接后转速是否保持、断电后是否保存，尚未验证。

### 档位表

九档转速与空间站的 BS2 配置一致，可用 `levels` 命令查看。

| 模式 | 档位 | 转速（RPM） |
|---|---|---|
| 静音 | 1 / 2 / 3 | 1300 / 1700 / 1900 |
| 标准 | 4 / 5 / 6 | 2100 / 2400 / 2700 |
| 强劲 | 7 / 8 / 9 | 2800 / 2900 / 3000 |

`--rpm` 支持 1300–3000，与 `--level` 二选一。按档位调速时使用对应的 RPM，不修改设备为静音、标准、强劲模式保存的默认档位。

## 设备设置

以下命令已实现，但尚未完成实机切换验证：

| 设置 | 命令参数 | 可选值 |
|---|---|---|
| 档位指示灯 | `--name gear-led` | `on` / `off` |
| 通电自启动 | `--name power-on` | `on` / `off` |
| 智能启停 | `--name smart-stop` | `off` / `instant` / `delay` |

```powershell
.\dist\bs2.exe option --name gear-led --value on
.\dist\bs2.exe option --name power-on --value on
.\dist\bs2.exe option --name smart-stop --value off
```

结果中的 `acknowledged: true` 表示设备接受了指令。这些设置暂时没有查询接口，因此 `readback_verified` 为 `false`，也不提供自动恢复。

## 参数与输出

| 参数 | 用途 |
|---|---|
| `--address` | 指定设备蓝牙地址 |
| `--config` | 指定配置文件 |
| `--interval` | `watch`、`control` 的查询间隔，默认 1 秒，支持 500ms–10s |
| `--duration` | `watch`、`control` 的运行时长，默认 0，持续到 Ctrl+C |
| `--timeout` | 操作超时，默认 30 秒 |
| `--debug` | 向 stderr 输出十六进制通信日志 |

普通命令输出 JSON。`watch` 和 `control` 每行输出一个带时间戳的 JSON 事件，便于重定向到文件或交给脚本处理。

普通命令的 `--timeout` 限制整条命令的耗时。对 `watch`、`control`，它限制单次操作的停滞时间，最小为 15 秒，不限制总运行时长。

## 常见问题

**扫描结果为空**

`scan --duration 8s` 只能发现正在广播的设备。已连接的 BS2 可能不再广播，可以用 `devices` 查找 Windows 中的设备记录，再按地址连接。设备记录存在不代表设备在线。

**提示设备被占用**

先退出飞智空间站。同一设备也不能同时运行多个 BS2 CLI 控制或查询命令；运行 `watch`、`control` 时，直接查看它们输出的状态即可。

**实际转速与目标转速不同**

风扇需要时间达到目标，读数也会有波动。`current_rpm` 是实际转速，`target_rpm` 是设备接受的目标值。

**退出后没有恢复**

只有 `control` 默认执行恢复。`set` 不恢复，`control --restore=false` 也不恢复。连接中断或检测到外部控制变化时，请重新运行 `status` 确认状态，必要时用 `preset` 切回原模式。

## 开发

```powershell
mise exec --command "go test -race ./..."
mise exec --command "go vet ./..."
mise exec --command "golangci-lint run"
```

Windows 下的 race 测试需要 GCC，例如 MinGW-w64；`golangci-lint` 需另行安装。

协议来自本机安装版空间站的分析，实测设备固件返回 `00 00 03 05`。协议格式、测试记录和已知差异见 [协议记录](evidence/protocol.md)。
