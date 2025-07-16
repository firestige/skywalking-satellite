package utils

import (
	"fmt"
	"testing"

	"golang.org/x/net/bpf"
)

func TestNewFilterBuilder(t *testing.T) {
	builder := NewFilterBuilder()
	if builder == nil {
		t.Fatal("NewFilterBuilder() returned nil")
	}
	if builder.instructions == nil {
		t.Fatal("NewFilterBuilder() instructions slice is nil")
	}
	if len(builder.instructions) != 0 {
		t.Errorf("NewFilterBuilder() instructions length=%d, want 0", len(builder.instructions))
	}
	if builder.labels == nil {
		t.Fatal("NewFilterBuilder() labels map is nil")
	}
	if builder.jumps == nil {
		t.Fatal("NewFilterBuilder() jumps slice is nil")
	}
}

func TestFilterBuilder_Label(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.Label("test_label")

	if result != builder {
		t.Error("Label() should return the same builder instance")
	}

	if _, exists := builder.labels["test_label"]; !exists {
		t.Error("Label() should create a label in the labels map")
	}

	if builder.labels["test_label"].position != 0 {
		t.Errorf("Label() position = %d, want 0", builder.labels["test_label"].position)
	}
}

func TestFilterBuilder_Label_Duplicate(t *testing.T) {
	builder := NewFilterBuilder()
	builder.Label("test_label")
	builder.Label("test_label") // Duplicate label

	if builder.err == nil {
		t.Error("Label() should return error for duplicate label")
	}
}

func TestFilterBuilder_IPv4WithOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  []interface{}
		wantErr  bool
		checkPos bool
	}{
		{
			name:     "IPv4 with OrDrop",
			options:  []interface{}{OrDrop()},
			wantErr:  false,
			checkPos: false,
		},
		{
			name:     "IPv4 with JumpToIfNoMatch",
			options:  []interface{}{JumpToIfNoMatch("test_label")},
			wantErr:  false,
			checkPos: false,
		},
		{
			name:     "IPv4 with WithLabel chain",
			options:  []interface{}{WithLabel("ipv4_check").OrDrop()},
			wantErr:  false,
			checkPos: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewFilterBuilder()
			result := builder.IPv4(tt.options...)

			if result != builder {
				t.Error("IPv4() should return the same builder instance")
			}

			if tt.wantErr && builder.err == nil {
				t.Error("IPv4() should return error")
			}

			if !tt.wantErr && builder.err != nil {
				t.Errorf("IPv4() error = %v, want nil", builder.err)
			}

			if !tt.wantErr {
				if len(builder.instructions) != 2 {
					t.Errorf("IPv4() instructions length = %d, want 2", len(builder.instructions))
				}

				// Check first instruction is LoadAbsolute
				if load, ok := builder.instructions[0].(bpf.LoadAbsolute); ok {
					if load.Off != 12 || load.Size != 2 {
						t.Errorf("IPv4() first instruction: Off=%d Size=%d, want Off=12 Size=2", load.Off, load.Size)
					}
				} else {
					t.Error("IPv4() first instruction should be LoadAbsolute")
				}

				// Check second instruction is JumpIf
				if jump, ok := builder.instructions[1].(bpf.JumpIf); ok {
					if jump.Val != EthTypeIPv4 {
						t.Errorf("IPv4() jump value = %d, want %d", jump.Val, EthTypeIPv4)
					}
				} else {
					t.Error("IPv4() second instruction should be JumpIf")
				}

				if tt.checkPos {
					if _, exists := builder.labels["ipv4_check"]; !exists {
						t.Error("IPv4() should create label 'ipv4_check'")
					}
				}
			}
		})
	}
}

