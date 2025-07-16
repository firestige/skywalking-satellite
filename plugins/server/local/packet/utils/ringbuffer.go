package utils

import (
	"sync/atomic"
	"unsafe"
)

// RingBuffer 无锁环形缓冲区，避免GC压力
type RingBuffer struct {
	buffer []unsafe.Pointer // 存储指针避免GC扫描
	mask   uint64

	// 使用原子操作避免锁竞争
	readPos  uint64
	writePos uint64

	// 预分配的数据块
	dataBlocks [][]byte
	blockSize  int
	blockCount int
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(size int) *RingBuffer {
	// 确保size是2的幂次方
	if size&(size-1) != 0 {
		panic("size must be power of 2")
	}

	blockSize := 64 * 1024 // 64KB per block
	blockCount := size * 2 // 双倍块数量

	rb := &RingBuffer{
		buffer:     make([]unsafe.Pointer, size),
		mask:       uint64(size - 1),
		blockSize:  blockSize,
		blockCount: blockCount,
		dataBlocks: make([][]byte, blockCount),
	}

	// 预分配数据块
	for i := 0; i < blockCount; i++ {
		rb.dataBlocks[i] = make([]byte, blockSize)
	}

	return rb
}

// ZeroCopyWrite 零拷贝写入
func (rb *RingBuffer) ZeroCopyWrite(data []byte) *[]byte {
	pos := atomic.AddUint64(&rb.writePos, 1) - 1
	index := pos & rb.mask

	// 获取预分配的数据块
	blockIndex := int(pos) % rb.blockCount
	block := rb.dataBlocks[blockIndex]

	// 重置块大小并复制数据
	if len(data) > cap(block) {
		// 如果数据超过块大小，重新分配
		block = make([]byte, len(data))
		rb.dataBlocks[blockIndex] = block
	}

	block = block[:len(data)]
	copy(block, data)

	// 存储指针
	atomic.StorePointer(&rb.buffer[index], unsafe.Pointer(&block))

	return &block
}

// ZeroCopyRead 零拷贝读取
func (rb *RingBuffer) ZeroCopyRead() *[]byte {
	readPos := atomic.LoadUint64(&rb.readPos)
	writePos := atomic.LoadUint64(&rb.writePos)

	if readPos >= writePos {
		return nil // 缓冲区为空
	}

	index := readPos & rb.mask
	ptr := atomic.LoadPointer(&rb.buffer[index])

	if ptr == nil {
		return nil
	}

	atomic.AddUint64(&rb.readPos, 1)
	return (*[]byte)(ptr)
}
