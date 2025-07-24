package main

import (
	"context"
	"fmt"
	"time"

	"github.com/apache/skywalking-satellite/plugins/server/local/packet/capture"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/processor"
)

// 完整的TCP流重组集成示例
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 创建capture层
	captureBuilder := capture.NewNetworkCaptureBuilder(ctx).
		WithInterface("eth0").
		WithFilter("tcp port 80 or tcp port 443 or tcp port 9090").
		WithWorkerCount(4).
		WithRingSize(1024)

	dataSource, err := captureBuilder.Build()
	if err != nil {
		panic(fmt.Sprintf("Failed to create capture: %v", err))
	}

	// 2. 创建应用层处理器
	appProcessor := processor.NewApplicationProcessor(ctx)

	// 3. 启动capture
	if err := dataSource.Prepare(); err != nil {
		panic(fmt.Sprintf("Failed to prepare capture: %v", err))
	}

	if err := dataSource.Start(); err != nil {
		panic(fmt.Sprintf("Failed to start capture: %v", err))
	}

	// 4. 启动数据处理流水线
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 从capture层获取传输层帧
				frame, err := dataSource.Fetch()
				if err != nil {
					continue
				}

				// 传递给应用层处理器进行TCP流重组
				appProcessor.ProcessFrameFromCapture(frame)
			}
		}
	}()

	// 5. 处理重组后的完整消息
	go func() {
		messageChan := appProcessor.GetOutputChannel()
		for {
			select {
			case <-ctx.Done():
				return
			case message := <-messageChan:
				handleCompleteMessage(message)
			}
		}
	}()

	// 6. 运行一段时间后退出
	fmt.Println("TCP流重组器运行中...")
	time.Sleep(30 * time.Second)

	fmt.Println("关闭中...")
	dataSource.Close()
	appProcessor.Close()
}

// handleCompleteMessage 处理完整的应用层消息
func handleCompleteMessage(message *processor.CompleteMessage) {
	fmt.Printf("收到完整%s消息:\n", message.Protocol)
	fmt.Printf("  连接: %s:%d -> %s:%d\n",
		message.Connection.SrcHost, message.Connection.SrcPort,
		message.Connection.DestHost, message.Connection.DstPort)
	fmt.Printf("  方向: %s\n", message.Direction)
	fmt.Printf("  大小: %d bytes\n", len(message.Data))
	fmt.Printf("  时间: %d\n", message.Timestamp)

	// 根据协议类型进行特殊处理
	switch message.Protocol {
	case "HTTP":
		handleHTTPMessage(message)
	case "gRPC":
		handleGRPCMessage(message)
	}

	fmt.Println("---")
}

func handleHTTPMessage(message *processor.CompleteMessage) {
	// HTTP消息处理
	data := string(message.Data)

	if len(data) > 100 {
		fmt.Printf("  HTTP内容: %s...\n", data[:100])
	} else {
		fmt.Printf("  HTTP内容: %s\n", data)
	}

	// 这里可以进一步解析HTTP头、URL、方法等
}

func handleGRPCMessage(message *processor.CompleteMessage) {
	// gRPC消息处理
	fmt.Printf("  gRPC帧长度: %d\n", len(message.Data))

	if len(message.Data) >= 5 {
		compressed := message.Data[0]
		length := int(message.Data[1])<<24 | int(message.Data[2])<<16 |
			int(message.Data[3])<<8 | int(message.Data[4])

		fmt.Printf("  压缩标志: %d\n", compressed)
		fmt.Printf("  消息长度: %d\n", length)
	}

	// 这里可以进一步解析protobuf消息
}
