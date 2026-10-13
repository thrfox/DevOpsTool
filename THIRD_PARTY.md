# 第三方组件与资源

发布时请一并保留此清单及相应组件的许可证和版权声明；资源许可证也可以从程序“关于”窗口查看。

| 组件 | 上游 | 许可证 |
| --- | --- | --- |
| Fyne | https://github.com/fyne-io/fyne | BSD-3-Clause |
| go-qrcode | https://github.com/skip2/go-qrcode | MIT |
| OjG | https://github.com/ohler55/ojg | MIT |
| xmlquery / xpath | https://github.com/antchfx/xmlquery / https://github.com/antchfx/xpath | MIT |
| etree | https://github.com/beevik/etree | BSD-2-Clause |
| miekg/dns | https://github.com/miekg/dns | BSD-3-Clause |
| 图片剪贴板 | https://github.com/golang-design/clipboard | MIT |
| SVG 渲染 | https://github.com/srwiley/oksvg / https://github.com/srwiley/rasterx | BSD-3-Clause |
| Go 扩展库 | https://cs.opensource.google/go/x | BSD-3-Clause |
| Noto Sans SC | https://github.com/google/fonts/tree/main/ofl/notosanssc | SIL Open Font License 1.1 |

完整组件版本及间接依赖以 `go.mod`、`go.sum` 为准。可执行文件中嵌入了依赖许可证和中文字体许可证，便于单文件分发。

应用和模块图标为本项目原创 SVG，PNG 和 ICO 均由同一矢量源生成。Windows 资源工具 rsrc 仅用于构建，不随发布程序运行。
