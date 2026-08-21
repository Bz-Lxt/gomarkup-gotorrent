package util

import "testing"

func TestFormatBytesAndRate(t *testing.T) {
	if FormatBytes(500) != "500 B" {
		t.Errorf("500 B = %s", FormatBytes(500))
	}
	if FormatBytes(2048) != "2.00 KB" {
		t.Errorf("2048 = %s", FormatBytes(2048))
	}
	if FormatRate(1024) != "1.00 KB/s" {
		t.Errorf("rate = %s", FormatRate(1024))
	}
	if FormatBytes(-10) != "-10 B" {
		t.Errorf("负值 = %s", FormatBytes(-10))
	}
}

func TestPercent(t *testing.T) {
	if Percent(50, 100) != 50 {
		t.Errorf("50%% = %v", Percent(50, 100))
	}
	if Percent(1, 0) != 100 {
		t.Error("total=0 应返回 100")
	}
	if Percent(200, 100) != 100 {
		t.Error("超过 100 应截断")
	}
}

func TestHex20(t *testing.T) {
	var h [20]byte
	h[0], h[19] = 0xab, 0xcd
	s := EncodeHex20(h)
	got, err := DecodeHex20(s)
	if err != nil || got != h {
		t.Errorf("往返失败: %v %v", got, err)
	}
	if _, err := DecodeHex20("zz"); err == nil {
		t.Error("非法 hex 应失败")
	}
	if len(ShortHash(h)) != 16 {
		t.Errorf("ShortHash 长度 = %d", len(ShortHash(h)))
	}
}

func TestClamp(t *testing.T) {
	if ClampInt(5, 1, 3) != 3 || ClampInt(-1, 0, 2) != 0 {
		t.Error("ClampInt 错误")
	}
	if ClampInt64(9, 0, 4) != 4 {
		t.Error("ClampInt64 错误")
	}
}
