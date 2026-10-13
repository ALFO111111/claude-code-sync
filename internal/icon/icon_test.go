package icon

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
)

// 角上透明、中心是白色箭头圈里的底色、出错版右下角是红点。
func TestRender(t *testing.T) {
	img := Render(64, Style{})
	if a := img.NRGBAAt(0, 0).A; a != 0 {
		t.Errorf("左上角应透明，alpha=%d", a)
	}
	if c := img.NRGBAAt(32, 32); c.A != 255 || c.R < 200 || c.B > 50 {
		t.Errorf("中心应是不透明的琥珀色底，得到 %+v", c)
	}
	bad := Render(64, Style{Badge: true})
	if c := bad.NRGBAAt(49, 49); c.R < 200 || c.G > 80 {
		t.Errorf("右下角应是红点，得到 %+v", c)
	}
}

// ICO 目录项数量、尺寸、偏移都对，每项都是能解码的 PNG。
func TestICO(t *testing.T) {
	data := ICO(AppSizes, Style{})
	if n := binary.LittleEndian.Uint16(data[4:]); int(n) != len(AppSizes) {
		t.Fatalf("图标数 %d，应为 %d", n, len(AppSizes))
	}
	for i, want := range AppSizes {
		e := data[6+16*i:]
		size := binary.LittleEndian.Uint32(e[8:])
		off := binary.LittleEndian.Uint32(e[12:])
		img, err := png.Decode(bytes.NewReader(data[off : off+size]))
		if err != nil {
			t.Fatalf("第 %d 项解码失败: %v", i, err)
		}
		if got := img.Bounds().Dx(); got != want {
			t.Errorf("第 %d 项尺寸 %d，应为 %d", i, got, want)
		}
	}
}