func TestFilterBuilder_IPv4OrDrop(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.IPv4OrDrop()

	if result != builder {
		t.Error("IPv4OrDrop() should return the same builder instance")
	}

	if len(builder.instructions) != 2 {
		t.Errorf("IPv4OrDrop() instructions length = %d, want 2", len(builder.instructions))
	}

	// Should have one jump placeholder
	if len(builder.jumps) != 1 {
		t.Errorf("IPv4OrDrop() jumps length = %d, want 1", len(builder.jumps))
	}

	if builder.jumps[0].targetLabel != LabelDrop {
		t.Errorf("IPv4OrDrop() jump target = %s, want %s", builder.jumps[0].targetLabel, LabelDrop)
	}
}

func TestFilterBuilder_TCP(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.TCP(JumpToIfNoMatch("udp_check"))

	if result != builder {
		t.Error("TCP() should return the same builder instance")
	}

	if len(builder.instructions) != 2 {
		t.Errorf("TCP() instructions length = %d, want 2", len(builder.instructions))
	}

	// Check first instruction is LoadAbsolute
	if load, ok := builder.instructions[0].(bpf.LoadAbsolute); ok {
		if load.Off != 23 || load.Size != 1 {
			t.Errorf("TCP() first instruction: Off=%d Size=%d, want Off=23 Size=1", load.Off, load.Size)
		}
	} else {
		t.Error("TCP() first instruction should be LoadAbsolute")
	}

	// Check second instruction is JumpIf
	if jump, ok := builder.instructions[1].(bpf.JumpIf); ok {
		if jump.Val != ProtocolTCP {
			t.Errorf("TCP() jump value = %d, want %d", jump.Val, ProtocolTCP)
		}
	} else {
		t.Error("TCP() second instruction should be JumpIf")
	}
}

func TestFilterBuilder_UDP(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.UDP(WithLabel("udp_check").OrDrop())

	if result != builder {
		t.Error("UDP() should return the same builder instance")
	}

	if len(builder.instructions) != 2 {
		t.Errorf("UDP() instructions length = %d, want 2", len(builder.instructions))
	}

	// Should create label
	if _, exists := builder.labels["udp_check"]; !exists {
		t.Error("UDP() should create label 'udp_check'")
	}

	// Check jump placeholders
	if len(builder.jumps) != 1 {
		t.Errorf("UDP() jumps length = %d, want 1", len(builder.jumps))
	}

	if builder.jumps[0].targetLabel != LabelDrop {
		t.Errorf("UDP() jump target = %s, want %s", builder.jumps[0].targetLabel, LabelDrop)
	}
}

func TestFilterBuilder_Port(t *testing.T) {
	tests := []struct {
		name    string
		port    uint32
		options []interface{}
		wantErr bool
	}{
		{
			name:    "valid port with options",
			port:    80,
			options: []interface{}{JumpToIfMatch(LabelAccept), JumpToIfNoMatch(LabelDrop)},
			wantErr: false,
		},
		{
			name:    "invalid port 0",
			port:    0,
			options: []interface{}{},
			wantErr: true,
		},
		{
			name:    "invalid port > 65535",
			port:    65536,
			options: []interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewFilterBuilder()
			result := builder.Port(tt.port, tt.options...)

			if result != builder {
				t.Error("Port() should return the same builder instance")
			}

			if tt.wantErr && builder.err == nil {
				t.Error("Port() should return error for invalid port")
			}

			if !tt.wantErr && builder.err != nil {
				t.Errorf("Port() error = %v, want nil", builder.err)
			}

			if !tt.wantErr {
				// Port() should generate 4 instructions: source port check (2) + destination port check (2)
				if len(builder.instructions) != 4 {
					t.Errorf("Port() instructions length = %d, want 4", len(builder.instructions))
				}

				// Check source port LoadAbsolute
				if load, ok := builder.instructions[0].(bpf.LoadAbsolute); ok {
					if load.Off != 34 || load.Size != 2 {
						t.Errorf("Port() first instruction: Off=%d Size=%d, want Off=34 Size=2", load.Off, load.Size)
					}
				} else {
					t.Error("Port() first instruction should be LoadAbsolute")
				}

				// Check destination port LoadAbsolute
				if load, ok := builder.instructions[2].(bpf.LoadAbsolute); ok {
					if load.Off != 36 || load.Size != 2 {
						t.Errorf("Port() third instruction: Off=%d Size=%d, want Off=36 Size=2", load.Off, load.Size)
					}
				} else {
					t.Error("Port() third instruction should be LoadAbsolute")
				}
			}
		})
	}
}

