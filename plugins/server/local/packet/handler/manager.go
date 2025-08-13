package handler

import (
	"fmt"
	"strings"

	"github.com/apache/skywalking-satellite/internal/pkg/log"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/types"
	"github.com/apache/skywalking-satellite/plugins/server/local/packet/utils"
	"github.com/google/gopacket/layers"
)

type frameHandlerAdapter struct {
	name    string // Handler name for logging
	handler func(frame *types.RawFrameData) error
}

func newFrameHandlerAdapter(name string, handler func(frame *types.RawFrameData) error) *frameHandlerAdapter {
	if handler == nil {
		log.Logger.Error("Handler function cannot be nil")
		return nil
	}
	return &frameHandlerAdapter{name: name, handler: handler}
}

func (a *frameHandlerAdapter) Handle(frame *types.RawFrameData) error {
	return a.handler(frame)
}

func (a *frameHandlerAdapter) Name() string {
	return a.name
}

type HandlerMapping struct {
	protocol layers.IPProtocol // Protocol type (e.g., TCP, UDP)
	mathcer  utils.PortMatcher // List of port ranges
	handler  *frameHandlerAdapter
}

func (mapping *HandlerMapping) getHandler(protocol layers.IPProtocol, port int) (*frameHandlerAdapter, bool) {
	if mapping.isMatch(protocol, port) {
		return mapping.handler, true
	}
	return nil, false
}

func (mapping *HandlerMapping) isMatch(protocol layers.IPProtocol, port int) bool {
	if protocol == mapping.protocol {
		return mapping.mathcer.IsMatch(port)
	}
	return false
}

type Manager struct {
	handlerMappings map[string]*HandlerMapping // name -> HandlerMapping
	allowMutex      bool
}

// NewManager creates a new Manager instance
func NewManager(allowMutex bool) *Manager {
	return &Manager{
		handlerMappings: make(map[string]*HandlerMapping),
		allowMutex:      allowMutex,
	}
}

func (m *Manager) AddMapping(protocol layers.IPProtocol, ports string, name string, handler func(*types.RawFrameData) error) error {
	// Validate name
	if name == "" {
		return fmt.Errorf("handler name cannot be empty")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("handler name cannot be empty or whitespace only")
	}

	// Validate handler
	if handler == nil {
		return fmt.Errorf("handler function cannot be nil")
	}

	// Create port matcher
	matcher, err := utils.NewPortMatcher(ports, true)
	if err != nil {
		return fmt.Errorf("failed to create port matcher: %w", err)
	}

	// Check if handler with same name already exists
	if existingMapping, exists := m.handlerMappings[name]; exists {
		return m.handleExistingMapping(existingMapping, protocol, matcher, name, handler)
	}

	// Create new mapping
	newMapping := &HandlerMapping{
		protocol: protocol,
		mathcer:  matcher,
		handler:  newFrameHandlerAdapter(name, handler),
	}

	// Check for conflicts with other handlers
	if err := m.checkForConflicts(newMapping, name); err != nil {
		return err
	}

	// Add new mapping
	m.handlerMappings[name] = newMapping
	log.Logger.Infof("Added new handler mapping: name=%s, protocol=%s", name, protocol)
	return nil
}

// handleExistingMapping handles the case where a mapping with the same name already exists
func (m *Manager) handleExistingMapping(existingMapping *HandlerMapping, protocol layers.IPProtocol, matcher utils.PortMatcher, name string, handler func(*types.RawFrameData) error) error {
	// Check if protocol matches
	if existingMapping.protocol != protocol {
		return fmt.Errorf("handler name '%s' already exists with different protocol: existing=%s, new=%s", name, existingMapping.protocol, protocol)
	}

	// Merge port matchers
	log.Logger.Debugf("Handler named %s already exists, merging port matcher", name)
	mergedMatcher, err := existingMapping.mathcer.Merge(matcher)
	if err != nil {
		return fmt.Errorf("failed to merge port matchers for handler '%s': %w", name, err)
	}

	// Update existing mapping
	existingMapping.mathcer = mergedMatcher
	// Note: We keep the original handler function, assuming same name means same handler logic
	log.Logger.Infof("Merged port matcher for existing handler: name=%s", name)
	return nil
}

// checkForConflicts checks if the new mapping conflicts with existing mappings
func (m *Manager) checkForConflicts(newMapping *HandlerMapping, newName string) error {
	for existingName, existingMapping := range m.handlerMappings {
		// Skip if same name (already handled)
		if existingName == newName {
			continue
		}

		// Check if same protocol and overlapping ports
		if existingMapping.protocol == newMapping.protocol && existingMapping.mathcer.IsOverlapped(newMapping.mathcer) {
			if m.allowMutex {
				log.Logger.Warnf("Port overlap detected between handlers '%s' and '%s' for protocol %s, but mutex is allowed", existingName, newName, newMapping.protocol)
			} else {
				return fmt.Errorf("handler '%s' conflicts with existing handler '%s' on protocol %s with overlapping ports", newName, existingName, newMapping.protocol)
			}
		}
	}
	return nil
}

func (m *Manager) GetHandler(protocol layers.IPProtocol, port int) (types.FrameHandler, bool) {
	for name, mapping := range m.handlerMappings {
		if handler, exists := mapping.getHandler(protocol, port); exists {
			log.Logger.Debugf("Found handler for protocol %s and port %d: %s", protocol, port, name)
			return handler, true
		}
	}
	return nil, false
}

func (m *Manager) GetDefaultHandlerByProtocol(protocol layers.IPProtocol) (types.FrameHandler, bool) {
	for _, mapping := range m.handlerMappings {
		if mapping.protocol == protocol {
			return mapping.handler, true
		}
	}
	return nil, false
}

// GetHandlerByName returns a handler by its name
func (m *Manager) GetHandlerByName(name string) (*HandlerMapping, bool) {
	mapping, exists := m.handlerMappings[name]
	return mapping, exists
}

// RemoveHandler removes a handler by name
func (m *Manager) RemoveHandler(name string) bool {
	if _, exists := m.handlerMappings[name]; exists {
		delete(m.handlerMappings, name)
		log.Logger.Infof("Removed handler: %s", name)
		return true
	}
	return false
}

// ListHandlers returns all handler names
func (m *Manager) ListHandlers() []string {
	names := make([]string, 0, len(m.handlerMappings))
	for name := range m.handlerMappings {
		names = append(names, name)
	}
	return names
}

// GetHandlerCount returns the number of registered handlers
func (m *Manager) GetHandlerCount() int {
	return len(m.handlerMappings)
}
