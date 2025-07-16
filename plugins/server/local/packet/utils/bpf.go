package utils

import (
	"fmt"

	"golang.org/x/net/bpf"
)

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

// Hardcoded label names for common use cases
const (
	LabelAccept = "accept"
	LabelDrop   = "drop"
)

// Internal label structure for tracking positions
type label struct {
	name     string
	position int
}

// Jump placeholder for deferred jump resolution
type jumpPlaceholder struct {
	instructionIndex int
	targetLabel      string
	isSkipTrue       bool
}

// Option interface provides flexible configuration for BPF filter methods.
// It allows users to customize jump behavior and label creation in a type-safe manner.
//
// Usage patterns:
//   - Simple options: IPv4(OrDrop())
//   - Chain configuration: UDP(WithLabel("udp_check").orDrop())
//   - Multiple options: Port(80, JumpToIfMatch("accept"), JumpToIfNoMatch("drop"))
//
// Common options:
//   - OrDrop(): Jump to drop label if condition fails
//   - OrAccept(): Jump to accept label if condition fails
//   - JumpToIfMatch(label): Jump to label if condition matches
//   - JumpToIfNoMatch(label): Jump to label if condition doesn't match
//   - WithLabel(name): Create a label at current position and configure jumps
type Option interface {
	apply(*optionConfig)
}

// Internal configuration structure for option processing
type optionConfig struct {
	labelName     string
	jumpIfMatch   string
	jumpIfNoMatch string
}

// Label option implementation
type labelOption struct {
	name string
}

func (l labelOption) apply(config *optionConfig) {
	config.labelName = l.name
}

// Match jump option - jumps to target if condition matches
type jumpIfMatchOption struct {
	target string
}

func (j jumpIfMatchOption) apply(config *optionConfig) {
	config.jumpIfMatch = j.target
}

// No-match jump option - jumps to target if condition doesn't match
type jumpIfNoMatchOption struct {
	target string
}

func (j jumpIfNoMatchOption) apply(config *optionConfig) {
	config.jumpIfNoMatch = j.target
}

// Chain configuration structure for fluent API
type chainConfig struct {
	labelName     string
	jumpIfMatch   string
	jumpIfNoMatch string
}

// Chain configuration methods for fluent API
func (c *chainConfig) JumpToIfMatch(target string) *chainConfig {
	c.jumpIfMatch = target
	return c
}

func (c *chainConfig) OrNotMatch(target string) *chainConfig {
	c.jumpIfNoMatch = target
	return c
}

func (c *chainConfig) OrDrop() *chainConfig {
	c.jumpIfNoMatch = LabelDrop
	return c
}

func (c *chainConfig) OrAccept() *chainConfig {
	c.jumpIfNoMatch = LabelAccept
	return c
}

// Option constructor functions

// WithLabel creates a chain configuration starting with a label
func WithLabel(name string) *chainConfig {
	return &chainConfig{labelName: name}
}

// JumpToIfMatch creates an option to jump to target if condition matches
func JumpToIfMatch(target string) Option {
	return jumpIfMatchOption{target: target}
}

// JumpToIfNoMatch creates an option to jump to target if condition doesn't match
func JumpToIfNoMatch(target string) Option {
	return jumpIfNoMatchOption{target: target}
}

// OrDrop creates an option to jump to drop label if condition doesn't match
func OrDrop() Option {
	return jumpIfNoMatchOption{target: LabelDrop}
}

// OrAccept creates an option to jump to accept label if condition doesn't match
func OrAccept() Option {
	return jumpIfNoMatchOption{target: LabelAccept}
}

