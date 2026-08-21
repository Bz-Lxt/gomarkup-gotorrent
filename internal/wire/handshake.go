// Package wire 实现 Peer 之间的有线协议（简化版 BitTorrent 协议）。
//
// 握手格式（共 49 + len(pstr) 字节）：
//
//	<pstrlen:1><pstr><reserved:8><info_hash:20><peer_id:20>
//
// 消息格式：
//
//	<length:4 大端><id:1><payload>
//
// length 为 id + payload 的字节数；length=0 表示 keep-alive。
package wire

import (
	"fmt"
	"io"
)

// ProtocolStr 协议标识串。
const ProtocolStr = "GoTorrent protocol"

// Handshake 表示一次协议握手。
type Handshake struct {
	InfoHash [20]byte
	PeerID   [20]byte
}

// Write 将握手写入连接。
func (h *Handshake) Write(w io.Writer) error {
	buf := make([]byte, 0, 49+len(ProtocolStr))
	buf = append(buf, byte(len(ProtocolStr)))
	buf = append(buf, ProtocolStr...)
	buf = append(buf, make([]byte, 8)...) // reserved
	buf = append(buf, h.InfoHash[:]...)
	buf = append(buf, h.PeerID[:]...)
	_, err := w.Write(buf)
	return err
}

// ReadHandshake 从连接读取并校验握手。
func ReadHandshake(r io.Reader) (*Handshake, error) {
	lenBuf := make([]byte, 1)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, fmt.Errorf("读取 pstrlen 失败: %w", err)
	}
	pstrLen := int(lenBuf[0])
	if pstrLen == 0 || pstrLen > 64 {
		return nil, fmt.Errorf("非法 pstrlen: %d", pstrLen)
	}
	rest := make([]byte, pstrLen+8+20+20)
	if _, err := io.ReadFull(r, rest); err != nil {
		return nil, fmt.Errorf("读取握手体失败: %w", err)
	}
	if string(rest[:pstrLen]) != ProtocolStr {
		return nil, fmt.Errorf("协议串不匹配: %q", rest[:pstrLen])
	}
	h := &Handshake{}
	copy(h.InfoHash[:], rest[pstrLen+8:pstrLen+8+20])
	copy(h.PeerID[:], rest[pstrLen+8+20:])
	return h, nil
}