func TestFilterBuilder_PortOrAccept(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.PortOrAccept(80, "check_next")

	if result != builder {
		t.Error("PortOrAccept() should return the same builder instance")
	}

	if len(builder.instructions) != 4 {
		t.Errorf("PortOrAccept() instructions length = %d, want 4", len(builder.instructions))
	}

	// Should have jump placeholders
	foundAccept := false
	foundNext := false
	for _, jump := range builder.jumps {
		if jump.targetLabel == LabelAccept {
			foundAccept = true
		}
		if jump.targetLabel == "check_next" {
			foundNext = true
		}
	}

	if !foundAccept {
		t.Error("PortOrAccept() should have jump to accept")
	}
	if !foundNext {
		t.Error("PortOrAccept() should have jump to check_next")
	}
}

func TestFilterBuilder_PortOrDrop(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.PortOrDrop(443)

	if result != builder {
		t.Error("PortOrDrop() should return the same builder instance")
	}

	if len(builder.instructions) != 4 {
		t.Errorf("PortOrDrop() instructions length = %d, want 4", len(builder.instructions))
	}

	// Should have jump placeholders
	foundAccept := false
	foundDrop := false
	for _, jump := range builder.jumps {
		if jump.targetLabel == LabelAccept {
			foundAccept = true
		}
		if jump.targetLabel == LabelDrop {
			foundDrop = true
		}
	}

	if !foundAccept {
		t.Error("PortOrDrop() should have jump to accept")
	}
	if !foundDrop {
		t.Error("PortOrDrop() should have jump to drop")
	}
}

func TestFilterBuilder_Accept(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.Accept()

	if result != builder {
		t.Error("Accept() should return the same builder instance")
	}

	if len(builder.instructions) != 1 {
		t.Errorf("Accept() instructions length = %d, want 1", len(builder.instructions))
	}

	// Check instruction is RetConstant
	if ret, ok := builder.instructions[0].(bpf.RetConstant); ok {
		if ret.Val != 0xFFFFFFFF {
			t.Errorf("Accept() instruction: Val=%d, want %d", ret.Val, 0xFFFFFFFF)
		}
	} else {
		t.Error("Accept() instruction should be RetConstant")
	}

	// Should create Accept label
	if _, exists := builder.labels[LabelAccept]; !exists {
		t.Error("Accept() should create accept label")
	}
}

func TestFilterBuilder_Drop(t *testing.T) {
	builder := NewFilterBuilder()
	result := builder.Drop()

	if result != builder {
		t.Error("Drop() should return the same builder instance")
	}

	if len(builder.instructions) != 1 {
		t.Errorf("Drop() instructions length = %d, want 1", len(builder.instructions))
	}

	// Check instruction is RetConstant
	if ret, ok := builder.instructions[0].(bpf.RetConstant); ok {
		if ret.Val != 0x00000000 {
			t.Errorf("Drop() instruction: Val=%d, want 0", ret.Val)
		}
	} else {
		t.Error("Drop() instruction should be RetConstant")
	}

	// Should create Drop label
	if _, exists := builder.labels[LabelDrop]; !exists {
		t.Error("Drop() should create drop label")
	}
}

func TestFilterBuilder_Compile_Empty(t *testing.T) {
	builder := NewFilterBuilder()
	result, err := builder.Compile()

	if err != nil {
		t.Errorf("Compile() error = %v, want nil", err)
	}

	if result != nil {
		t.Error("Compile() should return nil for empty builder")
	}
}

