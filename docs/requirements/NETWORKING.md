# Networking Requirements

## Telnet Transport

- NET-1: The server MUST implement the Telnet protocol (RFC 854) over TCP.
- NET-2: The server MUST support ANSI color codes for text formatting.
- NET-3: The server MUST negotiate Telnet options including NAWS (window size), TTYPE (terminal type), and CHARSET.
- NET-4: The server SHOULD support the GMCP (Generic MUD Communication Protocol) for structured data exchange with MUD clients.
- NET-5: The server SHOULD support MSSP (MUD Server Status Protocol) for MUD listing services.
- NET-6: The server MUST handle Telnet IAC sequences correctly and not pass them to the command parser.

## WebSocket Transport

- NET-7: The server MUST accept WebSocket connections for browser-based and modern clients.
- NET-8: The WebSocket transport MUST support TLS termination or operate behind a TLS-terminating reverse proxy.
- NET-9: The WebSocket transport MUST exchange messages using a defined JSON message schema.
- NET-10: The WebSocket transport MUST support the same game functionality as the Telnet transport.

## gRPC Transport

- NET-11: The server MUST expose a gRPC API defined via Protocol Buffer service definitions.
- NET-12: The gRPC API MUST support all game operations available through the Telnet and WebSocket transports.
- NET-13: The gRPC API MUST support bidirectional streaming for real-time game events.
- NET-14: The gRPC API MUST be designed to serve as the backend for dedicated GUI clients and web UIs.
- NET-15: The gRPC service definitions MUST be versioned and maintain backward compatibility within a major version.

## Transport Abstraction

- NET-16: The engine MUST define a transport-agnostic session interface that all three transports implement.
- NET-17: Game logic MUST NOT depend on the transport type; all transports MUST produce and consume the same internal message types.
- NET-18: Each transport MUST handle its own serialization/deserialization to and from the engine's internal message format.

## Connection Management

- NET-19: The server MUST support configurable idle timeouts per transport.
- NET-20: The server MUST support graceful disconnection with state persistence on unexpected connection loss.
- NET-21: The server MUST support connection rate limiting per source IP.
- NET-22: The server MUST log connection and disconnection events with timestamps and source addresses.
