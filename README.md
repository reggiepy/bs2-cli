# BS2 CLI（Windows 蓝牙验证版）

使用 Go 独立控制飞智 BS2 压风式散热器，无需打开空间站，也不加载飞智 DLL、.NET 或 Python。Go 版本通过 `mise.toml` 固定为 **1.26.5**。Windows 自身的蓝牙驱动和 WinRT 仍然是运行依赖。

## 已完成的实机验证

2026-10-21，在本机名为 `FlyDigi BS2`、固件原始字节为 `00 00 03 05` 的设备上：

- 查询模式、目标转速、实际转速。
- 空间站完全退出后，设置 **2800 RPM**，连续读回目标 2800、实际 2800 RPM（过程中有 2700/2900 RPM 的瞬时读数）。
- 测试后恢复原始模式编号 **3**；最终再次读回目标和实际转速均为 **3000 RPM**。
- Windows 设备记录枚举成功。广播扫描返回空列表；已连接或不广播的设备请用 `devices` 查地址。

当前只实现蓝牙。USB、有线 HID、RGB、智能启停、CPU/GPU 自动温控、后台自启动均未实现。`--hold` 仅保持连接并查询，不是自动温控，也不承诺断线后的固定转速保持行为。

## 使用

先从系统托盘完全退出飞智空间站。工具在连接前检查空间站进程，发现它运行会报错；检查之后再启动空间站，仍可能发生控制竞争。不要让其他程序同时控制该散热器。

在本目录 PowerShell 中运行：

```powershell
# 已编译好的程序
.\dist\bs2.exe devices
.\dist\bs2.exe status --address DE:32:62:6D:30:20

# 设置固定目标，并保持连接观察 8 秒，最后输出读回状态
.\dist\bs2.exe set --address DE:32:62:6D:30:20 --rpm 2800 --hold 8s

# 恢复本次测试前的模式（3）
.\dist\bs2.exe preset --address DE:32:62:6D:30:20 --number 3

# 仅扫描正在广播的 BS2
.\dist\bs2.exe scan --duration 8s
```

地址是本次本机验证设备的地址；其他电脑应以 `devices` 或 `scan` 的输出为准。`devices` 读取 Windows 已知设备记录，不代表设备当前在线。

普通 `set` 执行一次即退出；`--hold` 到期也会退出，**不会自动恢复先前档位**。需要恢复时显式运行 `preset`。试验版将 `--rpm` 限定为 1300–3000，只有 2800 和 2900 进行过设置验证，不代表该范围内每个转速都已测试。

输出为 JSON，诊断和错误写入 stderr。添加 `--debug` 查看收发十六进制数据。`firmware_hex` 保留原始字节，不把它擅自解释成版本号。

默认整条命令最长 30 秒；可用 `--timeout 60s` 调整。Windows BLE 库的部分系统调用无法用 Go context 取消，因此 CLI 通过一个自己的子进程执行 BLE 操作，超时仅终止该子进程。写操作超时不代表设备未改变，请重新查询状态。一个命令运行期间临时会有父、子两个 `bs2.exe` 进程，退出后均结束。

## 构建与检查

```powershell
mise exec --command "go version"
# 若 mise 报配置未信任，在核对本目录配置后执行 mise trust
# 若固定版本尚未安装，执行 mise install
mise exec --command "go test -race ./..."
mise exec --command "go vet ./..."
mise exec --command "golangci-lint run"
mise exec --command "go build -trimpath -o dist/bs2.exe ./cmd/bs2"
```

也可使用 `mise run build`、`mise run test`、`mise run vet`。本机 race 检查使用现有 MinGW GCC；普通可执行文件不需要飞智开发工具。`golangci-lint` 是本机已有的检查工具，不包含在本仓库工具版本固定范围内。

## 项目结构

- `cmd/bs2`：命令行、JSON 输出、进程超时隔离。
- `internal/ble`：Windows BLE、设备枚举、占用检查。
- `internal/protocol`：帧编码、校验和分包重组。
- `internal/device`：设备查询、调速与档位指令。
- `evidence/protocol.md`：协议来源、实测差异和限制。

本版基于 Windows/amd64 验证；不宣称支持 Linux、macOS 或其他散热器型号。短时间重复连接曾出现 Windows 暂时未发现特征的问题，现仅对特征发现做最多两次额外重试；不会自动重发已执行的调速指令。

本次检查：`go test -race ./...`、`go vet ./...`、`golangci-lint run` 全部通过。构建为 Go 1.26.5 / windows / amd64 / CGO_ENABLED=1，产物 4,528,640 字节。SHA-256 和构建记录见 `dist/SHA256SUMS`、`dist/build-info.json`。
