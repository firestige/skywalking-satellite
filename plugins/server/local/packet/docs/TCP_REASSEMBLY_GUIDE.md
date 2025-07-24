# TCP流重组技术方案

## 问题背景

TCP是面向流的协议，应用层的一个完整消息可能被分割成多个TCP段传输。capture层只能获取到单独的TCP段，应用层需要将这些段重组成完整的消息。

## 分层处理架构

```
┌─────────────────────────────────────────────┐
│              应用层处理器                    │
│  ┌─────────────┐  ┌─────────────┐           │
│  │ HTTP重组器  │  │ gRPC重组器  │  ...     │
│  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────┘
                     ↑
              RawFrameData (TCP段)
                     ↑
┌─────────────────────────────────────────────┐
│                Capture层                    │
│  ┌─────────────┐  ┌─────────────┐           │
│  │ 传输层解析  │  │ 帧处理流水  │           │
│  └─────────────┘  └─────────────┘           │
└─────────────────────────────────────────────┘
```

## 核心组件设计

### 1. MessageBoundary 接口

```go
type MessageBoundary interface {
    // 检测消息边界，返回完整消息的长度，-1表示不完整
    DetectBoundary(data []byte) int
    // 获取协议名称
    GetProtocolName() string
}
```

**设计理念**:
- 策略模式：不同协议有不同的边界检测策略
- 可扩展：新协议只需实现此接口
- 高效：检测逻辑尽可能简单快速

### 2. TCPStreamReassembler 重组器

**核心功能**:
- **流管理**: 为每个TCP连接维护独立的缓冲区
- **消息提取**: 使用边界检测器识别完整消息
- **内存管理**: 超时清理、大小限制防止内存泄漏
- **并发安全**: 使用读写锁保护共享状态

**关键数据结构**:
```go
type tcpStream struct {
    connKey     string        // 连接标识
    buffer      *bytes.Buffer // 流缓冲区
    lastUpdate  time.Time     // 最后更新时间
    direction   string        // 数据方向
    connection  types.Connection
}
```

### 3. 协议边界检测器

#### HTTP边界检测
```go
func (h *HTTPBoundaryDetector) DetectBoundary(data []byte) int {
    // 1. 查找HTTP头结束标记 "\r\n\r\n"
    headerEnd := bytes.Index(data, []byte("\r\n\r\n"))
    if headerEnd == -1 {
        return -1 // 头部不完整
    }
    
    // 2. 解析Content-Length
    contentLength := h.extractContentLength(headers)
    
    // 3. 检查总长度是否足够
    totalLength := headerEndPos + contentLength
    if len(data) < totalLength {
        return -1 // 消息体不完整
    }
    
    return totalLength
}
```

#### gRPC边界检测
```go
func (g *gRPCBoundaryDetector) DetectBoundary(data []byte) int {
    if len(data) < 5 {
        return -1 // gRPC帧头不完整
    }
    
    // gRPC帧格式: [1字节压缩标志][4字节长度][消息内容]
    length := int(data[1])<<24 | int(data[2])<<16 | 
              int(data[3])<<8 | int(data[4])
    
    totalLength := 5 + length
    if len(data) < totalLength {
        return -1 // 消息不完整
    }
    
    return totalLength
}
```

## 数据流处理

### 1. 从Capture到Processor

```go
// Capture层输出
type RawFrameData struct {
    Data       []byte         // TCP段数据
    Connection types.Connection // 连接信息
    Direction  string         // 数据方向
    Timestamp  int64          // 时间戳
}

// 应用层输出
type CompleteMessage struct {
    Data       []byte         // 完整消息数据
    Connection types.Connection // 连接信息
    Protocol   string         // 协议类型
    Direction  string         // 数据方向
    Timestamp  int64          // 时间戳
    Meta       map[string]string // 元数据
}
```

### 2. 处理流程

```
TCP段1 → ┐
TCP段2 → ├→ 流缓冲区 → 边界检测 → 完整消息1
TCP段3 → ┘                   ├→ 完整消息2
TCP段4 → ┐                   └→ ...
TCP段5 → ├→ 流缓冲区 → 边界检测 → 完整消息3
...      ┘
```

