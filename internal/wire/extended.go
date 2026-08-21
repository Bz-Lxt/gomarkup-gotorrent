package wire

import (
	"encoding/binary"
	"fmt"
)

// MsgExtended 是 BitTorrent 扩展协议（BEP-10）的消息 ID。
const MsgExtended MessageID = 20

// 扩展消息子类型（GoTorrent 自定义编号，仅在本实现内互通）。
const (
	ExtHandshakeID = 0
	ExtPexID       = 1
)

// NewExtended 构造扩展消息：<extID:1><payload>。
func NewExtended(extID byte, payload []byte) *Message {
	p := make([]byte, 1+len(payload))
	p[0] = extID
	copy(p[1:], payload)
	return &Message{ID: MsgExtended, Payload: p}
}

// ParseExtended 解析扩展消息，返回子类型与负载。
func (m *Message) ParseExtended() (extID byte, payload []byte, err error) {
	if m.ID != MsgExtended {
		return 0, nil, fmt.Errorf("不是扩展消息")
	}
	if len(m.Payload) < 1 {
		return 0, nil, fmt.Errorf("扩展消息负载为空")
	}
	return m.Payload[0], m.Payload[1:], nil
}

// NewCancel 构造 Cancel 消息，格式与 Request 相同。
func NewCancel(index, begin, length int) *Message {
	p := make([]byte, 12)
	binary.BigEndian.PutUint32(p[0:4], uint32(index))
	binary.BigEndian.PutUint32(p[4:8], uint32(begin))
	binary.BigEndian.PutUint32(p[8:12], uint32(length))
	return &Message{ID: MsgCancel, Payload: p}
}

// String 返回消息类型名称，便于日志。
func (id MessageID) String() string {
	switch id {
	case MsgChoke:
		return "choke"
	case MsgUnchoke:
		return "unchoke"
	case MsgInterested:
		return "interested"
	case MsgNotInterested:
		return "not_interested"
	case MsgHave:
		return "have"
	case MsgBitfield:
		return "bitfield"
	case MsgRequest:
		return "request"
	case MsgPiece:
		return "piece"
	case MsgCancel:
		return "cancel"
	case MsgExtended:
		return "extended"
	case 255:
		return "keep_alive"
	default:
		return fmt.Sprintf("unknown(%d)", id)
	}
}
