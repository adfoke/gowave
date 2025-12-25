# Gowave

Gowave 是一个用 Go 语言编写的简单命令行音频工具，支持播放和录制 WAV 格式的音频文件。它具有基于 TUI（终端用户界面）的播放进度展示。

## 功能特性

- **音频播放**：支持播放 WAV 格式音频，并带有实时进度条。
- **音频录制**：支持从麦克风录制音频并保存为 WAV 文件。
- **终端界面**：使用 Bubble Tea 构建的精美 TUI 界面。
- **跨平台**：支持 Windows, macOS 和 Linux。

## 安装

确保你已经安装了 Go 环境 (1.21+)，然后克隆仓库并编译：

```bash
git clone https://github.com/adfoke/gowave.git
cd gowave
go build -o gowave
```

### 依赖项

在 Linux 上，你可能需要安装 `alsa` 开发库：
- Ubuntu/Debian: `sudo apt-get install libasound2-dev`
- CentOS/Fedora: `sudo dnf install alsa-lib-devel`

## 使用方法

### 播放音频

你可以直接运行程序并传入 WAV 文件路径，或者使用 `play` 子命令：

```bash
# 简写
./gowave music.wav

# 使用 play 命令
./gowave play music.wav
```

**播放快捷键：**
- `[Space]`：暂停/恢复播放
- `[q]` 或 `[Esc]`：退出播放器

### 录制音频

使用 `record` 子命令开始录音：

```bash
./gowave record output.wav
```

**录音说明：**
- 程序会开始从默认麦克风捕获音频。
- 按 `Ctrl+C` 停止录音并保存文件。

## 项目结构

- `main.go`: 程序入口，包含播放器逻辑和 TUI 实现。
- `record.go`: 录音模块实现。

## 技术栈

- [Bubble Tea](https://github.com/charmbracelet/bubbletea): TUI 框架。
- [Beep](https://github.com/faiface/beep): 音频播放库。
- [Malgo](https://github.com/gen2brain/malgo): 跨平台音频输入/输出库。
- [go-audio/wav](https://github.com/go-audio/wav): WAV 编码处理。

## 许可证

[MIT License](LICENSE)
