# AFPacket Server

This is a network packet capture server for SkyWalking Satellite based on AF_PACKET v3.

## Overview

The AFPacket server provides high-performance network packet capture capabilities using Linux AF_PACKET sockets. It captures packets from network interfaces and processes them through configurable handlers to extract relevant data for monitoring and analysis.

## Architecture

The server consists of several key components:

- **PacketCapture**: Manages low-level packet capture using AF_PACKET v3
- **EventLoop**: Coordinates packet processing and distribution
- **HandlerManager**: Manages packet handlers for different protocols
- **DataPipeline**: Provides async processing with non-blocking channels
- **MonitoringManager**: Tracks performance metrics and drop rates

## Key Features

1. **Complete Lifecycle Management**: All components support graceful shutdown
2. **Extensible Design**: Plugin-based handlers for different packet types
3. **Non-blocking Processing**: Independent pipelines prevent blocking between data types
4. **Drop Reporting**: Configurable thresholds for packet drop monitoring
5. **Performance Monitoring**: Real-time statistics and health metrics

## Configuration

```yaml
servers:
  - plugin_name: "afpacket-server"
    # Network interface to capture on
    interface: "eth0"
    # Ring buffer size (number of blocks)
    buffer_size: 1024
    # BPF filter expression for packet filtering
    filter: "tcp or udp"
    # Statistics reporting interval
    stats_interval: 10s
    # Drop count threshold for reporting alerts
    drop_threshold: 100
    # Handler configurations
    handlers:
      - name: "http-handler"
        enabled: true
        pipeline_buffer_size: 1000
        drop_policy: "drop_oldest"
      - name: "tcp-handler"
        enabled: true
        pipeline_buffer_size: 500
        drop_policy: "block"
```

## Usage

The AFPacket server integrates with SkyWalking Satellite as a sharing plugin and works with receiver components to collect network monitoring data.

## Requirements

- Linux system with AF_PACKET support
- Root privileges or appropriate capabilities for packet capture
- Network interface access

## Components

### PacketCapture
- Initializes AF_PACKET v3 sockets
- Manages ring buffers for high-performance capture
- Provides packet source for processing

### EventLoop
- Reads packets from capture source
- Distributes packets to appropriate handlers
- Manages event processing lifecycle

### HandlerManager
- Registers and manages packet handlers
- Routes packets to capable handlers
- Supports dynamic handler registration

### PacketHandlers
- **HTTPHandler**: Processes HTTP traffic
- **TCPHandler**: Processes TCP packets
- **UDPHandler**: Processes UDP packets
- Extensible for custom protocol handlers

### DataPipeline
- Async processing with independent channels
- Non-blocking submission with drop policies
- Worker pool management for scalability

### MonitoringManager
- Periodic statistics reporting
- Drop rate monitoring and alerting
- Performance metrics collection

## Integration

The server implements the SkyWalking Satellite `api.Server` interface and can be used as a sharing plugin across multiple pipes for efficient resource utilization.
