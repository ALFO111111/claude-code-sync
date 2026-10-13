// Package icon 程序图标：琥珀色圆角方块 + 两个首尾相接的白色循环箭头（"同步"的意象）。
//
// 用代码画而不是放图片：几何形状逐像素判断 + 4x4 超采样抗锯齿，任意尺寸都清晰，
// 托盘要的各种状态（同步中转圈、出错红点）也能直接生成。
//
//	tools/genicon  生成 exe 图标（build/windows/icon.ico、build/appicon.png）和界面 logo
//	internal/gui   运行时生成托盘图标
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

// Style 图标的变体。
type Style struct {
	Rotate float64 // 箭头顺时针旋转的角度（弧度），托盘"同步中"转圈动画用
	Badge  bool    // 右下角红点：上次同步出错
}

// 几何参数，都按边长 1 的比例算。
const (
	margin   = 0.03 // 方块离边缘的留白
	corner   = 0.22 // 圆角半径
	ringR    = 0.27 // 箭头所在圆的半径
	arrowA   = 0.55 // 箭头三角沿圆弧的长度（弧度）
	arcFrom  = -170 * math.Pi / 180
	arcTo    = -40 * math.Pi / 180 // 上半弧的终点，三角从这里开始
	badgeX   = 0.77
	badgeY   = 0.77
	badgeR   = 0.19
	badgeGap = 0.05 // 红点外圈镂空，和底下的图形隔开
)

// glyph 线宽相关的参数：小尺寸（托盘 16~24px）画粗一点，不然缩小后箭头尖看不出来。
type glyph struct {
	strokeW float64 // 箭头线宽
	arrowW  float64 // 箭头三角的半宽
}

func glyphFor(size int) glyph {
	if size <= 24 {
		return glyph{strokeW: 0.14, arrowW: 0.17}
	}
	return glyph{strokeW: 0.11, arrowW: 0.13}
}

var (
	bgTop    = rgb(0xf5, 0x9e, 0x0b)
	bgBottom = rgb(0xd9, 0x77, 0x06)
	white    = rgb(0xff, 0xff, 0xff)
	red      = rgb(0xdc, 0x26, 0x26)
)

type rgbf struct{ r, g, b float64 }

func rgb(r, g, b uint8) rgbf { return rgbf{float64(r), float64(g), float64(b)} }

// Render 画一张 size×size 的图标。
func Render(size int, s Style) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4 // 每个像素 4x4 个采样点
	n := float64(size)
	gl := glyphFor(size)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var r, g, b, a float64 // 预乘 alpha 累加
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) / n
					y := (float64(py) + (float64(sy)+0.5)/ss) / n
					c, ok := sample(x, y, s, gl)
					if ok {
						r, g, b, a = r+c.r, g+c.g, b+c.b, a+1
					}
				}
			}
			if a == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(r / a)), G: uint8(math.Round(g / a)), B: uint8(math.Round(b / a)),
				A: uint8(math.Round(a / ss / ss * 255)),
			})
		}
	}
	return img
}

// sample 单个采样点 (x, y)（0~1）的颜色；ok=false 表示透明。
func sample(x, y float64, s Style, g glyph) (rgbf, bool) {
	if s.Badge {
		d := math.Hypot(x-badgeX, y-badgeY)
		if d <= badgeR {
			return red, true
		}
		if d <= badgeR+badgeGap {
			return rgbf{}, false
		}
	}
	if !inRoundRect(x, y) {
		return rgbf{}, false
	}
	if g.contains(x, y, s.Rotate) {
		return white, true
	}
	t := (y - margin) / (1 - 2*margin)
	return rgbf{
		bgTop.r + (bgBottom.r-bgTop.r)*t,
		bgTop.g + (bgBottom.g-bgTop.g)*t,
		bgTop.b + (bgBottom.b-bgTop.b)*t,
	}, true
}

func inRoundRect(x, y float64) bool {
	const half = 0.5 - margin
	dx := math.Max(math.Abs(x-0.5)-half+corner, 0)
	dy := math.Max(math.Abs(y-0.5)-half+corner, 0)
	return math.Hypot(dx, dy) <= corner
}

