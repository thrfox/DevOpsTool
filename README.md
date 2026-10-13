# DevOpsTool · 开发运维工具箱

使用 Go + Fyne 开发的跨平台桌面工具合集。Windows 发布为一个可执行文件，双击启动，不需要浏览器、本地网页服务、Go、Node.js 或 WebView。中文字体及界面资源嵌入程序，工具数据在本地处理。

本项目采用 [MIT 许可证](LICENSE)。第三方组件及字体遵循各自的许可证，详见 [第三方组件与资源](THIRD_PARTY.md)。

## 启动

Windows v0.0.1.2026101318 程序位于 `dist/DevOpsTool.exe`。普通工具直接启动即可；修改及恢复网络适配器配置时，请右键选择“以管理员身份运行”。应用图标已嵌入窗口、任务栏和可执行文件，模块采用统一的蓝青色图标。

## GitHub Actions 构建与发布

工作流位于 `.github/workflows/release.yml`。在仓库 **Actions → 桌面构建与标签发布 → Run workflow** 手动运行，会执行测试并构建各平台成果物，供本次运行的 **Artifacts** 下载；手动运行不创建 Release。普通分支提交不会自动触发发布构建。

推送格式为 `v主版本.次版本.修订版本.年月日小时` 的标签，会在全部平台构建成功后自动创建同名 Release。小功能迭代使用最小时间版本，例如 `v0.0.1.2026101318`，时间按上海时区生成。打标签前先确保需要发布的代码及工作流已推送；同一小时内不重复发布同名标签。

```powershell
git pull --ff-only
git tag -a v0.0.1.2026101318 -m "发布：桌面工具箱时间版本"
git push origin v0.0.1.2026101318
```

仅在本地创建标签不会触发 GitHub 构建，必须将标签推送到远端。成果物可在仓库 **Releases** 页面下载：

| 平台 | 架构 | 成果物 |
| --- | --- | --- |
| Windows | amd64 | 可直接启动的 `.exe`，另附包含许可证的 `.zip` |
| Linux | amd64 | 含程序及许可证的 `.tar.gz`；需要桌面图形库，建议 Ubuntu 22.04 或更新环境 |
| macOS | amd64、arm64 | 含 `.app` 应用包及许可证的 `.tar.gz`；选择与电脑处理器匹配的架构 |

Release 同时附带 `SHA256SUMS.txt`。Windows 可用 `Get-FileHash 文件名 -Algorithm SHA256` 对照；Linux 使用 `sha256sum -c SHA256SUMS.txt`，macOS 使用 `shasum -a 256 -c SHA256SUMS.txt`（检查全部成果物时须先下载清单中的所有文件）。

标签完整版本会写入界面及关于窗口，预发布标签如 `v0.0.1.2026101318-rc.1` 会标记为预发布。macOS 系统包版本使用前三段及工作流构建序号，界面和成果物名称仍显示完整时间版本。macOS 包使用本地临时签名，尚未接入 Apple 开发者证书及公证。

构建作业只有源码读取权限；发布作业通过 GitHub 自动生成的 `GITHUB_TOKEN` 获取 `contents: write`，不需要配置个人令牌。构建失败不会创建 Release；已存在的 Release 不自动覆盖，避免重跑时改变既有发布成果物。

同一套源码分别编译 Windows、macOS 和 Linux 程序，各平台不能共用同一个二进制文件。macOS 推荐分发 `.app` 应用包；Linux 需要系统图形库和桌面环境。

## 初版功能

| 模块 | 功能 |
| --- | --- |
| 网络配置 | 原生接口读取、缓存即时回填、后台核对；DHCP / 静态 IPv4、掩码、网关；DNS 可留空或指定；单网卡方案与备份；仅写入变化的地址配置或 DNS |
| 图片 / Base64 | PNG、JPEG 与 Base64 双向转换；Data URI 前缀；文件导入或粘贴截图；预览、复制、导出 |
| 网络调试 | 真实网卡和源 IPv4；阿里 / 腾讯 / 谷歌 / 自定义上游 DNS；Ping、Telnet、DNS 查询、Traceroute 四个独立页面；超时、取消、导出 |
| 二维码生成 | 中文文本及网址；尺寸选择；四档纠错等级；PNG 预览和导出 |
| 格式化 / 测试 | JSON、SQL、XML 格式化及可选纠错；JSONPath、正则表达式、XPath 测试；原文保留、修正说明；文件导入、复制和导出 |
| 图片压缩 | 文件导入或粘贴图片；PNG 无损优化；JPEG 质量；可选等比例缩小；大小比较；预览和导出 |

