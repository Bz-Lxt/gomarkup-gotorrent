package wire

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MessageID 消息类型。
type MessageID uint8

const (
	MsgChoke         MessageID = 0 // 拒绝对方的上传请求
	MsgUnchoke       MessageID = 1 // 允许对方向我们请求数据
	MsgInterested    MessageID = 2 // 对对方拥有的分片感兴趣
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4 // 告知对方我们拥有某分片
	MsgBitfield      MessageID = 5 // 握手后发送的完整位图
	MsgRequest       MessageID = 6 // 请求一个块: <index:4><begin:4><length:4>
	MsgPiece         MessageID = 7 // 返回块数据: <index:4><begin:4><block...>
	MsgCancel        MessageID = 8 // 取消请求
)

// MaxBlockSize 单次请求的最大块大小（16KB，与主流客户端一致）。
const MaxBlockSize = 16 * 1024

// MaxMessageSize 消息最大长度，防止恶意长度前缀耗尽内存。
const MaxMessageSize = 1 << 20 // 1MB

// Message 一条协议消息。ID 为 255 表示 keep-alive。
type Message struct {
	ID      MessageID
	Payload []byte
}

// KeepAlive 构造 keep-alive 消息。
func KeepAlive() *Message { return &Message{ID: 255} }

// NewHave 构造 Have 消息。
func NewHave(index int) *Message {
	p := make([]byte, 4)
	binary.BigEndian.PutUint32(p, uint32(index))
	return &Message{ID: MsgHave, Payload: p}
}

// NewBitfield 构造 Bitfield 消息。
func NewBitfield(bf []byte) *Message {
	return &Message{ID: MsgBitfield, Payload: bf}
}

// NewRequest 构造 Request 消息。
func NewRequest(index, begin, length int) *Message {
	p := make([]byte, 12)
	binary.BigEndian.PutUint32(p[0:4], uint32(index))
	binary.BigEndian.PutUint32(p[4:8], uint32(begin))
	binary.BigEndian.PutUint32(p[8:12], uint32(length))
	return &Message{ID: MsgRequest, Payload: p}
}

// NewPiece 构造 Piece 消息。
func NewPiece(index, begin int, block []byte) *Message {
	p := make([]byte, 8+len(block))
	binary.BigEndian.PutUint32(p[0:4], uint32(index))
	binary.BigEndian.PutUint32(p[4:8], uint32(begin))
	copy(p[8:], block)
	return &Message{ID: MsgPiece, Payload: p}
}

// ParseHave 解析 Have 消息的分片索引。
func (m *Message) ParseHave() (int, error) {
	if len(m.Payload) != 4 {
		return 0, fmt.Errorf("Have 负载长度非法: %d", len(m.Payload))
	}
	return int(binary.BigEndian.Uint32(m.Payload)), nil
}

// ParseRequest 解析 Request/Cancel 消息，返回 index、begin、length。
func (m *Message) ParseRequest() (index, begin, length int, err error) {
	if len(m.Payload) != 12 {
		return 0, 0, 0, fmt.Errorf("Request 负载长度非法: %d", len(m.Payload))
	}
	return int(binary.BigEndian.Uint32(m.Payload[0:4])),
		int(binary.BigEndian.Uint32(m.Payload[4:8])),
		int(binary.BigEndian.Uint32(m.Payload[8:12])), nil
}

// ParsePiece 解析 Piece 消息，返回 index、begin、block。
func (m *Message) ParsePiece() (index, begin int, block []byte, err error) {
	if len(m.Payload) < 8 {
		return 0, 0, nil, fmt.Errorf("Piece 负载长度非法: %d", len(m.Payload))
	}
	return int(binary.BigEndian.Uint32(m.Payload[0:4])),
		int(binary.BigEndian.Uint32(m.Payload[4:8])),
		m.Payload[8:], nil
}

// Write 将消息写入连接（自动添加长度前缀）。
func (m *Message) Write(w io.Writer) error {
	if m.ID == 255 { // keep-alive
		_, err := w.Write(make([]byte, 4))
		return err
	}
	buf := make([]byte, 5+len(m.Payload))
	binary.BigEndian.PutUint32(buf[0:4], uint32(1+len(m.Payload)))
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	_, err := w.Write(buf)
	return err
}

// Read 从连接读取一条消息（处理 keep-alive 与长度校验）。
func Read(r io.Reader) (*Message, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return KeepAlive(), nil
	}
	if length > MaxMessageSize {
		return nil, fmt.Errorf("消息过大: %d 字节", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("读取消息体失败: %w", err)
	}
	return &Message{ID: MessageID(body[0]), Payload: body[1:]}, nil
}