// contains 两段圆弧箭头：上半段（左→右，箭头朝下）和它绕中心转 180° 的下半段。
func (g glyph) contains(x, y, rotate float64) bool {
	// 把采样点反向旋转，相当于把图形正向旋转
	dx, dy := x-0.5, y-0.5
	sin, cos := math.Sincos(-rotate)
	dx, dy = dx*cos-dy*sin, dx*sin+dy*cos
	return g.inArrow(dx, dy) || g.inArrow(-dx, -dy)
}

// inArrow 以圆心为原点，屏幕坐标（y 朝下，角度增大 = 顺时针）。
func (g glyph) inArrow(dx, dy float64) bool {
	d := math.Hypot(dx, dy)
	ang := math.Atan2(dy, dx)
	// 弧身
	if math.Abs(d-ringR) <= g.strokeW/2 && ang >= arcFrom && ang <= arcTo {
		return true
	}
	// 起点圆头
	sx, sy := ringR*math.Cos(arcFrom), ringR*math.Sin(arcFrom)
	if math.Hypot(dx-sx, dy-sy) <= g.strokeW/2 {
		return true
	}
	// 终点三角：底边沿半径方向，尖端落在圆上往前 arrowA 弧度处
	cx, cy := math.Cos(arcTo), math.Sin(arcTo)
	b1x, b1y := (ringR+g.arrowW)*cx, (ringR+g.arrowW)*cy
	b2x, b2y := (ringR-g.arrowW)*cx, (ringR-g.arrowW)*cy
	tx, ty := ringR*math.Cos(arcTo+arrowA), ringR*math.Sin(arcTo+arrowA)
	return inTriangle(dx, dy, b1x, b1y, b2x, b2y, tx, ty)
}

func inTriangle(px, py, ax, ay, bx, by, cx, cy float64) bool {
	cross := func(x1, y1, x2, y2, x3, y3 float64) float64 {
		return (x2-x1)*(y3-y1) - (y2-y1)*(x3-x1)
	}
	d1 := cross(ax, ay, bx, by, px, py)
	d2 := cross(bx, by, cx, cy, px, py)
	d3 := cross(cx, cy, ax, ay, px, py)
	neg := d1 < 0 || d2 < 0 || d3 < 0
	pos := d1 > 0 || d2 > 0 || d3 > 0
	return !(neg && pos)
}

// PNG 编码成 PNG。
func PNG(img image.Image) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // 写内存不会失败
	return buf.Bytes()
}

// ICO 生成包含多个尺寸的 .ico（每个尺寸内嵌 PNG，Vista 起支持）。
// Windows 会按显示场景（任务栏、托盘、高 DPI）挑最接近的尺寸，不用临时缩放所以不糊。
func ICO(sizes []int, s Style) []byte {
	pngs := make([][]byte, len(sizes))
	for i, n := range sizes {
		pngs[i] = PNG(Render(n, s))
	}
	var ico bytes.Buffer
	w := func(v any) { _ = binary.Write(&ico, binary.LittleEndian, v) }
	w(uint16(0))          // reserved
	w(uint16(1))          // type: icon
	w(uint16(len(sizes))) // count
	offset := 6 + 16*len(sizes)
	for i, n := range sizes {
		dim := uint8(n)
		if n >= 256 {
			dim = 0 // 0 表示 256
		}
		ico.WriteByte(dim) // width
		ico.WriteByte(dim) // height
		ico.WriteByte(0)   // palette
		ico.WriteByte(0)   // reserved
		w(uint16(1))       // planes
		w(uint16(32))      // bpp
		w(uint32(len(pngs[i])))
		w(uint32(offset))
		offset += len(pngs[i])
	}
	for _, p := range pngs {
		ico.Write(p)
	}
	return ico.Bytes()
}

// AppSizes exe 图标包含的尺寸（资源管理器、任务栏、Alt+Tab 各取所需）。
var AppSizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

// TraySizes 托盘图标包含的尺寸：托盘在 100%~200% 缩放下要 16~32，系统按 DPI 挑。
var TraySizes = []int{16, 20, 24, 32, 40, 48, 64}
