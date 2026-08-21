package wire

import (
	"bytes"
	"testing"
)

func TestHandshakeRoundTrip(t *testing.T) {
	h := &Handshake{InfoHash: [20]byte{1, 2, 3}, PeerID: [20]byte{9, 8, 7}}
	var buf bytes.Buffer
	if err := h.Write(&buf); err != nil {
		t.Fatal(err)
	}
	got, err := ReadHandshake(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.InfoHash != h.InfoHash || got.PeerID != h.PeerID {
		t.Error("握手往返不一致")
	}
}

func TestHandshakeWrongProtocol(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteByte(4)
	buf.WriteString("XXXX")
	buf.Write(make([]byte, 8+20+20))
	if _, err := ReadHandshake(&buf); err == nil {
		t.Error("协议串不匹配应报错")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	msgs := []*Message{
		{ID: MsgChoke},
		{ID: MsgUnchoke},
		{ID: MsgInterested},
		NewHave(42),
		NewBitfield([]byte{0b10100000}),
		NewRequest(1, 16384, 16384),
		NewPiece(2, 0, []byte("block-data")),
		KeepAlive(),
	}
	for _, m := range msgs {
		var buf bytes.Buffer
		if err := m.Write(&buf); err != nil {
			t.Fatal(err)
		}
		got, err := Read(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != m.ID || !bytes.Equal(got.Payload, m.Payload) {
			t.Errorf("消息往返不一致: %+v vs %+v", m, got)
		}
	}
}

func TestParseHelpers(t *testing.T) {
	idx, err := NewHave(7).ParseHave()
	if err != nil || idx != 7 {
		t.Errorf("ParseHave = %d, %v", idx, err)
	}
	i, b, l, err := NewRequest(1, 2, 3).ParseRequest()
	if err != nil || i != 1 || b != 2 || l != 3 {
		t.Errorf("ParseRequest = %d,%d,%d, %v", i, b, l, err)
	}
	i, b, blk, err := NewPiece(4, 8, []byte("xy")).ParsePiece()
	if err != nil || i != 4 || b != 8 || string(blk) != "xy" {
		t.Errorf("ParsePiece = %d,%d,%q, %v", i, b, blk, err)
	}
}

func TestReadOversized(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
	if _, err := Read(&buf); err == nil {
		t.Error("超大消息应报错")
	}
}

func TestExtendedAndCancel(t *testing.T) {
	m := NewExtended(ExtPexID, []byte("hi"))
	id, payload, err := m.ParseExtended()
	if err != nil || id != ExtPexID || string(payload) != "hi" {
		t.Errorf("ParseExtended = %d %q %v", id, payload, err)
	}
	c := NewCancel(1, 2, 3)
	i, b, l, err := c.ParseRequest()
	if err != nil || i != 1 || b != 2 || l != 3 {
		t.Errorf("Cancel 解析失败: %v", err)
	}
	if MsgPiece.String() != "piece" || MessageID(99).String() != "unknown(99)" {
		t.Error("MessageID.String 异常")
	}
}