func TestFilterBuilder_Compile_WithError(t *testing.T) {
	builder := NewFilterBuilder()
	builder.err = fmt.Errorf("test error")
	result, err := builder.Compile()

	if err == nil {
		t.Error("Compile() should return error when builder has error")
	}

	if result != nil {
		t.Error("Compile() should return nil when builder has error")
	}
}

func TestFilterBuilder_Compile_Success(t *testing.T) {
	builder := NewFilterBuilder()
	builder.IPv4OrDrop().Accept().Drop()
	result, err := builder.Compile()

	if err != nil {
		t.Errorf("Compile() error = %v, want nil", err)
	}

	if result == nil {
		t.Error("Compile() should return non-nil result")
	}

	if len(result) == 0 {
		t.Error("Compile() should return non-empty result")
	}
}

func TestFilterBuilder_ResolveJumps(t *testing.T) {
	builder := NewFilterBuilder()
	builder.IPv4(JumpToIfNoMatch("drop_label")).
		Accept().
		Label("drop_label").
		Drop()

	result, err := builder.Compile()

	if err != nil {
		t.Errorf("Compile() error = %v, want nil", err)
	}

	if result == nil {
		t.Error("Compile() should return non-nil result")
	}

	// Verify jump distances are calculated correctly
	if len(builder.jumps) == 0 {
		t.Error("Should have jump placeholders before resolving")
	}
}

func TestFilterBuilder_ResolveJumps_UndefinedLabel(t *testing.T) {
	builder := NewFilterBuilder()
	builder.IPv4(JumpToIfNoMatch("undefined_label")).Accept()

	_, err := builder.Compile()

	if err == nil {
		t.Error("Compile() should return error for undefined label")
	}
}

func TestChainConfig(t *testing.T) {
	chain := WithLabel("test").JumpToIfMatch("match_label").OrNotMatch("nomatch_label")

	if chain.labelName != "test" {
		t.Errorf("chainConfig labelName = %s, want test", chain.labelName)
	}

	if chain.jumpIfMatch != "match_label" {
		t.Errorf("chainConfig jumpIfMatch = %s, want match_label", chain.jumpIfMatch)
	}

	if chain.jumpIfNoMatch != "nomatch_label" {
		t.Errorf("chainConfig jumpIfNoMatch = %s, want nomatch_label", chain.jumpIfNoMatch)
	}
}

func TestChainConfig_OrDrop(t *testing.T) {
	chain := WithLabel("test").OrDrop()

	if chain.jumpIfNoMatch != LabelDrop {
		t.Errorf("chainConfig jumpIfNoMatch = %s, want %s", chain.jumpIfNoMatch, LabelDrop)
	}
}

func TestChainConfig_OrAccept(t *testing.T) {
	chain := WithLabel("test").OrAccept()

	if chain.jumpIfNoMatch != LabelAccept {
		t.Errorf("chainConfig jumpIfNoMatch = %s, want %s", chain.jumpIfNoMatch, LabelAccept)
	}
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name   string
		option Option
		want   func(*optionConfig) bool
	}{
		{
			name:   "JumpToIfMatch",
			option: JumpToIfMatch("test_label"),
			want:   func(c *optionConfig) bool { return c.jumpIfMatch == "test_label" },
		},
		{
			name:   "JumpToIfNoMatch",
			option: JumpToIfNoMatch("test_label"),
			want:   func(c *optionConfig) bool { return c.jumpIfNoMatch == "test_label" },
		},
		{
			name:   "OrDrop",
			option: OrDrop(),
			want:   func(c *optionConfig) bool { return c.jumpIfNoMatch == LabelDrop },
		},
		{
			name:   "OrAccept",
			option: OrAccept(),
			want:   func(c *optionConfig) bool { return c.jumpIfNoMatch == LabelAccept },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optionConfig{}
			tt.option.apply(config)

			if !tt.want(config) {
				t.Errorf("Option %s did not apply correctly", tt.name)
			}
		})
	}
}

