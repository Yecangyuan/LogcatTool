# LogCaTool

终端版 Android Logcat 查看器，基于 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 构建。

![截图](./pic/pic.png)

## 功能

- **实时日志流** — 通过 ADB 实时查看 Android 设备日志
- **日志文件读取** — 支持从已保存的 logcat 文件读取（离线模式）
- **颜色编码** — 不同日志级别使用不同颜色显示
- **搜索与过滤** — 支持文本/正则搜索、Tag/PID/包名过滤
- **日志级别切换** — 快速切换 V/D/I/W/E/F 级别显示
- **多设备支持** — 连接多设备时可选择目标设备
- **暂停/恢复** — 暂停日志流而不丢失新日志
- **书签** — 标记重要日志行，快速跳转
- **导出日志** — 将过滤后的日志导出到文件
- **自动滚动** — 可切换的自动滚动到底部
- **Vim 风格导航** — j/k/G/g 快速浏览

## 安装

```bash
go install github.com/simley/logcatool@latest
```

或从源码构建：
```bash
git clone https://github.com/simley/logcatool.git
cd logcatool
go build -o logcatool .
```

## 使用

### 实时查看设备日志
```bash
./logcatool
```

### 指定设备
```bash
./logcatool -s <设备序列号>
```

### 读取日志文件
```bash
./logcatool -f logcat.txt
```

### 参数说明
| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-f` | 从日志文件读取 | - |
| `-s` | 指定设备序列号 | 自动选择 |
| `-n` | 日志缓冲区大小 | 50000 |
| `-v` | 显示版本信息 | - |
| `--debug` | 开启调试日志 | - |

## 快捷键

### 导航
| 按键 | 功能 |
|------|------|
| `↑` / `k` | 向上滚动 |
| `↓` / `j` | 向下滚动 |
| `PgUp` / `Ctrl+u` | 上翻页 |
| `PgDn` / `Ctrl+d` | 下翻页 |
| `g` / `Home` | 跳到顶部 |
| `G` / `End` | 跳到底部 |

### 过滤与搜索
| 按键 | 功能 |
|------|------|
| `/` | 搜索（支持正则表达式）|
| `t` | Tag 过滤 |
| `p` | 包名过滤 |
| `i` | PID 过滤 |
| `1`-`6` | 切换日志级别 V/D/I/W/E/F |

### 操作
| 按键 | 功能 |
|------|------|
| `Space` | 暂停/恢复日志流 |
| `c` | 清除日志 |
| `d` | 选择设备 |
| `e` | 导出日志 |
| `b` | 添加/移除书签 |
| `n` / `N` | 下一个/上一个书签 |
| `w` | 切换换行模式 |
| `s` | 切换自动滚动 |
| `?` | 帮助面板 |
| `q` / `Ctrl+c` | 退出 |

## 前置条件

- Go 1.25+
- Android SDK Platform Tools（`adb` 命令可用）
- Android 设备已开启 USB 调试（实时模式）

## 许可证

MIT
