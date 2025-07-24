# Capture包重构总结

## 重构目标
1. 解决对象池泄漏和状态不一致的风险
2. 消除PacketInfo和RawFrameData的重复
3. 统一双管道为单管道架构
4. 明确传输层vs应用层的职责边界
5. 消除命名混淆(decoder/parser/assembler)

## 重构成果

### 1. 文件架构重组

**删除的文件:**
- `workers.go` - 功能合并到frame_pipeline.go

**新增的文件:**
- `transport_parser.go` - 统一的传输层解析器（替代packet_parser和decoder）
- `frame_builder.go` - 帧数据构建器
- `frame_pipeline.go` - 统一的帧处理流水线
- `types/transport_frame.go` - 统一的传输层帧结构

**修改的文件:**
- `capture.go` - 主结构体简化，使用单管道
- `capture_loop.go` - 简化为直接创建TransportFrame
- `pool_manager.go` - 统一为TransportFrame池
- `handle.go` - 清理方法简化
- `utils/bounded_pool.go` - 独立的有界对象池实现

### 2. 数据结构统一

**旧架构:**
```
Raw Packet → PacketInfo → RawFrameData
    ↓           ↓            ↓
packetChan → processing → frameChan
```

**新架构:**
```
Raw Packet → TransportFrame (state-driven)
    ↓              ↓
framePipeline → processing → framePipeline
```

**TransportFrame状态流转:**
- `StateRaw` → `StateParsed` → `StateReady`
- TCP流: `StateRaw` → `StateParsed` → `StateReassembled` → `StateReady`

### 3. 核心改进

**对象池优化:**
- 统一使用`TransportFrame`池，消除了PacketInfo池
- 有界池设计，防止内存无限增长
- 完整的状态重置，包括时间戳和Meta清理
- 详细的统计信息追踪

**架构简化:**
- 单管道设计：只有`framePipeline`
- 状态驱动处理：通过`FrameState`控制流程
- 减少数据拷贝：直接复用TransportFrame

**职责明确:**
- `TransportParser`: 只处理L2-L4层解析
- `FrameBuilder`: 构建传输层载荷帧
- 应用层协议解析交给后续处理器

### 4. 命名优化

**消除歧义:**
- `TransportType` 替代 `ProtocolType`
- `TransportParser` 替代 `PacketParser`/`Decoder`
- `TCPStreamReassembler` 替代 `TCPAssembler`（暂时简化移除）

**清晰边界:**
- Transport Layer (L2-L4): capture包负责
- Application Layer (L7): 后续处理器负责

### 5. 内存安全

**对象池改进:**
- 池大小限制：基于worker数量和通道大小计算
- 安全重置：完整清理所有字段
- 统计监控：命中率、未命中、丢弃等指标

**资源管理:**
- 统一的资源清理流程
- 正确的通道关闭顺序
- 池状态统计输出

## 性能优势

1. **内存效率**: 单一数据结构，减少分配和GC压力
2. **处理效率**: 状态驱动，减少条件判断
3. **并发安全**: 有界池设计，防止内存爆炸
4. **可观测性**: 详细的池统计信息

## 代码统计

**重构前:**
- 文件数: ~10个
- 总行数: ~600行/文件
- 数据结构: PacketInfo + RawFrameData
- 管道: packetChan + frameChan

**重构后:**
- 文件数: 8个核心文件
- 总行数: ~100-150行/文件
- 数据结构: TransportFrame (统一)
- 管道: framePipeline (单一)

每个文件职责单一，易于维护和测试。

## 扩展性

新架构易于扩展：
- 添加新传输层协议：只需在TransportParser中添加
- 添加新处理状态：扩展FrameState枚举
- 优化对象池：调整池大小计算逻辑
- 监控增强：扩展PoolStats统计项

## 兼容性

保持了对外接口的兼容性：
- `Fetch()` 方法依然返回 `*RawFrameData`
- 通过 `ToRawFrameData()` 方法提供转换
- 核心功能保持不变，只是内部实现优化