## 网络配置操作

1. 插上设备网线，选择对应有线适配器。程序用 Windows 原生接口读取当前 IP、掩码、网关、DNS 和 DHCP 状态；再次切换时先显示缓存并标注核对状态，后台更新系统实际值。读取不再弹出阻塞窗口，后台结果不会覆盖用户刚输入的参数。
2. 右侧填写设备调试方案。网关可留空；子网掩码可填写 `255.255.255.0` 或前缀 `24`。
3. DNS 可以留空：DHCP 模式下留空表示自动获取 DNS；静态 IP 模式下留空表示清除该网卡的 IPv4 DNS，适合直连设备。也可以选择阿里、腾讯、谷歌预设或填写自定义地址。当前系统 DNS 仍在左侧完整显示；自动获取的 DNS 在方案表单中回填为“留空 / 不指定 DNS”。保存和加载空 DNS 方案时不会自动改为公共 DNS。
4. 此网卡的方案下拉框只显示绑定到当前适配器的配置。点击“保存方案”更新当前方案；“另存新方案”保留参数并清空名称和备注，方便保存不同设备网段。
5. 点击“应用到此网卡”，确认当前值及修改后值。只修改此适配器，不改动其他网卡。
6. “恢复此网卡备份”只恢复当前适配器最近一次应用前的设置。没有匹配备份时拒绝恢复，不会误用其他适配器的备份。

前缀 `24` 对应掩码 `255.255.255.0`，`16` 对应 `255.255.0.0`。初版表单一次配置一个静态 IPv4；应用会替换该网卡的 IPv4 地址和默认网关。备份保留原静态地址列表、默认网关、DHCP 和 DNS 设置；不会改动其他网卡或自定义目的网段路由。

配置绑定适配器 GUID，防止接口索引变化后改错网卡。其他网卡或机器的方案不会混入当前列表；迁移方案需明确绑定到新适配器。配置读取失败或文件损坏时保留原文件，不自动覆盖。

## 切换速度

网卡查询使用 `GetAdaptersAddresses`、`GetIfEntry2Ex` 和原生路由表，常规读取及应用不启动 PowerShell / WMI。应用先通过原生接口捕获备份并比较设置，只使用 `netsh` 写入变化的 IPv4 地址配置或 DNS；相同方案不重复改写，只改 DNS 时不重建 IP。DNS 设置跳过远程可达性验证，地址格式仍在本地校验；恢复和异常回滚保留原有 PowerShell 兼容路径。

本机只读测量：旧当前值读取约 2796 ms，原生读取 20 次平均约 6.44 ms；界面测试中的热缓存回填低于 1 ms，首次控件及字体渲染约 33 ms。该数据是配置读取和控件回填时间，不是网卡完整生效时间，也不包含显示器刷新延迟。DHCP 获取租约、地址冲突检测及设备响应仍可能需要额外时间。界面显示读取时间与设置提交时间，系统修改的端到端耗时需要在可恢复的测试网络中实测。

详细只读对比数据保存在 `dist/network-timing.json`；验证不会修改当前网卡。不同电脑、接口数量及系统负载会影响耗时。

## 上游 DNS 和独立网络工具

