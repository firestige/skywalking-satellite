package utils

import "golang.org/x/net/bpf"

// Protocol constants
const (
	ProtocolTCP  = 6
	ProtocolUDP  = 17
	ProtocolICMP = 1
	EthTypeIPv4  = 0x0800
	EthTypeIPv6  = 0x86DD
	EthTypeARP   = 0x0806
)

// Common port constants
const (
	PortHTTP   = 80
	PortHTTPS  = 443
	PortSSH    = 22
	PortFTP    = 21
	PortTelnet = 23
	PortSMTP   = 25
	PortDNS    = 53
	PortDHCP   = 67
	PortTFTP   = 69
	PortPOP3   = 110
	PortNTP    = 123
	PortSNMP   = 161
	PortLDAP   = 389
)

type filterBuilder struct {
	instructions []bpf.Instruction
}

func NewFilterBuilder() *filterBuilder {
	return &filterBuilder{
		instructions: make([]bpf.Instruction, 0),
	}
}

// +2 commands to build a BPF filter
func (b *filterBuilder) EtherType(ethType uint32, skipOnFail uint8) *filterBuilder {
	b.instructions = append(b.instructions,
		bpf.LoadAbsolute{Off: 12, Size: 2}, // Load the EtherType field (offset 12, size 2 bytes)
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: ethType, SkipTrue: skipOnFail},
	)
	return b
}

// +2 commands to build a BPF filter
func (b *filterBuilder) IPv4(skipOnFail uint8) *filterBuilder {
	return b.EtherType(EthTypeIPv4, skipOnFail)
}

// +2 commands to build a BPF filter
func (b *filterBuilder) IPv6(skipOnFail uint8) *filterBuilder {
	return b.EtherType(EthTypeIPv6, skipOnFail)
}

// +2 commands to build a BPF filter
func (b *filterBuilder) IPProtocol(protocol uint32, skipOnFail uint8) *filterBuilder {
	b.instructions = append(b.instructions,
		bpf.LoadAbsolute{Off: 23, Size: 1}, // Load the protocol field (offset 23, size 1 byte)
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: protocol, SkipTrue: skipOnFail},
	)
	return b
}

// +2 commands to build a BPF filter
func (b *filterBuilder) TCP(skipOnFail uint8) *filterBuilder {
	return b.IPProtocol(ProtocolTCP, skipOnFail)
}

// +2 commands to build a BPF filter
func (b *filterBuilder) UDP(skipOnFail uint8) *filterBuilder {
	return b.IPProtocol(ProtocolUDP, skipOnFail)
}

// +2 commands to build a BPF filter
func (b *filterBuilder) SrcPort(port uint32, skipOnFail uint8) *filterBuilder {
	b.instructions = append(b.instructions,
		bpf.LoadAbsolute{Off: 20, Size: 2}, // Load the source port field (offset 34, size 2 bytes)
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: port, SkipTrue: skipOnFail},
	)
	return b
}

// +2 commands to build a BPF filter
func (b *filterBuilder) DstPort(port uint32, skipOnFail uint8) *filterBuilder {
	b.instructions = append(b.instructions,
		bpf.LoadAbsolute{Off: 22, Size: 2}, // Load the destination port field (offset 36, size 2 bytes)
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: port, SkipTrue: skipOnFail},
	)
	return b
}

// +4 commands to build a BPF filter
func (b *filterBuilder) Port(port uint32, skipOnSuccess uint8) *filterBuilder {
	b.instructions = append(b.instructions,
		bpf.LoadAbsolute{Off: 20, Size: 2}, // Load the source port field (offset 34, size 2 bytes)
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: port, SkipTrue: skipOnSuccess},
		bpf.LoadAbsolute{Off: 22, Size: 2}, // Load the destination port field (offset 36, size 2 bytes)
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: port, SkipTrue: skipOnSuccess - 2},
	)
	return b
}

// +1 commands to build a BPF filter
func (b *filterBuilder) Accept() *filterBuilder {
	b.instructions = append(b.instructions,
		bpf.RetConstant{Val: 0xFFFFFFFF}, // Accept the packet
	)
	return b
}

// +1 commands to build a BPF filter
func (b *filterBuilder) Drop() *filterBuilder {
	b.instructions = append(b.instructions,
		bpf.RetConstant{Val: 0x00000000}, // Drop the packet
	)
	return b
}

func (b *filterBuilder) Compile() ([]bpf.RawInstruction, error) {
	if len(b.instructions) == 0 {
		return nil, nil // No filter instructions
	}

	// Compile the filter instructions
	filter, err := bpf.Assemble(b.instructions)
	if err != nil {
		return nil, err
	}

	return filter, nil
}

type PrebuildFilter struct{}

func (f *PrebuildFilter) SIP(port uint32) ([]bpf.RawInstruction, error) {
	return NewFilterBuilder().
		IPv4(15).      // 0-1: 如果不是IPv4，跳过15条指令到Drop（指令16）
		TCP(6).        // 2-3: 如果不是TCP，跳过6条指令到UDP检查（指令9）
		Port(port, 2). // 4-7: 如果是指定端口，跳过2条指令到Accept（指令8）
		Accept().      // 8: 接受TCP SIP包
		UDP(6).        // 9-10: 如果不是UDP，跳过6条指令到Drop（指令16）
		Port(port, 2). // 11-14: 如果是指定端口，跳过2条指令到Accept（指令15）
		Accept().      // 15: 接受UDP SIP包
		Drop().        // 16: 丢弃其他包
		Compile()
}
