package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/signal"
	"syscall"


	"github.com/gen2brain/malgo"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

// 配置参数
const (
	SampleRate  = 44100
	Channels    = 1
	BitDepth    = 16
	OutputName  = "output.wav"
)

func runRecord(outputName string) {
	// 1. 初始化 Malgo 上下文
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	// 2. 创建一个 Channel 用于传输音频数据
	// 缓冲区设置大一点，防止写入文件太慢导致数据丢失
	audioChan := make(chan []byte, 1024)

	// 3. 配置麦克风参数
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16 // 16位深度
	deviceConfig.Capture.Channels = Channels
	deviceConfig.SampleRate = SampleRate
	deviceConfig.Alsa.NoMMap = 1 // Linux 特有修正，防止某些驱动报错

	// 4. 定义回调函数：当麦克风有声音进来时调用
	deviceCallbacks := malgo.DeviceCallbacks{
		Data: func(pOutputSample, pInputSamples []byte, framecount uint32) {
			// 注意：这里必须拷贝数据，因为 pInputSamples 在函数结束后会被底层复用
			dataCopy := make([]byte, len(pInputSamples))
			copy(dataCopy, pInputSamples)
			
			// 将数据发送到管道，如果在处理不过来则丢弃（非阻塞），防止卡死硬件
			select {
			case audioChan <- dataCopy:
			default:
				// Channel 满了，丢弃这一帧（通常不会发生，除非磁盘太慢）
			}
		},
	}

	// 5. 初始化设备
	device, err := malgo.InitDevice(ctx.Context, deviceConfig, deviceCallbacks)
	if err != nil {
		panic(err)
	}

	// 6. 准备 WAV 文件写入
	outFile, err := os.Create(outputName)
	if err != nil {
		panic(err)
	}
	defer outFile.Close()

	// 创建 WAV 编码器
	// 参数: writer, sampleRate, bitDepth, numChans, audioFormat(1=PCM)
	encoder := wav.NewEncoder(outFile, SampleRate, BitDepth, Channels, 1)

	// 7. 启动录音设备
	if err := device.Start(); err != nil {
		panic(err)
	}

	fmt.Printf("正在录音... 请说话 (按 Ctrl+C 停止并保存)\n")

	// 8. 启动一个协程处理 Ctrl+C 信号，确保文件正确关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 9. 主循环：从 Channel 读取数据并编码写入文件
	// 这里使用 label 跳出循环
Loop:
	for {
		select {
		case <-sigChan:
			fmt.Println("\n检测到停止信号，正在保存文件...")
			break Loop // 跳出 for 循环

		case data := <-audioChan:
			// malgo 给的是 []byte，wav 库需要 IntBuffer
			// 我们需要把 []byte (Little Endian) 转成 int
			intData := make([]int, len(data)/2)
			for i := 0; i < len(intData); i++ {
				// 将两个 byte 转成一个 int16，再转成 int
				val := int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
				intData[i] = int(val)
			}

			// 写入 WAV 编码器
			buf := &audio.IntBuffer{
				Format: &audio.Format{
					SampleRate:  SampleRate,
					NumChannels: Channels,
				},
				Data:           intData,
				SourceBitDepth: BitDepth,
			}
			if err := encoder.Write(buf); err != nil {
				fmt.Println("写入错误:", err)
			}
		}
	}

	// 10. 收尾工作
	device.Uninit()
	
	//以此关闭编码器至关重要！它会回过头去修改文件头的“文件大小”字段
	if err := encoder.Close(); err != nil {
		fmt.Println("关闭 WAV 编码器失败:", err)
	}
	
	fmt.Println("录音完成！已保存为", outputName)
}