| 预设 | IPv4 地址 | 官方说明 |
| --- | --- | --- |
| 阿里 DNS | 223.5.5.5、223.6.6.6 | [阿里公共 DNS](https://www.alibabacloud.com/help/zh/dns/free-service-disclaimer) |
| 腾讯 DNS | 119.29.29.29 | [DNSPod 接入指南](https://docs.dnspod.com/public-dns/public-dns-guide-zh/) |
| 谷歌 DNS | 8.8.8.8、8.8.4.4 | [Google Public DNS](https://developers.google.com/speed/public-dns) |

网络调试共用出口网卡、源 IP 和固定上游 DNS；该设置独立于网卡配置的可选 DNS，域名调试仍须指定上游。具体参数分开：Ping 仅显示次数和超时，Telnet 显示端口，DNS 显示记录类型，Traceroute 显示最大跳数和每跳探测次数。多个上游按顺序尝试；超时、SERVFAIL、REFUSED 时尝试下一个，不回退系统 DNS。设备直连没有外网时，直接输入设备 IP 即可，无需请求公共 DNS。

Windows Traceroute 使用绑定源地址的 ICMP TTL 探测，逐跳显示结果；Linux 需要安装 `traceroute`，macOS 使用系统命令。Traceroute 最长 180 秒，其余网络任务最长 45 秒，均可取消。

## 粘贴图片

复制截图或图片内容后，在图片转换或压缩页面点击“粘贴图片”；也可点击图片区域后按 `Ctrl+V`，macOS 使用 `⌘V`。文本框中的粘贴仍按文本处理。复制文件路径或资源管理器中的文件图标不等同于复制图片内容，请使用文件导入。剪贴板读取失败会显示原因，不会清空用户剪贴板。

## 绕过 TUN 的实现与边界

- Windows：通过系统属性筛选物理网卡；TCP 和 DNS 在套接字上设置 `IP_UNICAST_IF`，同时绑定源地址。
- Linux：TCP 和 DNS 使用 `SO_BINDTODEVICE`；可能需要 root 或 `CAP_NET_RAW`；Ping 使用 `ping -I` 指定接口。
- macOS：TCP 和 DNS 使用 `IP_BOUND_IF`；Ping 绑定源地址并核验当前路由。
- Windows ICMP 使用 `IcmpSendEcho2Ex` 指定源地址，并在发送前核验该源地址的实际路由；路由不符合所选接口时停止并说明原因。
- 域名解析通过用户指定的 DNS 服务器及所选网卡发送，避免使用系统中可能被 TUN 接管的解析器。
- 不会自动添加、删除或修改系统路由。VPN 防火墙、透明 DNS 重定向或路由在操作期间变化等情况仍可能阻止直连；结果中的出口信息只说明本程序选择和约束的接口，不能证明 VPN 之外的所有系统拦截均已绕过。
- 初版网络调试出口支持 IPv4；DNS 可查询 AAAA 记录。TCP / Telnet 功能是端口连通性检测，不提供交互式 Telnet 终端。

## 文本工具说明

- JSON 格式化直接处理原文本，保留大整数和数值表示，不通过浮点反序列化重建文本。
- JSONPath 示例：`$.items[*].name`、`$.items[?(@.price > 3)].price`。
- 正则采用 Go RE2 语法，支持命名捕获组，不支持前后查找、反向引用等 PCRE 扩展。匹配位置使用 UTF-8 字节偏移，最多显示 1000 条。
- XPath 初版支持节点选择，如 `//item/title`、`//item[@id='1']`；不提供标量函数结果界面。
- SQL 使用保留字面量及注释的词法排版，不是 SQL 语法检查器；不同数据库方言的复杂过程体可能无法按预期排版。
- “尝试纠错”默认关闭，开启后保留左侧原文并列出修正说明：JSON 支持单引号、未加引号键名、注释、末尾逗号、Python 布尔值及 None；XML 修复未转义的 `&`，保留 CDATA 和注释；SQL 修复全角标点且不改写字符串与注释。支持去除 BOM 和 Markdown 围栏。不猜测缺失的字符串、逗号、括号或 XML 标签；含 DOCTYPE 的错误 XML 不自动改写实体。
- 文本输入最多 4 MiB；图片文件最多 32 MiB、2400 万像素，防止超大输入占用过多内存。
- PNG 不缩放时保持像素；重编码可能去掉元数据。如果无损优化没有缩小文件则保留原文件。JPEG 为有损压缩，透明背景合成为白色；初版不按 EXIF 方向自动旋转，也不保留原图片元数据。

## 配置位置

程序使用操作系统用户配置目录：

- Windows：`%AppData%\DevOpsTool\profiles.json`
- macOS：`~/Library/Application Support/DevOpsTool/profiles.json`
- Linux：`$XDG_CONFIG_HOME/DevOpsTool/profiles.json`，默认 `~/.config/DevOpsTool/profiles.json`

Windows 网卡备份保存在上述 `DevOpsTool/backups/` 下；时间戳文件留存历史，`<适配器标识摘要>.last.json` 保存该网卡最近备份。旧版 `last-backup.json` 仅在标识匹配所选网卡时用于兼容恢复。

## 源码构建

需要 Go 1.24 或以上版本；桌面构建需要 C 编译器。

### Windows

安装 Go 和 MinGW-w64，或使用项目内便携编译器。可选运行 `python scripts/bootstrap.py` 下载工具到 `.tools/`；下载 Go 时验证官方 SHA256，C 编译器使用发布资产提供的 SHA256（如有）。不会安装到系统目录或永久修改 PATH。

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -Test
```

输出：`dist/DevOpsTool.exe`。构建使用静态链接选项，发布前检查动态库导入，确保仅依赖 Windows 系统 DLL。

### Linux

以 Debian/Ubuntu 为例，先安装 Go、`gcc`、`pkg-config`、`libgl1-mesa-dev`、`xorg-dev`、`libx11-dev`、`traceroute`；其他发行版使用对应包。当前剪贴板依赖 X11 或 XWayland；无法访问时会提示，不影响文件导入。

```sh
go test ./...
go build -trimpath -ldflags '-s -w' -o dist/DevOpsTool ./cmd/devopstool
```

### macOS

安装 Go 和 Xcode Command Line Tools：

```sh
xcode-select --install
go test ./...
go build -trimpath -ldflags '-s -w' -o dist/DevOpsTool ./cmd/devopstool
go install fyne.io/tools/cmd/fyne@latest
cd cmd/devopstool
fyne package -os darwin -name DevOpsTool -icon ../../internal/ui/assets/app.png -appID cn.azukin.devopstool
```

使用 [Fyne 官方构建说明](https://docs.fyne.io/started/packaging/) 配置签名和安装包。Linux 和 macOS 的完整桌面运行需要在对应系统验证，Windows 上的跨平台核心编译检查不能替代实际运行测试。

## 项目结构

```text
cmd/devopstool/     桌面程序入口
internal/ui/       中文界面、主题、任务调度、嵌入字体
internal/tools/    文本、图片及二维码工具逻辑
internal/network/ 跨平台网络调试及 Windows 网卡操作
internal/config/  配置校验、持久化
scripts/           便携工具准备与 Windows 构建
dist/              编译输出和渲染预览（不纳入版本管理）
```

## 验证

```powershell
# 全量测试及 Windows 单文件构建
powershell -ExecutionPolicy Bypass -File scripts/build.ps1 -Test

# 生成软件渲染器界面预览
$env:DEVOPSTOOL_RENDER = '1'
go test ./internal/ui -run TestDesktopRendering -count=1
```

自动测试覆盖配置持久化及损坏保护、网卡当前值回填、单网卡方案隔离、独立网络页面、上游 DNS 校验、格式化纠错、图片粘贴按钮及快捷键、Base64 往返、PNG 像素保真、JPEG 缩放及透明背景、二维码、表达式查询、SQL 字面量及运算符保留、套接字出口绑定、PowerShell 语法、模拟切换及失败恢复、六个模块的界面渲染。已额外完成真实网卡只读查询，以及物理出口上的本机 Ping、TCP、Traceroute 和新版原生界面启动验证。测试未改写当前网卡或用户剪贴板。网卡真实切换和 VPN 环境对外直连需在可恢复的测试网络中进一步验收。详细记录见 [VALIDATION.md](VALIDATION.md)。

## 第三方资源

中文字体来自 [Google Fonts 的 Noto Sans SC](https://github.com/google/fonts/tree/main/ofl/notosanssc)，采用 SIL Open Font License；许可证随源码保留在 `internal/ui/assets/OFL.txt`。

Go 第三方依赖由 `go.mod` 和 `go.sum` 固定。Fyne、二维码、JSONPath、XPath 等组件的许可证说明见 [THIRD_PARTY.md](THIRD_PARTY.md)。