// BPF Filter Builder provides a fluent API for constructing BPF packet filters.
// It uses automatic jump distance calculation and label resolution to eliminate
// manual jump counting errors.
//
// Basic Usage:
//
//	builder := NewFilterBuilder()
//	filter, err := builder.
//	    IPv4OrDrop().
//	    TCP(JumpToIfNoMatch("check_udp")).
//	    PortOrAccept(80, "check_udp").
//	    Label("check_udp").
//	    UDP(OrDrop()).
//	    PortOrDrop(53).
//	    Accept().
//	    Drop().
//	    Compile()
//
// Advanced Usage with Options:
//
//	filter, err := NewFilterBuilder().
//	    IPv4(WithLabel("ipv4_check").orDrop()).
//	    TCP(JumpToIfMatch("tcp_ports"), JumpToIfNoMatch("udp_check")).
//	    Port(80, JumpToIfMatch(LabelAccept), JumpToIfNoMatch("check_https")).
//	    Label("check_https").
//	    PortOrDrop(443).
//	    Accept().
//	    Drop().
//	    Compile()
//
// Key Features:
//   - Automatic jump distance calculation
//   - Label-based control flow
//   - Type-safe option configuration
//   - Hardcoded Accept/Drop labels for common patterns
//   - Comprehensive error handling
//   - Support for IPv4, TCP, UDP, and port filtering
//
// See prebuildFilter for example implementations of common filter patterns.
type filterBuilder struct {
	instructions []bpf.Instruction
	labels       map[string]*label
	jumps        []jumpPlaceholder
	err          error
}

// NewFilterBuilder creates a new BPF filter builder instance
func NewFilterBuilder() *filterBuilder {
	return &filterBuilder{
		instructions: make([]bpf.Instruction, 0),
		labels:       make(map[string]*label),
		jumps:        make([]jumpPlaceholder, 0),
	}
}

// Port validation function
func (b *filterBuilder) validatePort(port uint32) error {
	if port == 0 {
		return fmt.Errorf("invalid port number: 0, port must be > 0")
	}
	if port > 65535 {
		return fmt.Errorf("invalid port number: %d, port must be <= 65535", port)
	}
	return nil
}

// Process options and chain configurations
func (b *filterBuilder) processOptions(opts []Option, chain *chainConfig) *optionConfig {
	config := &optionConfig{}

	// Process chain configuration
	if chain != nil {
		config.labelName = chain.labelName
		config.jumpIfMatch = chain.jumpIfMatch
		config.jumpIfNoMatch = chain.jumpIfNoMatch
	}

	// Process options
	for _, opt := range opts {
		opt.apply(config)
	}

	return config
}

// Create label if specified in configuration
func (b *filterBuilder) createLabelIfSpecified(labelName string) *filterBuilder {
	if labelName != "" {
		return b.Label(labelName)
	}
	return b
}

// Label creates a custom label at the current instruction position
func (b *filterBuilder) Label(labelName string) *filterBuilder {
	if b.err != nil {
		return b
	}

	// Check if label already exists
	if _, exists := b.labels[labelName]; exists {
		b.err = fmt.Errorf("label '%s' already exists", labelName)
		return b
	}

	// Create label and record current position
	b.labels[labelName] = &label{
		name:     labelName,
		position: len(b.instructions),
	}

	return b
}

// Internal method to add jump instructions with label placeholders
func (b *filterBuilder) jumpToLabel(condition bpf.JumpTest, value uint32, trueLabel, falseLabel string) *filterBuilder {
	if b.err != nil {
		return b
	}

	// Add placeholder jump instruction
	jumpInstruction := bpf.JumpIf{
		Cond:      condition,
		Val:       value,
		SkipTrue:  0, // Placeholder, resolved later
		SkipFalse: 0, // Placeholder, resolved later
	}

	currentIndex := len(b.instructions)
	b.instructions = append(b.instructions, jumpInstruction)

	// Record jump placeholders
	if trueLabel != "" {
		b.jumps = append(b.jumps, jumpPlaceholder{
			instructionIndex: currentIndex,
			targetLabel:      trueLabel,
			isSkipTrue:       true,
		})
	}

	if falseLabel != "" {
		b.jumps = append(b.jumps, jumpPlaceholder{
			instructionIndex: currentIndex,
			targetLabel:      falseLabel,
			isSkipTrue:       false,
		})
	}

	return b
}

// IPv4 adds IPv4 EtherType check with configurable jump behavior
func (b *filterBuilder) IPv4(options ...interface{}) *filterBuilder {
	if b.err != nil {
		return b
	}

	// Parse parameters
	var opts []Option
	var chain *chainConfig

	for _, opt := range options {
		switch v := opt.(type) {
		case Option:
			opts = append(opts, v)
		case *chainConfig:
			chain = v
		}
	}

	config := b.processOptions(opts, chain)

	// Create label if specified
	b.createLabelIfSpecified(config.labelName)

	// Add IPv4 check instructions
	b.instructions = append(b.instructions, bpf.LoadAbsolute{Off: 12, Size: 2})
	return b.jumpToLabel(bpf.JumpEqual, EthTypeIPv4, config.jumpIfMatch, config.jumpIfNoMatch)
}