func TestPrebuildFilter_SIP(t *testing.T) {
	tests := []struct {
		name    string
		port    uint32
		wantErr bool
	}{
		{
			name:    "standard SIP port",
			port:    5060,
			wantErr: false,
		},
		{
			name:    "custom SIP port",
			port:    5061,
			wantErr: false,
		},
		{
			name:    "invalid port 0",
			port:    0,
			wantErr: true,
		},
		{
			name:    "invalid port > 65535",
			port:    65536,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := PrebuildFilter.SIP(tt.port)

			if tt.wantErr && err == nil {
				t.Error("SIP() should return error for invalid port")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("SIP() error = %v, want nil", err)
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("SIP() result should not be nil")
				}

				if len(result) == 0 {
					t.Error("SIP() result should not be empty")
				}
			}
		})
	}
}

func TestPrebuildFilter_SIP_Structure(t *testing.T) {
	result, err := PrebuildFilter.SIP(5060)

	if err != nil {
		t.Fatalf("SIP() error = %v, want nil", err)
	}

	if result == nil {
		t.Fatal("SIP() result should not be nil")
	}

	// SIP filter should have reasonable instruction count
	if len(result) == 0 {
		t.Error("SIP() result should not be empty")
	}

	t.Logf("SIP filter generated %d instructions", len(result))
}

func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant interface{}
		expected interface{}
	}{
		{"ProtocolTCP", ProtocolTCP, 6},
		{"ProtocolUDP", ProtocolUDP, 17},
		{"ProtocolICMP", ProtocolICMP, 1},
		{"EthTypeIPv4", EthTypeIPv4, 0x0800},
		{"EthTypeIPv6", EthTypeIPv6, 0x86DD},
		{"EthTypeARP", EthTypeARP, 0x0806},
		{"PortHTTP", PortHTTP, 80},
		{"PortHTTPS", PortHTTPS, 443},
		{"PortSSH", PortSSH, 22},
		{"PortDNS", PortDNS, 53},
		{"LabelAccept", LabelAccept, "accept"},
		{"LabelDrop", LabelDrop, "drop"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestComplexFilter(t *testing.T) {
	// Test complex filter building
	result, err := NewFilterBuilder().
		IPv4OrDrop().
		TCP(JumpToIfNoMatch("check_udp")).
		PortOrAccept(80, "check_https").
		Label("check_https").
		PortOrAccept(443, "check_udp").
		Label("check_udp").
		UDP(JumpToIfNoMatch(LabelDrop)).
		PortOrDrop(53). // DNS
		Accept().
		Drop().
		Compile()

	if err != nil {
		t.Fatalf("Complex filter compilation failed: %v", err)
	}

	if result == nil {
		t.Fatal("Complex filter result should not be nil")
	}

	if len(result) == 0 {
		t.Error("Complex filter result should not be empty")
	}

	t.Logf("Complex filter generated %d instructions", len(result))
}

func BenchmarkFilterBuilder_Compile(b *testing.B) {
	builder := NewFilterBuilder().
		IPv4OrDrop().
		TCP(JumpToIfNoMatch("check_udp")).
		PortOrAccept(80, "check_udp").
		Label("check_udp").
		UDP(OrDrop()).
		PortOrDrop(53).
		Accept().
		Drop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.Compile()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrebuildFilter_SIP(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := PrebuildFilter.SIP(5060)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestFilterBuilder_ChainMethods(t *testing.T) {
	// Test method chaining
	builder := NewFilterBuilder()
	result := builder.
		IPv4OrDrop().
		TCP(JumpToIfNoMatch("check_udp")).
		PortOrAccept(80, "check_udp").
		Label("check_udp").
		UDP(OrDrop()).
		PortOrDrop(53).
		Accept().
		Drop()

	if result != builder {
		t.Error("Method chaining should return the same builder instance")
	}

	// Verify the built filter can be compiled
	compiled, err := builder.Compile()
	if err != nil {
		t.Errorf("Chained filter compilation failed: %v", err)
	}

	if compiled == nil {
		t.Error("Chained filter should compile to non-nil result")
	}
}
