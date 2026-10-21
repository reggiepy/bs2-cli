# BS2 协议与验证记录

## 静态来源

安装目录：`C:\Program Files\FlydigiSpaceStation`。主程序集版本 3.4.9.5。

- REA 主程序分类 Evidence：`ev_97c07f60176b59ad605cfe63ad64d6866cfe5908e4ab526c37324631daa09b9d`。
- `BLEServiceController.dll` 成员 Evidence：`ev_7fe36b136ae3f6cbc9903bb070dea284faeb2f3820419b4701a2097946857765`。
- `Middleware.dll` 成员 Evidence：`ev_ccf56a60286b842d37fffc375802ca59f45394be27eedebcf7087106f1d022be`。
- 辅助反编译：ILSpyCmd 8.2.0.7535。C# 重建用于理解逻辑，不是厂商源码。私有反编译产物在 `.tools/`，不属于 Go 工具运行依赖，不打包发布。
- `BLEConfigV2.fdg` 给出服务 65520/0xFFF0、写特征 65522/0xFFF2、通知特征 65521/0xFFF1。

协议追踪到 `ProtocolBase.Package/Content`、`BS1Protocol`、`CrcUtil.Crc8`、各 `*Proto` 构造函数以及 `FanSpeedSettingService`。`Util.Bytes.UShort2Bytes` 和 `FanSpeedConsultStruct` 确认转速为 uint16 小端。

## 帧

`5A A5 | command | length | payload | checksum`

`length = 2 + payload字节数`，整帧长为 `length + 3`。checksum 为 command、length、payload 的逐字节和模 256。原库函数名叫 Crc8，但不是多项式 CRC。原库接收解包没有真正验证校验和；本实现要求校验通过。

| 指令 | 请求 payload | 应答 payload | 验证情况 |
|---|---|---|---|
| 01 | 无 | 固件原始 4 字节 | 实测 |
| 22 | 无 | 当前转速 uint16 LE + 目标转速 uint16 LE | 实测 |
| 25 | 无 | 原始模式编号 1 字节 | 实测 |
| 23 | 无 | 软件控制应答；原库接受 01 或 03 | 实测返回 01 |
| 21 | RPM uint16 LE | 01 成功 | 实测 2800、2900 |
| 08 | 档位数值 | 01 成功 | 实测 03 和 04；最终仅保留 03 |
| 24 | 无 | 退出软件控制 | 实测出现 01 和 02；最终工具不使用 |

通知是连续字节流，可能一条通知包含多帧，也可能在任意字节处断帧。解析器跨请求保留不完整帧，解析同一通知的所有完整帧，再按 command 匹配。订阅中途遇到前一帧尾部时搜索下一个帧头。不存在已确认的请求序号；同一命令的旧应答、其他控制器应答仍可能混淆，因此通过控制独占检查和设置后读回降低误判，不能声称完全消除所有陈旧应答风险。

## 实机证据（2026-10-21）

设备名 `FlyDigi BS2`；固件字节 `00 00 03 05`。关键原始帧：

```text
查询固件 TX: 5A A5 01 02 03
         RX: 5A A5 01 06 00 00 03 05 0F
原始模式 RX: 5A A5 25 03 03 2B
原始转速 RX: 5A A5 22 06 B8 0B B8 0B AE

进入软件控制 TX: 5A A5 23 02 25
             RX: 5A A5 23 03 01 27
设置 2800    TX: 5A A5 21 04 F0 0A 1F
             RX: 5A A5 21 03 01 25
软件控制模式 RX: 5A A5 25 03 05 2D
实际/目标2800 RX: 5A A5 22 06 F0 0A F0 0A 1C

恢复原始模式 TX: 5A A5 08 03 03 0E
             RX: 5A A5 08 03 01 0C
恢复模式读回 RX: 5A A5 25 03 03 2B
恢复初期转速 RX: 5A A5 22 06 54 0B B8 0B 4A
```