// IPv4OrDrop adds IPv4 check, drops packet if not IPv4
func (b *filterBuilder) IPv4OrDrop() *filterBuilder {
	return b.IPv4(OrDrop())
}

// TCP adds TCP protocol check with configurable jump behavior
func (b *filterBuilder) TCP(options ...interface{}) *filterBuilder {
	if b.err != nil {
		return b
	}

	// Parse parameters
	var opts []Option
	var chain *chainConfig

	for _, opt := range options {
		switch v := opt.(type) {
		case Option:
			opts = append(opts, v)
		case *chainConfig:
			chain = v
		}
	}

	config := b.processOptions(opts, chain)

	// Create label if specified
	b.createLabelIfSpecified(config.labelName)

	// Add TCP check instructions
	b.instructions = append(b.instructions, bpf.LoadAbsolute{Off: 23, Size: 1})
	return b.jumpToLabel(bpf.JumpEqual, ProtocolTCP, config.jumpIfMatch, config.jumpIfNoMatch)
}

// TCPOrDrop adds TCP check, drops packet if not TCP
func (b *filterBuilder) TCPOrDrop() *filterBuilder {
	return b.TCP(OrDrop())
}

// UDP adds UDP protocol check with configurable jump behavior
func (b *filterBuilder) UDP(options ...interface{}) *filterBuilder {
	if b.err != nil {
		return b
	}

	// Parse parameters
	var opts []Option
	var chain *chainConfig

	for _, opt := range options {
		switch v := opt.(type) {
		case Option:
			opts = append(opts, v)
		case *chainConfig:
			chain = v
		}
	}

	config := b.processOptions(opts, chain)

	// Create label if specified
	b.createLabelIfSpecified(config.labelName)

	// Add UDP check instructions
	b.instructions = append(b.instructions, bpf.LoadAbsolute{Off: 23, Size: 1})
	return b.jumpToLabel(bpf.JumpEqual, ProtocolUDP, config.jumpIfMatch, config.jumpIfNoMatch)
}

// UDPOrDrop adds UDP check, drops packet if not UDP
func (b *filterBuilder) UDPOrDrop() *filterBuilder {
	return b.UDP(OrDrop())
}

// Port adds source or destination port check with configurable jump behavior
func (b *filterBuilder) Port(port uint32, options ...interface{}) *filterBuilder {
	if b.err != nil {
		return b
	}

	if err := b.validatePort(port); err != nil {
		b.err = err
		return b
	}

	// Parse parameters
	var opts []Option
	var chain *chainConfig

	for _, opt := range options {
		switch v := opt.(type) {
		case Option:
			opts = append(opts, v)
		case *chainConfig:
			chain = v
		}
	}

	config := b.processOptions(opts, chain)

	// Create label if specified
	b.createLabelIfSpecified(config.labelName)

	// Check source port
	b.instructions = append(b.instructions, bpf.LoadAbsolute{Off: 34, Size: 2})
	b.jumpToLabel(bpf.JumpEqual, port, config.jumpIfMatch, "")

	// Check destination port
	b.instructions = append(b.instructions, bpf.LoadAbsolute{Off: 36, Size: 2})
	b.jumpToLabel(bpf.JumpEqual, port, config.jumpIfMatch, config.jumpIfNoMatch)

	return b
}

// PortOrAccept adds port check, accepts if match, jumps to label if no match
func (b *filterBuilder) PortOrAccept(port uint32, noMatchJumpTo string) *filterBuilder {
	return b.Port(port, JumpToIfMatch(LabelAccept), JumpToIfNoMatch(noMatchJumpTo))
}

// PortOrDrop adds port check, accepts if match, drops if no match
func (b *filterBuilder) PortOrDrop(port uint32) *filterBuilder {
	return b.Port(port, JumpToIfMatch(LabelAccept), OrDrop())
}

