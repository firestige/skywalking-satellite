# 数据包处理流水线架构设计文档

## 1. 架构概述

本设计实现了一个清晰、可扩展的数据包处理流水线，将网络数据包的捕获、处理和分发解耦为独立的组件。整个架构遵循单一职责原则，每个组件都有明确的职责边界。

## 2. 核心组件

### 2.1 DataSource (数据源)
- **职责**: 负责从网络接口捕获原始数据包
- **实现**: `networkCapture` 使用 AF_PACKET 进行高性能数据包捕获
- **特点**: 
  - 支持 BPF 过滤器
  - 可配置缓冲区大小
  - 生命周期管理 (Prepare/Start/Close)

### 2.2 StreamProcessor (流处理器)
- **职责**: 协调数据流的处理，管理过滤器、映射器和聚合器链
- **特点**:
  - 支持链式配置 (WithFilter/WithMapper/WithAggregator/WithSink)
  - 多工作协程并发处理
  - 批处理支持，提高处理效率
  - 内置 TCP 流重组功能
  - 背压控制，避免内存溢出

### 2.3 HandlerManager (处理器管理器)
- **职责**: 管理各种协议的处理器，支持动态注册和卸载
- **特点**:
  - 线程安全的处理器注册/注销
  - 支持协议自动识别
  - 扩展性强，易于添加新协议支持

### 2.4 Dispatcher (分发器)
- **职责**: 将处理后的数据分发给合适的处理器
- **特点**:
  - 支持多处理器并发处理
  - 内置重试机制
  - 统计信息收集
  - 错误处理和恢复

### 2.5 FrameHandler (帧处理器)
- **职责**: 处理特定协议的数据帧
- **特点**:
  - 协议特定的解析逻辑
  - 支持协议自动识别
  - 可插拔设计

## 3. 数据流图

```
NetworkCapture -> StreamProcessor -> Dispatcher -> FrameHandler
     |               |                    |             |
     |               |                    |             |
 [AF_PACKET]    [Filter Chain]      [HandlerManager] [SIP/HTTP/...]
     |               |                    |             |
     |               |                    |             |
 [BPF Filter]   [Mapper Chain]      [Retry Logic]  [Protocol Parse]
     |               |                    |             |
     |               |                    |             |
 [Raw Packets]  [Aggregator Chain]  [Load Balance]  [Business Logic]
```

## 4. 处理流程

### 4.1 数据捕获阶段
1. NetworkCapture 使用 AF_PACKET 从网络接口捕获原始数据包
2. 应用 BPF 过滤器进行初步过滤
3. 将 gopacket.Packet 转换为 RawFrameData 格式

### 4.2 流处理阶段
1. **过滤器链**: 按注册顺序应用过滤器，移除不需要的数据包
2. **映射器链**: 按注册顺序应用映射器，添加元数据或转换数据格式
3. **聚合器链**: 进行 TCP 流重组，将分片的数据包合并
4. **批处理**: 收集多个数据包进行批量处理，提高效率

### 4.3 分发处理阶段
1. Dispatcher 接收处理后的数据包
2. 查询 HandlerManager 获取所有注册的处理器
3. 对每个支持当前协议的处理器进行处理
4. 实现重试机制和错误恢复

## 5. 架构优势

### 5.1 清晰性
- **单一职责**: 每个组件都有明确的职责边界
- **分层设计**: 从数据捕获到业务处理，层次清晰
- **接口抽象**: 使用接口定义组件间的交互，降低耦合

### 5.2 可扩展性
- **插件化**: 处理器可以动态注册和卸载
- **链式配置**: 过滤器、映射器和聚合器可以灵活组合
- **协议无关**: 框架不依赖特定协议，易于扩展新协议

### 5.3 性能
- **多工作协程**: 并发处理提高吞吐量
- **批处理**: 减少系统调用和上下文切换
- **背压控制**: 避免内存溢出和系统过载
- **高效数据结构**: 使用 channel 和 sync.Pool 优化内存使用

### 5.4 健壮性
- **错误隔离**: 单个处理器的错误不会影响其他处理器
- **重试机制**: 支持可配置的重试策略
- **生命周期管理**: 统一的启动和关闭流程
- **资源清理**: 确保资源正确释放

## 6. 配置示例

```go
// 创建处理管道
pipeline, err := NewPipelineBuilder().
    WithInterface("eth0").
    WithBufferSize(2048).
    WithWorkerCount(8).
    WithBatchSize(200).
    WithFlushInterval(50 * time.Millisecond).
    Build()

// 启动管道
pipeline.Start()

// 运行时获取统计信息
stats := pipeline.GetStats()
```

## 7. 扩展点

### 7.1 新协议支持
```go
// 实现新的协议处理器
type customProtocolHandler struct {
    name string
}

func (h *customProtocolHandler) Handle(ctx context.Context, frame types.RawFrameData) error {
    // 协议特定的处理逻辑
    return nil
}

func (h *customProtocolHandler) IsSupported(frame types.RawFrameData) bool {
    // 协议识别逻辑
    return true
}

// 注册到处理器管理器
handlerManager.RegisterHandler(customProtocolHandler)
```

### 7.2 自定义过滤器
```go
// 创建自定义过滤器
customFilter := func(frame types.RawFrameData) bool {
    // 自定义过滤逻辑
    return shouldProcess(frame)
}

// 添加到处理链
streamProcessor.WithFilter(customFilter)
```

### 7.3 自定义聚合器
```go
// 创建自定义聚合器
customAggregator := func(frames []types.RawFrameData) ([]types.RawFrameData, error) {
    // 自定义聚合逻辑
    return aggregateFrames(frames), nil
}

// 添加到处理链
streamProcessor.WithAggregator(customAggregator)
```

## 8. 性能考虑

### 8.1 内存管理
- 使用对象池减少 GC 压力
- 及时释放不再使用的资源
- 避免内存泄漏

### 8.2 并发控制
- 使用 context 进行取消信号传播
- 合理设置工作协程数量
- 避免协程泄漏

### 8.3 I/O 优化
- 使用 AF_PACKET 进行高性能数据包捕获
- 批处理减少系统调用
- 异步处理避免阻塞

## 9. 监控和调试

### 9.1 统计信息
- 处理的数据包数量
- 错误数量和类型
- 丢弃的数据包数量
- 各个处理器的性能指标

### 9.2 日志记录
- 结构化日志记录
- 可配置的日志级别
- 性能相关的日志

### 9.3 调试支持
- 支持运行时状态查询
- 提供调试接口
- 支持性能分析

## 10. 总结

该架构设计具有以下特点：

1. **清晰简单**: 组件职责明确，接口设计简洁
2. **高可扩展**: 支持插件化扩展，易于添加新功能
3. **高性能**: 多协程并发处理，批处理优化
4. **健壮稳定**: 错误隔离，重试机制，资源管理
5. **易于维护**: 代码结构清晰，便于理解和维护

这个设计为网络数据包处理提供了一个solid的基础架构，可以满足各种复杂的业务需求。
