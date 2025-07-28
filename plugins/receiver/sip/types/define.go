package types

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type Connection struct {
	SrcIp     string
	SrcPort   int
	DstIp     string
	DstPort   int
	Protocol  string
	Direction Direction
}

type WithConnection interface {
	Connection() *Connection
}