// Accept adds packet acceptance instruction and creates accept label
func (b *filterBuilder) Accept() *filterBuilder {
	if b.err != nil {
		return b
	}

	// Automatically create Accept label
	if _, exists := b.labels[LabelAccept]; !exists {
		b.labels[LabelAccept] = &label{
			name:     LabelAccept,
			position: len(b.instructions),
		}
	}

	b.instructions = append(b.instructions, bpf.RetConstant{Val: 0xFFFFFFFF})
	return b
}

// Drop adds packet drop instruction and creates drop label
func (b *filterBuilder) Drop() *filterBuilder {
	if b.err != nil {
		return b
	}

	// Automatically create Drop label
	if _, exists := b.labels[LabelDrop]; !exists {
		b.labels[LabelDrop] = &label{
			name:     LabelDrop,
			position: len(b.instructions),
		}
	}

	b.instructions = append(b.instructions, bpf.RetConstant{Val: 0x00000000})
	return b
}

// Resolve jump placeholders by calculating actual jump distances
func (b *filterBuilder) resolveJumps() error {
	for _, jump := range b.jumps {
		targetLabel, exists := b.labels[jump.targetLabel]
		if !exists {
			return fmt.Errorf("undefined label: %s", jump.targetLabel)
		}

		// Calculate jump distance
		currentPos := jump.instructionIndex
		targetPos := targetLabel.position

		// BPF jumps are relative to the next instruction
		skipCount := targetPos - currentPos - 1

		if skipCount < 0 {
			return fmt.Errorf("backward jump not supported from instruction %d to label %s at %d",
				currentPos, jump.targetLabel, targetPos)
		}

		if skipCount > 255 {
			return fmt.Errorf("jump distance too large: %d (max 255)", skipCount)
		}

		// Update jump instruction
		jumpInst := b.instructions[jump.instructionIndex]
		if jumpIf, ok := jumpInst.(bpf.JumpIf); ok {
			if jump.isSkipTrue {
				jumpIf.SkipTrue = uint8(skipCount)
			} else {
				jumpIf.SkipFalse = uint8(skipCount)
			}
			b.instructions[jump.instructionIndex] = jumpIf
		}
	}

	return nil
}

// Compile resolves all jumps and assembles the final BPF filter
func (b *filterBuilder) Compile() ([]bpf.RawInstruction, error) {
	if b.err != nil {
		return nil, b.err
	}

	if len(b.instructions) == 0 {
		return nil, nil
	}

	// Phase 1: Resolve jumps
	if err := b.resolveJumps(); err != nil {
		return nil, err
	}

	// Phase 2: Assemble instructions
	filter, err := bpf.Assemble(b.instructions)
	if err != nil {
		return nil, err
	}

	return filter, nil
}

// Prebuild filters provide common BPF filter implementations.
// Users can reference these implementations to understand how to use
// the filterBuilder and Option interfaces effectively.
//
// Usage:
//
//	sipFilter, err := PrebuildFilter.SIP(5060)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// The prebuildFilter struct is intentionally private to encourage
// users to use the singleton instance PrebuildFilter.
type prebuildFilter struct{}

// PrebuildFilter provides ready-to-use BPF filter implementations
var PrebuildFilter = &prebuildFilter{}

// SIP creates a BPF filter for SIP protocol traffic on specified port.
// Accepts both TCP and UDP traffic on the given port.
//
// Example usage:
//
//	filter, err := PrebuildFilter.SIP(5060)
//	if err != nil {
//	    return err
//	}
//	// Use filter with raw socket or packet capture
func (f *prebuildFilter) SIP(port uint32) ([]bpf.RawInstruction, error) {
	return NewFilterBuilder().
		// Check if IPv4, drop if not
		IPv4OrDrop().

		// Check if TCP, if not jump to UDP check
		TCP(JumpToIfNoMatch("check_udp")).

		// TCP port check, accept if match, check UDP if no match
		PortOrAccept(port, "check_udp").

		// UDP check using chain configuration
		UDP(WithLabel("check_udp").OrDrop()).

		// UDP port check, accept if match, drop if no match
		PortOrDrop(port).

		// Hardcoded Accept and Drop labels are created automatically
		Accept().
		Drop().
		Compile()
}
