package handler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/google/gopacket/layers"
	"github.com/sirupsen/logrus"
)

// Mock handler function for testing
func mockHandler(frame *types.RawFrameData) error {
	return nil
}

func mockHandler2(frame *types.RawFrameData) error {
	return nil
}

// Setup function to initialize logger for tests
func setupTest() {
	if log.Logger == nil {
		log.Logger = logrus.New()
		log.Logger.SetLevel(logrus.ErrorLevel) // Reduce log noise in tests
	}
}

func TestNewManager(t *testing.T) {
	setupTest()

	tests := []struct {
		name       string
		allowMutex bool
	}{
		{"with mutex allowed", true},
		{"without mutex allowed", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.allowMutex)
			if manager == nil {
				t.Error("NewManager should not return nil")
			}
			if manager.allowMutex != tt.allowMutex {
				t.Errorf("allowMutex = %v, want %v", manager.allowMutex, tt.allowMutex)
			}
			if len(manager.handlerMappings) != 0 {
				t.Error("New manager should have empty mappings")
			}
		})
	}
}

func TestAddMappingNameValidation(t *testing.T) {
	setupTest()

	tests := []struct {
		name        string
		handlerName string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty name",
			handlerName: "",
			expectError: true,
			errorMsg:    "handler name cannot be empty",
		},
		{
			name:        "whitespace only name",
			handlerName: "   ",
			expectError: true,
			errorMsg:    "handler name cannot be empty or whitespace only",
		},
		{
			name:        "tab only name",
			handlerName: "\t\t",
			expectError: true,
			errorMsg:    "handler name cannot be empty or whitespace only",
		},
		{
			name:        "newline only name",
			handlerName: "\n\r",
			expectError: true,
			errorMsg:    "handler name cannot be empty or whitespace only",
		},
		{
			name:        "mixed whitespace",
			handlerName: " \t \n ",
			expectError: true,
			errorMsg:    "handler name cannot be empty or whitespace only",
		},
		{
			name:        "valid name with leading/trailing spaces",
			handlerName: "  valid_handler  ",
			expectError: false,
		},
		{
			name:        "valid name",
			handlerName: "valid_handler",
			expectError: false,
		},
		{
			name:        "name with special characters",
			handlerName: "handler-123_test@domain",
			expectError: false,
		},
		{
			name:        "name with unicode",
			handlerName: "处理器_测试",
			expectError: false,
		},
		{
			name:        "name with control characters",
			handlerName: "handler\x00test",
			expectError: false, // Should allow but test behavior
		},
		{
			name:        "very long name",
			handlerName: strings.Repeat("a", 1000),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh manager for each test to avoid state pollution
			manager := NewManager(false)
			err := manager.AddMapping(layers.IPProtocolTCP, "8080", tt.handlerName, mockHandler)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("error message should contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAddMappingHandlerValidation(t *testing.T) {
	setupTest()
	manager := NewManager(false)

	err := manager.AddMapping(layers.IPProtocolTCP, "8080", "test_handler", nil)
	if err == nil {
		t.Error("expected error for nil handler")
	}
	if !strings.Contains(err.Error(), "handler function cannot be nil") {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestAddMappingPortsValidation(t *testing.T) {
	setupTest()

	tests := []struct {
		name        string
		ports       string
		expectError bool
	}{
		{"valid single port", "8080", false},
		{"valid port range", "8000-8100", false},
		{"valid multiple ports", "80,443,8080", false},
		{"invalid port", "abc", true},
		{"invalid range", "8100-8000", true},
		{"empty ports", "", true},
		{"invalid port number", "65536", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh manager for each test
			manager := NewManager(false)
			err := manager.AddMapping(layers.IPProtocolTCP, tt.ports, "test_handler", mockHandler)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAddMappingDuplicateNames(t *testing.T) {
	setupTest()
	manager := NewManager(false)

	// Add first handler
	err := manager.AddMapping(layers.IPProtocolTCP, "8080", "test_handler", mockHandler)
	if err != nil {
		t.Fatalf("failed to add first handler: %v", err)
	}

	tests := []struct {
		name        string
		protocol    layers.IPProtocol
		ports       string
		handlerName string
		handler     func(*types.RawFrameData) error
		expectError bool
		errorMsg    string
	}{
		{
			name:        "same name same protocol additional ports",
			protocol:    layers.IPProtocolTCP,
			ports:       "9090",
			handlerName: "test_handler",
			handler:     mockHandler,
			expectError: false, // Should merge
		},
		{
			name:        "same name different protocol",
			protocol:    layers.IPProtocolUDP,
			ports:       "8080",
			handlerName: "test_handler",
			handler:     mockHandler,
			expectError: true,
			errorMsg:    "already exists with different protocol",
		},
		{
			name:        "same name same protocol overlapping ports",
			protocol:    layers.IPProtocolTCP,
			ports:       "8080-8090",
			handlerName: "test_handler",
			handler:     mockHandler,
			expectError: false, // Should merge
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AddMapping(tt.protocol, tt.ports, tt.handlerName, tt.handler)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("error message should contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAddMappingConflicts(t *testing.T) {
	setupTest()

	tests := []struct {
		name       string
		allowMutex bool
	}{
		{"without mutex", false},
		{"with mutex", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.allowMutex)

			// Add first handler
			err := manager.AddMapping(layers.IPProtocolTCP, "8080", "handler1", mockHandler)
			if err != nil {
				t.Fatalf("failed to add first handler: %v", err)
			}

			// Try to add conflicting handler
			err = manager.AddMapping(layers.IPProtocolTCP, "8080", "handler2", mockHandler2)

			if tt.allowMutex {
				if err != nil {
					t.Errorf("should allow conflicting handlers when mutex is enabled: %v", err)
				}
			} else {
				if err == nil {
					t.Error("should reject conflicting handlers when mutex is disabled")
				}
				if !strings.Contains(err.Error(), "conflicts with existing handler") {
					t.Errorf("unexpected error message: %s", err.Error())
				}
			}
		})
	}
}

func TestGetHandler(t *testing.T) {
	setupTest()
	manager := NewManager(false)

	// Add test handlers
	err := manager.AddMapping(layers.IPProtocolTCP, "8080,9000-9100", "tcp_handler", mockHandler)
	if err != nil {
		t.Fatalf("failed to add TCP handler: %v", err)
	}

	err = manager.AddMapping(layers.IPProtocolUDP, "53,5353", "udp_handler", mockHandler2)
	if err != nil {
		t.Fatalf("failed to add UDP handler: %v", err)
	}

	tests := []struct {
		name     string
		protocol layers.IPProtocol
		port     int
		found    bool
	}{
		{"TCP port 8080", layers.IPProtocolTCP, 8080, true},
		{"TCP port 9050", layers.IPProtocolTCP, 9050, true},
		{"TCP port 9101", layers.IPProtocolTCP, 9101, false},
		{"UDP port 53", layers.IPProtocolUDP, 53, true},
		{"UDP port 5353", layers.IPProtocolUDP, 5353, true},
		{"UDP port 8080", layers.IPProtocolUDP, 8080, false},
		{"ICMP protocol", layers.IPProtocolICMPv4, 8080, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, found := manager.GetHandler(tt.protocol, tt.port)

			if found != tt.found {
				t.Errorf("found = %v, want %v", found, tt.found)
			}

			if tt.found && handler == nil {
				t.Error("handler should not be nil when found")
			}

			if !tt.found && handler != nil {
				t.Error("handler should be nil when not found")
			}
		})
	}
}

func TestManagerOperations(t *testing.T) {
	setupTest()
	manager := NewManager(false)

	// Test empty manager
	if manager.GetHandlerCount() != 0 {
		t.Error("new manager should have 0 handlers")
	}

	handlers := manager.ListHandlers()
	if len(handlers) != 0 {
		t.Error("new manager should list 0 handlers")
	}

	// Add some handlers
	manager.AddMapping(layers.IPProtocolTCP, "8080", "handler1", mockHandler)
	manager.AddMapping(layers.IPProtocolUDP, "53", "handler2", mockHandler2)

	// Test count
	if manager.GetHandlerCount() != 2 {
		t.Errorf("manager should have 2 handlers, got %d", manager.GetHandlerCount())
	}

	// Test list
	handlers = manager.ListHandlers()
	if len(handlers) != 2 {
		t.Errorf("should list 2 handlers, got %d", len(handlers))
	}

	// Test get by name
	mapping, exists := manager.GetHandlerByName("handler1")
	if !exists {
		t.Error("handler1 should exist")
	}
	if mapping == nil {
		t.Error("mapping should not be nil")
	}

	// Test get non-existent
	_, exists = manager.GetHandlerByName("nonexistent")
	if exists {
		t.Error("nonexistent handler should not exist")
	}

	// Test remove
	removed := manager.RemoveHandler("handler1")
	if !removed {
		t.Error("should have removed handler1")
	}

	if manager.GetHandlerCount() != 1 {
		t.Errorf("manager should have 1 handler after removal, got %d", manager.GetHandlerCount())
	}

	// Test remove non-existent
	removed = manager.RemoveHandler("nonexistent")
	if removed {
		t.Error("should not have removed nonexistent handler")
	}
}

func TestEdgeCases(t *testing.T) {
	setupTest()
	manager := NewManager(false)

	// Test with very specific protocol combinations
	protocols := []layers.IPProtocol{
		layers.IPProtocolTCP,
		layers.IPProtocolUDP,
		layers.IPProtocolICMPv4,
		layers.IPProtocolICMPv6,
		layers.IPProtocolIPv6HopByHop,
	}

	for i, protocol := range protocols {
		handlerName := fmt.Sprintf("handler_%d", i)
		err := manager.AddMapping(protocol, "8080", handlerName, mockHandler)
		if err != nil {
			t.Errorf("failed to add handler for protocol %s: %v", protocol, err)
		}
	}

	// All should be added successfully
	if manager.GetHandlerCount() != len(protocols) {
		t.Errorf("expected %d handlers, got %d", len(protocols), manager.GetHandlerCount())
	}

	// Test handler with empty port ranges (should fail)
	err := manager.AddMapping(layers.IPProtocolTCP, "", "empty_ports", mockHandler)
	if err == nil {
		t.Error("should fail with empty ports")
	}
}

func TestConcurrentNameNormalization(t *testing.T) {
	setupTest()
	manager := NewManager(false)

	// Names that should be treated as equivalent after trimming
	names := []string{
		"handler",
		" handler ",
		"  handler  ",
		"\thandler\t",
		"\nhandler\n",
	}

	// First one should succeed
	err := manager.AddMapping(layers.IPProtocolTCP, "8080", names[0], mockHandler)
	if err != nil {
		t.Fatalf("failed to add first handler: %v", err)
	}

	// All others should be treated as merges (same name after trimming)
	for i := 1; i < len(names); i++ {
		err := manager.AddMapping(layers.IPProtocolTCP, "9090", names[i], mockHandler)
		if err != nil {
			t.Errorf("failed to merge handler with name '%s': %v", names[i], err)
		}
	}

	// Should still have only 1 handler (all were merged)
	if manager.GetHandlerCount() != 1 {
		t.Errorf("expected 1 handler after merging, got %d", manager.GetHandlerCount())
	}
}
