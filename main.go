package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

// --- 1. 数据模型定义 ---

// audioState 存放音频底层的对象
type audioState struct {
	streamer beep.StreamSeekCloser // 音频流，用于读取数据
	format   beep.Format           // 音频格式信息（采样率等）
	ctrl     *beep.Ctrl            // 控制器，用于实现暂停功能
	duration time.Duration         // 总时长
	done     chan bool             // 播放完成的信号通道
}

// model 是 Bubble Tea 的核心状态存储
type model struct {
	audio    *audioState    // 音频状态
	progress progress.Model // 进度条组件
	filename string         // 文件名
	playing  bool           // UI 显示的播放状态
	pct      float64        // 当前进度百分比 (0.0 - 1.0)
	err      error          // 错误信息
}

// --- 2. 消息定义 ---

// tickMsg 用于定时触发 UI 更新（类似游戏的帧）
type tickMsg time.Time

// tickCmd 是一个指令，告诉 Bubble Tea 每隔 100ms 发送一次 tickMsg
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// --- 3. 初始化逻辑 ---

func initialModel(filename string) (*model, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}

	// 使用 Beep 解码 wav 文件
	streamer, format, err := wav.Decode(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("解码 WAV 失败: %w", err)
	}

	// 初始化扬声器 (只需初始化一次)
	// SampleRate.N(time.Second/10) 决定了缓冲区大小，影响延迟稳定性
	err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	if err != nil {
		streamer.Close()
		return nil, fmt.Errorf("初始化扬声器失败: %w", err)
	}

	// 创建一个可暂停的控制器 (Ctrl)
	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}

	// 播放音频
	// speaker.Play 是异步的，不会阻塞主线程
	done := make(chan bool)
	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		// 播放序列结束后，向 done 通道发送信号
		done <- true
	})))

	// 计算音频总时长
	duration := format.SampleRate.D(streamer.Len())

	// 初始化进度条组件
	prog := progress.New(progress.WithDefaultGradient())

	return &model{
		audio: &audioState{
			streamer: streamer,
			format:   format,
			ctrl:     ctrl,
			duration: duration,
			done:     done,
		},
		progress: prog,
		filename: filename,
		playing:  true,
		pct:      0.0,
	}, nil
}

// Close 释放资源
func (m *model) Close() {
	if m.audio != nil && m.audio.streamer != nil {
		m.audio.streamer.Close()
	}
}

// --- 4. Bubble Tea 核心方法 ---

// Init 在程序启动时调用
func (m model) Init() tea.Cmd {
	return tickCmd() // 开始定时循环
}

// Update 处理消息并更新状态
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// 键盘按键消息
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit // 退出程序

		case " ": // 空格键切换播放/暂停
			m.audio.ctrl.Paused = !m.audio.ctrl.Paused
			m.playing = !m.audio.ctrl.Paused
			return m, nil
		}

	// 定时器消息
	case tickMsg:
		// 1. 检查音频是否播放完毕
		select {
		case <-m.audio.done:
			return m, tea.Quit
		default:
		}

		// 2. 如果暂停中，只需继续维持定时器，不更新进度
		if m.audio.ctrl.Paused {
			return m, tickCmd()
		}

		// 3. 获取当前播放位置
		// 注意：Beep 在另一个 goroutine 运行，访问位置需要加锁
		speaker.Lock()
		position := m.audio.streamer.Position()
		speaker.Unlock()

		// 4. 计算百分比
		length := m.audio.streamer.Len()
		if length > 0 {
			m.pct = float64(position) / float64(length)
		}

		// 5. 更新进度条组件，并请求下一帧
		cmd := m.progress.SetPercent(m.pct)
		return m, tea.Batch(cmd, tickCmd())

	// 进度条组件内部的消息（如窗口大小改变时的重绘）
	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	// 终端窗口大小改变消息
	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - 10 // 让进度条自适应宽度
		if m.progress.Width > 80 {
			m.progress.Width = 80
		}
		return m, nil
	}

	return m, nil
}

// View 渲染界面字符串
func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	// 状态文字
	status := "▶ Playing"
	if !m.playing {
		status = "⏸ Paused " // 加空格为了对齐
	}

	// 格式化时间 (例如 00:15 / 03:40)
	currentPos := m.audio.format.SampleRate.D(m.audio.streamer.Position()).Round(time.Second)
	totalDur := m.audio.duration.Round(time.Second)

	// 界面布局
	s := "\n"
	s += fmt.Sprintf("🎵 File: \033[1m%s\033[0m\n", m.filename) // 粗体显示文件名
	s += fmt.Sprintf("   %s\n\n", status)
	s += "   " + m.progress.View() + "\n\n"
	s += fmt.Sprintf("   ⏱  %v / %v\n\n", currentPos, totalDur)
	s += "   [Space] Play/Pause  [q] Quit\n\n"

	return s
}

// --- 5. 主入口 ---

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "record":
		if len(os.Args) < 3 {
			fmt.Println("错误: 请指定录音保存路径")
			fmt.Println("用法: gowave record <output.wav>")
			os.Exit(1)
		}
		runRecord(os.Args[2])

	case "play":
		if len(os.Args) < 3 {
			fmt.Println("错误: 请指定要播放的文件")
			fmt.Println("用法: gowave play <input.wav>")
			os.Exit(1)
		}
		runPlayer(os.Args[2])

	case "-h", "--help", "help":
		printUsage()

	default:
		// 兼容旧用法: gowave <filename> 默认为播放
		runPlayer(command)
	}
}

func runPlayer(filename string) {
	m, err := initialModel(filename)
	if err != nil {
		fmt.Printf("Error initializing: %v\n", err)
		os.Exit(1)
	}

	// 启动 Bubble Tea 程序
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		m.Close()
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
	m.Close()
}

func printUsage() {
	fmt.Println("Gowave - 一个简单的命令行音频播放与录音工具")
	fmt.Println("\n用法:")
	fmt.Println("  gowave play <file.wav>    播放 WAV 文件")
	fmt.Println("  gowave record <file.wav>  录制音频到 WAV 文件")
	fmt.Println("  gowave <file.wav>         播放 WAV 文件 (简写)")
	fmt.Println("\n快捷键 (播放模式):")
	fmt.Println("  [Space]  暂停/播放")
	fmt.Println("  [q/Esc]  退出")
}