最后一条代表恢复初期实际 2900、目标 3000；风扇需要时间收敛，不能把目标值直接当成实际值。

最终构建运行验证再次收到 `5A A5 25 03 03 2B` 和 `5A A5 22 06 B8 0B B8 0B AE`，确认模式 03、实际/目标均已恢复 3000 RPM。

## 纠正与限制

1. 厂商 `FanSpeedDeviceMode` 枚举为 0-based，但本设备实测模式查询：03 对应发送档位 03，04 对应档位 04，进入软件控制后返回 05。初次错误映射将恢复指令发成 04，使目标短暂达到 4000。已改回 03/3000，并按实测编号修正；01/02 仍未经实机验证，00 标记 unknown。
2. 第二轮测试期间空间站运行，通知混有其他指令应答，目标从请求的 2800 变成 2100。程序报告读回不一致，没有把 ACK 当作设置成功。用户退出空间站后重试，连续观察实际/目标均达到 2800，然后恢复 03/3000。
3. 固定转速在连接维持期间验证成功。断开后是否保持、断电是否保留、是否存在设备 watchdog 尚未专项确认。不要把当前程序当成后台温控服务。
4. 广播扫描未发现设备；Windows 已知设备枚举和按地址直连成功。Windows 快速重连有时暂时返回空特征列表，工具仅重试发现阶段。
5. 没有修改厂商安装文件、刷固件、安装驱动或改变系统蓝牙配对。

## 原生界面对照与持续控制（后续开发）

用户提供的两张原生界面截图分别显示固定转速和智能变频，另有档位灯、智能启停和通电自启动。截图只提供交互语义，不作为协议字节的推断依据。

使用同一 ILSpyCmd 从同一安装程序集反编译 `FlydigiSpaceStation.Common.Utils.FanSpeedHelper`，核对 `DEVICE_MODES[TestBLEDeviceType.BS2]`：

- 静音三档：1300、1700、1900；厂商默认 1700。
- 标准三档：2100、2400、2700；厂商默认 2400。
- 强劲三档：2800、2900、3000；厂商默认 3000。
- Pro 具有额外 3500/3700/4000 组，不用于此 BS2 的公开命令。

`--level` 只是对应固定 RPM 的软件控制入口，不宣称已经实现原生默认档位持久化。`preset --number` 对外限制为 1–3。

`BLEServiceController.Service.Temperature.TemperatureService` 的 `Open/UpdateTemperature` 表明温度来自 `CPU Package` 和 `GPU Core`，不是通过已知 BS2 蓝牙查询获得。自动温控尚未实现。

从已分析的 `BLEServiceController.dll` 中追踪以下构造函数和 Run 方法（静态重建，尚未实机更改这些选项）：

| CLI 设置 | 类型 | 指令 | payload |
|---|---|---|---|
| gear-led | SetGearPositionLEDProto / Service | 48 | on=01，off=00 |
| power-on | SetAutoStartProto / Service | 0C | on=01，off=02 |
| smart-stop | SetSmartStartStopProto / Service + SmartStartStopMode | 0D | off=00，instant=01，delay=02 |

编码与 NACK 处理通过单元测试。由于没有确认 getter，结果明确包含 `readback_verified:false`。

实机持续控制：`control --level 7 --duration 4s` 从模式 03 / 3000 RPM 进入模式 05 / 2800 RPM，连续观察实际 2800 RPM，然后输出 restored，模式 03 / 目标 3000 RPM。另一次通过关闭父进程取消管道模拟退出，恢复成功；同一时间第二条 status 命令被设备互斥锁拒绝。取消管道路径与父进程收到 Ctrl+C 后使用的路径一致，但没有把它描述成直接发送真实键盘 Ctrl+C 的测试。

通过每次操作边界的心跳及进程监控，使 `watch/control` 总运行时间不受默认 30 秒限制。强制杀进程、断电、链路丢失仍无法保证恢复。检测到外部模式或目标变化时，主动放弃原状态恢复，避免覆盖用户的新设置。