## 协议检测策略

### 1. 基于端口的初步判断
```go
switch port {
case 80, 8080, 8000, 3000:
    return "HTTP"
case 443, 8443:
    return "HTTPS" // 需要TLS解析
case 9090, 50051:
    return "gRPC"
}
```

### 2. 基于内容的深度检测
```go
// HTTP特征
httpMethods := [][]byte{
    []byte("GET "), []byte("POST "), []byte("PUT "),
    []byte("DELETE "), []byte("HEAD "), []byte("OPTIONS "),
}

// HTTP响应特征
if len(data) >= 4 && string(data[:4]) == "HTTP" {
    return "HTTP"
}
```

## 性能优化策略

### 1. 内存管理
- **缓冲区复用**: 使用对象池避免频繁分配
- **大小限制**: 设置最大流缓冲区大小(1MB)
- **超时清理**: 定期清理不活跃的流(5分钟)

### 2. 并发处理
- **读写锁**: 保护流映射表的并发访问
- **通道缓冲**: 使用缓冲通道减少阻塞
- **协程池**: 限制协程数量避免资源耗尽

### 3. 快速路径
- **协议缓存**: 缓存连接的协议类型
- **早期退出**: 不匹配的协议快速跳过
- **零拷贝**: 尽可能避免数据拷贝

## 扩展性设计

### 1. 新协议支持

添加新协议只需要：
```go
// 1. 实现边界检测器
type MyProtocolBoundaryDetector struct{}

func (m *MyProtocolBoundaryDetector) DetectBoundary(data []byte) int {
    // 协议特定的边界检测逻辑
}

// 2. 注册到处理器
func NewMyProtocolProcessor() *TCPStreamReassembler {
    return NewTCPStreamReassembler(NewMyProtocolBoundaryDetector())
}
```

### 2. 复杂协议处理

对于需要状态机的复杂协议：
```go
type StatefulBoundaryDetector struct {
    state       ProtocolState
    stateData   interface{}
}

func (s *StatefulBoundaryDetector) DetectBoundary(data []byte) int {
    switch s.state {
    case StateWaitingHeader:
        return s.processHeader(data)
    case StateWaitingBody:
        return s.processBody(data)
    }
}
```

## 与Capture层集成

### 1. 数据流向
```
Capture → ApplicationProcessor → Protocol Reassemblers → Complete Messages
```

### 2. 集成代码示例
```go
// 创建处理器
appProcessor := processor.NewApplicationProcessor(ctx)

// 处理来自capture的帧
go func() {
    for {
        frame, err := dataSource.Fetch()
        if err != nil {
            continue
        }
        appProcessor.ProcessFrameFromCapture(frame)
    }
}()

// 处理完整消息
go func() {
    messageChan := appProcessor.GetOutputChannel()
    for message := range messageChan {
        handleCompleteMessage(message)
    }
}()
```

## 错误处理和容错

### 1. 数据损坏处理
- **校验和检查**: 对关键协议进行校验
- **重置机制**: 检测到错误时重置流状态
- **跳过机制**: 跳过无法解析的数据段

### 2. 内存保护
- **流大小限制**: 防止单个流占用过多内存
- **总数限制**: 限制同时处理的流数量
- **降级策略**: 内存不足时暂停新流创建

### 3. 监控和告警
- **统计信息**: 重组成功率、错误率、延迟等
- **性能指标**: 内存使用、CPU使用、吞吐量
- **告警机制**: 异常情况及时通知

## 总结

这个TCP流重组方案的优势：

1. **分层清晰**: Capture专注传输层，Processor处理应用层
2. **协议无关**: 通过接口抽象支持任意协议
3. **高性能**: 内存管理、并发优化、快速路径
4. **可扩展**: 新协议、新功能易于添加
5. **容错性**: 完善的错误处理和资源保护

通过这种设计，我们成功地将TCP流重组从capture层分离出来，让每一层都专注于自己的核心职责，同时保持了高性能和良好的扩展性。
