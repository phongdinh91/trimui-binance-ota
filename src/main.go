package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// appVersion controls OTA releases: increasing it on main starts verified auto-publish.
const (
	appVersion          = "v0.31"
	cacheSaveInterval   = 2 * time.Minute
	defaultRefreshIndex = 1

	EV_KEY       = 0x01
	EV_ABS       = 0x03
	ABS_HAT0X    = 0x10
	ABS_HAT0Y    = 0x11
	BTN_SOUTH    = 0x130 // physical B on Brick layout
	BTN_EAST     = 0x131 // physical A on Brick layout
	BTN_NORTH    = 0x133 // Brick Pro: physical Y
	BTN_WEST     = 0x134 // Brick Pro: physical X
	BTN_TL       = 0x136 // Brick Pro: physical L1
	BTN_TR       = 0x137 // Brick Pro: physical R1
	BTN_SELECT   = 0x13a
	BTN_START    = 0x13b
	BTN_MODE     = 0x13c
	KEY_ESC      = 1
	KEY_UP       = 103
	KEY_LEFT     = 105
	KEY_RIGHT    = 106
	KEY_DOWN     = 108
	KEY_HOMEPAGE = 172
)

var refreshOptions = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second, 60 * time.Second}

type bitfield struct{ Offset, Length, MsbRight uint32 }
type fbVarScreeninfo struct {
	Xres, Yres, XresVirtual, YresVirtual, Xoffset, Yoffset uint32
	BitsPerPixel, Grayscale                                uint32
	Red, Green, Blue, Transp                               bitfield
	Nonstd, Activate, Height, Width, AccelFlags, Pixclock  uint32
	LeftMargin, RightMargin, UpperMargin, LowerMargin      uint32
	HsyncLen, VsyncLen, Sync, Vmode, Rotate, Colorspace    uint32
	Reserved                                               [4]uint32
}
type fbFixScreeninfo struct {
	ID           [16]byte
	SmemStart    uint64
	SmemLen      uint32
	Type         uint32
	TypeAux      uint32
	Visual       uint32
	XPanStep     uint16
	YPanStep     uint16
	YWrapStep    uint16
	_pad         uint16
	LineLength   uint32
	MmioStart    uint64
	MmioLen      uint32
	Accel        uint32
	Capabilities uint16
	Reserved     [2]uint16
}

type framebuffer struct {
	f                 *os.File
	mapped            []byte // mmap thuc te cua /dev/fb0
	data              []byte // back buffer de tranh nhay/tearing khi ve lai
	v                 fbVarScreeninfo
	fix               fbFixScreeninfo
	w, h, stride, bpp int
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if e != 0 {
		return e
	}
	return nil
}

func openFramebuffer() (*framebuffer, error) {
	f, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	fb := &framebuffer{f: f}
	if err := ioctl(f.Fd(), 0x4600, unsafe.Pointer(&fb.v)); err != nil {
		f.Close()
		return nil, err
	}
	_ = ioctl(f.Fd(), 0x4602, unsafe.Pointer(&fb.fix))
	fb.w, fb.h, fb.bpp = int(fb.v.Xres), int(fb.v.Yres), int(fb.v.BitsPerPixel)
	if fb.w <= 0 || fb.h <= 0 || (fb.bpp != 16 && fb.bpp != 24 && fb.bpp != 32) {
		f.Close()
		return nil, fmt.Errorf("framebuffer khong ho tro: %dx%d %dbpp", fb.w, fb.h, fb.bpp)
	}
	fb.stride = int(fb.fix.LineLength)
	if fb.stride <= 0 {
		fb.stride = int(fb.v.XresVirtual) * fb.bpp / 8
	}
	if fb.stride <= 0 {
		fb.stride = fb.w * fb.bpp / 8
	}
	mapLen := fb.stride * int(fb.v.YresVirtual)
	if mapLen <= 0 {
		mapLen = fb.stride * fb.h
	}
	if fb.fix.SmemLen > 0 && int(fb.fix.SmemLen) < mapLen {
		mapLen = int(fb.fix.SmemLen)
	}
	fb.mapped, err = syscall.Mmap(int(f.Fd()), 0, mapLen, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, err
	}
	// Ve vao RAM truoc, sau do chi copy cac scanline thay doi sang framebuffer that.
	// Cach nay giam nhay man hinh rat ro khi chi di chuyen highlight bang D-pad.
	fb.data = make([]byte, len(fb.mapped))
	copy(fb.data, fb.mapped)
	return fb, nil
}
func (fb *framebuffer) flush() {
	if len(fb.mapped) == 0 || len(fb.data) == 0 {
		return
	}
	rows := fb.h
	if rows*fb.stride > len(fb.data) {
		rows = len(fb.data) / fb.stride
	}
	for y := 0; y < rows; y++ {
		off := y * fb.stride
		end := off + fb.stride
		if end > len(fb.data) || end > len(fb.mapped) {
			break
		}
		if !bytes.Equal(fb.data[off:end], fb.mapped[off:end]) {
			copy(fb.mapped[off:end], fb.data[off:end])
		}
	}
}
func (fb *framebuffer) close() {
	if fb.mapped != nil {
		_ = syscall.Munmap(fb.mapped)
	}
	if fb.f != nil {
		_ = fb.f.Close()
	}
}

type color struct{ r, g, b uint8 }

var (
	cBg     = color{15, 18, 23}
	cPanel  = color{25, 29, 36}
	cPanel2 = color{34, 39, 48}
	cText   = color{239, 242, 246}
	cMuted  = color{156, 163, 175}
	cYellow = color{240, 185, 11}
	cGreen  = color{22, 199, 132}
	cRed    = color{234, 57, 67}
	cBlue   = color{61, 130, 246}
)

func scaleBits(v uint8, n uint32) uint32 {
	if n == 0 {
		return 0
	}
	max := uint32((1 << n) - 1)
	return uint32(v) * max / 255
}
func (fb *framebuffer) putPixel(x, y int, c color) {
	if x < 0 || y < 0 || x >= fb.w || y >= fb.h {
		return
	}
	off := y*fb.stride + x*fb.bpp/8
	if off < 0 || off+fb.bpp/8 > len(fb.data) {
		return
	}
	rv := scaleBits(c.r, fb.v.Red.Length) << fb.v.Red.Offset
	gv := scaleBits(c.g, fb.v.Green.Length) << fb.v.Green.Offset
	bv := scaleBits(c.b, fb.v.Blue.Length) << fb.v.Blue.Offset
	av := uint32(0)
	if fb.v.Transp.Length > 0 {
		av = ((1 << fb.v.Transp.Length) - 1) << fb.v.Transp.Offset
	}
	p := rv | gv | bv | av
	switch fb.bpp {
	case 16:
		binary.LittleEndian.PutUint16(fb.data[off:off+2], uint16(p))
	case 24:
		fb.data[off] = byte(p)
		fb.data[off+1] = byte(p >> 8)
		fb.data[off+2] = byte(p >> 16)
	case 32:
		binary.LittleEndian.PutUint32(fb.data[off:off+4], p)
	}
}
func (fb *framebuffer) fill(c color) { fb.rect(0, 0, fb.w, fb.h, c) }
func (fb *framebuffer) rect(x, y, w, h int, c color) {
	if w <= 0 || h <= 0 {
		return
	}
	x0 := x
	y0 := y
	x1 := x + w
	y1 := y + h
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > fb.w {
		x1 = fb.w
	}
	if y1 > fb.h {
		y1 = fb.h
	}
	for yy := y0; yy < y1; yy++ {
		for xx := x0; xx < x1; xx++ {
			fb.putPixel(xx, yy, c)
		}
	}
}
func (fb *framebuffer) hline(x, y, w int, c color) { fb.rect(x, y, w, 1, c) }
func (fb *framebuffer) line(x0, y0, x1, y1 int, c color) {
	dx := int(math.Abs(float64(x1 - x0)))
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	dy := -int(math.Abs(float64(y1 - y0)))
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		fb.putPixel(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}
func drawFavoriteMark(fb *framebuffer, x, y, size int, on bool) {
	c := cMuted
	if on {
		c = cYellow
	}
	if size < 8 {
		size = 8
	}
	cx := float64(x + size/2)
	cy := float64(y + size/2)
	rOuter := float64(size) * 0.48
	rInner := rOuter * 0.43
	pts := make([][2]int, 10)
	for i := 0; i < 10; i++ {
		r := rOuter
		if i%2 == 1 {
			r = rInner
		}
		a := -math.Pi/2 + float64(i)*math.Pi/5
		pts[i] = [2]int{int(cx + math.Cos(a)*r), int(cy + math.Sin(a)*r)}
	}
	for i := 0; i < 10; i++ {
		j := (i + 1) % 10
		fb.line(pts[i][0], pts[i][1], pts[j][0], pts[j][1], c)
	}
	if on {
		for _, pt := range pts {
			fb.line(int(cx), int(cy), pt[0], pt[1], c)
		}
	}
}

var font5x7 = map[rune][7]byte{
	'0': {14, 17, 19, 21, 25, 17, 14}, '1': {4, 12, 4, 4, 4, 4, 14}, '2': {14, 17, 1, 2, 4, 8, 31}, '3': {30, 1, 1, 14, 1, 1, 30}, '4': {2, 6, 10, 18, 31, 2, 2}, '5': {31, 16, 16, 30, 1, 1, 30}, '6': {6, 8, 16, 30, 17, 17, 14}, '7': {31, 1, 2, 4, 8, 8, 8}, '8': {14, 17, 17, 14, 17, 17, 14}, '9': {14, 17, 17, 15, 1, 2, 12},
	'A': {14, 17, 17, 31, 17, 17, 17}, 'B': {30, 17, 17, 30, 17, 17, 30}, 'C': {14, 17, 16, 16, 16, 17, 14}, 'D': {30, 17, 17, 17, 17, 17, 30}, 'E': {31, 16, 16, 30, 16, 16, 31}, 'F': {31, 16, 16, 30, 16, 16, 16}, 'G': {14, 17, 16, 23, 17, 17, 15}, 'H': {17, 17, 17, 31, 17, 17, 17}, 'I': {14, 4, 4, 4, 4, 4, 14}, 'J': {7, 2, 2, 2, 2, 18, 12}, 'K': {17, 18, 20, 24, 20, 18, 17}, 'L': {16, 16, 16, 16, 16, 16, 31}, 'M': {17, 27, 21, 21, 17, 17, 17}, 'N': {17, 25, 21, 19, 17, 17, 17}, 'O': {14, 17, 17, 17, 17, 17, 14}, 'P': {30, 17, 17, 30, 16, 16, 16}, 'Q': {14, 17, 17, 17, 21, 18, 13}, 'R': {30, 17, 17, 30, 20, 18, 17}, 'S': {15, 16, 16, 14, 1, 1, 30}, 'T': {31, 4, 4, 4, 4, 4, 4}, 'U': {17, 17, 17, 17, 17, 17, 14}, 'V': {17, 17, 17, 17, 17, 10, 4}, 'W': {17, 17, 17, 21, 21, 21, 10}, 'X': {17, 17, 10, 4, 10, 17, 17}, 'Y': {17, 17, 10, 4, 4, 4, 4}, 'Z': {31, 1, 2, 4, 8, 16, 31},
	'.': {0, 0, 0, 0, 0, 4, 4}, ',': {0, 0, 0, 0, 4, 4, 8}, ':': {0, 4, 4, 0, 4, 4, 0}, '-': {0, 0, 0, 31, 0, 0, 0}, '+': {0, 4, 4, 31, 4, 4, 0}, '%': {17, 2, 4, 8, 16, 17, 0}, '/': {1, 2, 4, 8, 16, 0, 0}, '(': {2, 4, 8, 8, 8, 4, 2}, ')': {8, 4, 2, 2, 2, 4, 8}, ' ': {0, 0, 0, 0, 0, 0, 0}, '_': {0, 0, 0, 0, 0, 0, 31}, '|': {4, 4, 4, 4, 4, 4, 4}, '$': {4, 15, 20, 14, 5, 30, 4}, '#': {10, 31, 10, 10, 31, 10, 0},
}

type vnGlyph struct {
	Base           rune
	Modifier, Tone string
}

func vnDecompose(r rune) (vnGlyph, bool) {
	m := map[rune]vnGlyph{
		'À': {'A', "", "grave"}, 'Á': {'A', "", "acute"}, 'Ả': {'A', "", "hook"}, 'Ã': {'A', "", "tilde"}, 'Ạ': {'A', "", "dot"},
		'Ă': {'A', "breve", ""}, 'Ằ': {'A', "breve", "grave"}, 'Ắ': {'A', "breve", "acute"}, 'Ẳ': {'A', "breve", "hook"}, 'Ẵ': {'A', "breve", "tilde"}, 'Ặ': {'A', "breve", "dot"},
		'Â': {'A', "circ", ""}, 'Ầ': {'A', "circ", "grave"}, 'Ấ': {'A', "circ", "acute"}, 'Ẩ': {'A', "circ", "hook"}, 'Ẫ': {'A', "circ", "tilde"}, 'Ậ': {'A', "circ", "dot"},
		'È': {'E', "", "grave"}, 'É': {'E', "", "acute"}, 'Ẻ': {'E', "", "hook"}, 'Ẽ': {'E', "", "tilde"}, 'Ẹ': {'E', "", "dot"},
		'Ê': {'E', "circ", ""}, 'Ề': {'E', "circ", "grave"}, 'Ế': {'E', "circ", "acute"}, 'Ể': {'E', "circ", "hook"}, 'Ễ': {'E', "circ", "tilde"}, 'Ệ': {'E', "circ", "dot"},
		'Ì': {'I', "", "grave"}, 'Í': {'I', "", "acute"}, 'Ỉ': {'I', "", "hook"}, 'Ĩ': {'I', "", "tilde"}, 'Ị': {'I', "", "dot"},
		'Ò': {'O', "", "grave"}, 'Ó': {'O', "", "acute"}, 'Ỏ': {'O', "", "hook"}, 'Õ': {'O', "", "tilde"}, 'Ọ': {'O', "", "dot"},
		'Ô': {'O', "circ", ""}, 'Ồ': {'O', "circ", "grave"}, 'Ố': {'O', "circ", "acute"}, 'Ổ': {'O', "circ", "hook"}, 'Ỗ': {'O', "circ", "tilde"}, 'Ộ': {'O', "circ", "dot"},
		'Ơ': {'O', "horn", ""}, 'Ờ': {'O', "horn", "grave"}, 'Ớ': {'O', "horn", "acute"}, 'Ở': {'O', "horn", "hook"}, 'Ỡ': {'O', "horn", "tilde"}, 'Ợ': {'O', "horn", "dot"},
		'Ù': {'U', "", "grave"}, 'Ú': {'U', "", "acute"}, 'Ủ': {'U', "", "hook"}, 'Ũ': {'U', "", "tilde"}, 'Ụ': {'U', "", "dot"},
		'Ư': {'U', "horn", ""}, 'Ừ': {'U', "horn", "grave"}, 'Ứ': {'U', "horn", "acute"}, 'Ử': {'U', "horn", "hook"}, 'Ữ': {'U', "horn", "tilde"}, 'Ự': {'U', "horn", "dot"},
		'Ỳ': {'Y', "", "grave"}, 'Ý': {'Y', "", "acute"}, 'Ỷ': {'Y', "", "hook"}, 'Ỹ': {'Y', "", "tilde"}, 'Ỵ': {'Y', "", "dot"},
		'Đ': {'D', "bar", ""},
	}
	g, ok := m[r]
	return g, ok
}

func drawASCII(fb *framebuffer, x, y, scale int, s string, c color) {
	if scale < 1 {
		scale = 1
	}
	cx := x
	for _, rr := range strings.ToUpper(s) {
		base := rr
		mod, tone := "", ""
		if vg, ok := vnDecompose(rr); ok {
			base, mod, tone = vg.Base, vg.Modifier, vg.Tone
		}
		g, ok := font5x7[base]
		if !ok {
			g = font5x7[' ']
		}
		baseY := y + scale // chừa một hàng nhỏ cho dấu phía trên
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if (g[row] & (1 << (4 - col))) != 0 {
					fb.rect(cx+col*scale, baseY+row*scale, scale, scale, c)
				}
			}
		}
		px := func(col, row int) { fb.rect(cx+col*scale, y+row*scale, scale, scale, c) }
		switch mod {
		case "circ":
			px(1, 0)
			px(2, 0)
			px(3, 0)
		case "breve":
			px(1, 0)
			px(3, 0)
			px(2, 0)
		case "horn":
			px(4, 0)
		case "bar":
			fb.rect(cx, baseY+3*scale, 5*scale, scale, c)
		}
		switch tone {
		case "acute":
			px(3, 0)
			px(4, 0)
		case "grave":
			px(0, 0)
			px(1, 0)
		case "hook":
			px(2, 0)
			px(3, 0)
		case "tilde":
			px(0, 0)
			px(2, 0)
			px(4, 0)
		case "dot":
			px(2, 8)
		}
		cx += 6 * scale
	}
}

func asciiWidth(scale int, s string) int { return len([]rune(s)) * 6 * scale }

// drawASCIIRatio vẽ font 5x7 với đơn vị pixel dạng phân số. Ví dụ 9/2 = 4.5px.
func drawASCIIRatio(fb *framebuffer, x, y, num, den int, s string, c color) {
	if num < 1 {
		num = 1
	}
	if den < 1 {
		den = 1
	}
	pos := func(n int) int { return n * num / den }
	cellRect := func(x0, y0, x1, y1 int) {
		w, h := x1-x0, y1-y0
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		fb.rect(x0, y0, w, h, c)
	}
	cx := x
	for _, rr := range strings.ToUpper(s) {
		base := rr
		mod, tone := "", ""
		if vg, ok := vnDecompose(rr); ok {
			base, mod, tone = vg.Base, vg.Modifier, vg.Tone
		}
		g, ok := font5x7[base]
		if !ok {
			g = font5x7[' ']
		}
		baseY := y + pos(1)
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if (g[row] & (1 << (4 - col))) != 0 {
					cellRect(cx+pos(col), baseY+pos(row), cx+pos(col+1), baseY+pos(row+1))
				}
			}
		}
		px := func(col, row int) { cellRect(cx+pos(col), y+pos(row), cx+pos(col+1), y+pos(row+1)) }
		switch mod {
		case "circ":
			px(1, 0)
			px(2, 0)
			px(3, 0)
		case "breve":
			px(1, 0)
			px(3, 0)
			px(2, 0)
		case "horn":
			px(4, 0)
		case "bar":
			cellRect(cx, baseY+pos(3), cx+pos(5), baseY+pos(4))
		}
		switch tone {
		case "acute":
			px(3, 0)
			px(4, 0)
		case "grave":
			px(0, 0)
			px(1, 0)
		case "hook":
			px(2, 0)
			px(3, 0)
		case "tilde":
			px(0, 0)
			px(2, 0)
			px(4, 0)
		case "dot":
			px(2, 8)
		}
		cx += pos(6)
	}
}

func asciiWidthRatio(num, den int, s string) int {
	if den < 1 {
		den = 1
	}
	return len([]rune(s)) * (6 * num / den)
}

func appDir() string {
	p, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(p)
}

type otaConfig struct {
	ManifestURL  string `json:"manifest_url"`
	CheckOnStart bool   `json:"check_on_start"`
}

type otaManifest struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Notes   string `json:"notes"`
}

type otaCheckResult struct {
	Manifest otaManifest
	Err      error
}

type otaUpdateResult struct {
	Version string
	Err     error
}

func loadOTAConfig() otaConfig {
	cfg := otaConfig{CheckOnStart: true}
	b, err := os.ReadFile(filepath.Join(appDir(), "ota.json"))
	if err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_OTA_MANIFEST")); v != "" {
		cfg.ManifestURL = v
	}
	return cfg
}

func versionNumbers(v string) []int {
	v = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "v"))
	parts := strings.FieldsFunc(v, func(r rune) bool { return r < '0' || r > '9' })
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func isNewerVersion(remote, local string) bool {
	r := versionNumbers(remote)
	l := versionNumbers(local)
	n := len(r)
	if len(l) > n {
		n = len(l)
	}
	for i := 0; i < n; i++ {
		rv, lv := 0, 0
		if i < len(r) {
			rv = r[i]
		}
		if i < len(l) {
			lv = l[i]
		}
		if rv != lv {
			return rv > lv
		}
	}
	return false
}

func fetchOTAManifest(client *http.Client, manifestURL string) (otaManifest, error) {
	manifestURL = strings.TrimSpace(manifestURL)
	if manifestURL == "" {
		return otaManifest{}, errors.New("OTA chưa cấu hình URL")
	}
	req, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return otaManifest{}, err
	}
	req.Header.Set("User-Agent", "Binance-TrimUI/"+appVersion)
	resp, err := client.Do(req)
	if err != nil {
		return otaManifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return otaManifest{}, fmt.Errorf("OTA manifest HTTP %d", resp.StatusCode)
	}
	var m otaManifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 128<<10)).Decode(&m); err != nil {
		return otaManifest{}, err
	}
	m.Version = strings.TrimSpace(m.Version)
	m.URL = strings.TrimSpace(m.URL)
	m.SHA256 = strings.ToLower(strings.TrimSpace(m.SHA256))
	if m.Version == "" || m.URL == "" || len(m.SHA256) != 64 {
		return otaManifest{}, errors.New("OTA manifest không hợp lệ")
	}
	if _, err := hex.DecodeString(m.SHA256); err != nil {
		return otaManifest{}, errors.New("OTA SHA-256 không hợp lệ")
	}
	return m, nil
}

// Backward-compatible entry point; the worker uses a progress callback.
func downloadOTA(client *http.Client, m otaManifest, dst string) error {
 return downloadOTAWithProgress(client,m,dst,nil)
}

func otaRelativeName(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	needle := "BinanceGia.pak/"
	idx := strings.Index(name, needle)
	if idx < 0 {
		return "", false
	}
	rel := strings.TrimPrefix(name[idx+len(needle):], "/")
	if rel == "" {
		return "", false
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return clean, true
}

func extractOTA(zipPath, stageDir string) error {return extractOTAWithProgress(zipPath,stageDir,nil)}

func extractOTAWithProgress(zipPath, stageDir string, progress func(otaProgressEvent)) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
 totalFiles:=int64(0)
 for _,zf:=range zr.File {if _,ok:=otaRelativeName(zf.Name);ok&&!zf.FileInfo().IsDir(){totalFiles++}}
 doneFiles:=int64(0)
 reportOTA(progress,"extract",0,totalFiles,"")
	for _, zf := range zr.File {
		rel, ok := otaRelativeName(zf.Name)
		if !ok {
			continue
		}
		if zf.FileInfo().Mode()&os.ModeSymlink != 0 {
			return errors.New("OTA không cho phép symlink")
		}
		dst := filepath.Join(stageDir, rel)
		if !strings.HasPrefix(dst, stageDir+string(os.PathSeparator)) && dst != stageDir {
			return errors.New("OTA ZIP path không an toàn")
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0755); err != nil {
				return err
			}
			continue
		}
		if zf.UncompressedSize64>uint64(otaMaxFileSize) {
            return fmt.Errorf("OTA file qua lon: %s",rel)
        }
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		mode := zf.Mode().Perm()
		if mode == 0 {
			mode = 0644
		}
		if rel == "binance-gia" || rel == "launch.sh" {
			mode = 0755
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			rc.Close()
			return err
		}
		n, cpErr := io.Copy(out, io.LimitReader(rc, otaMaxFileSize+1))
		cl1, cl2 := rc.Close(), out.Close()
		if cpErr != nil {return cpErr}
        if n>otaMaxFileSize || uint64(n)!=zf.UncompressedSize64 {
            return fmt.Errorf("OTA file bi cat/qua lon: %s",rel)
        }
		if cl1 != nil {
			return cl1
		}
		if cl2 != nil {
			return cl2
		}
        doneFiles++
        reportOTA(progress,"extract",doneFiles,totalFiles,rel)
	}
	for _, req := range []string{"binance-gia", "launch.sh", "config.json"} {
		if st, err := os.Stat(filepath.Join(stageDir, req)); err != nil || st.IsDir() {
			return fmt.Errorf("OTA thiếu %s", req)
		}
	}
	return nil
}

func copyFileAtomic(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp := dst + ".ota-new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		_ = os.Remove(tmp)
		return cpErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func persistentOTAFile(rel string) bool {
	base := filepath.Base(rel)
	switch base {
	case "favorites.json", "settings.json", "market-cache.json", "ota.json", "binance-gia.log":
		return true
	default:
		return false
	}
}

func applyOTA(stageDir string) error {return applyOTAWithProgress(stageDir,nil)}

func applyOTAWithProgress(stageDir string, progress func(otaProgressEvent)) error {
	root := appDir()
	parent := filepath.Dir(root)
	backup, err := os.MkdirTemp(parent, ".binance-ota-backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(backup)
	var files []string
	err = filepath.Walk(stageDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stageDir, path)
		if err != nil {
			return err
		}
		if persistentOTAFile(rel) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return err
	}
	reportOTA(progress,"install",0,int64(len(files)),"")
    completed:=int64(0)
	backed := make(map[string]bool)
	created := make(map[string]bool)
	rollback := func() {
		for rel := range created {
			_ = os.Remove(filepath.Join(root, rel))
		}
		for rel := range backed {
			b := filepath.Join(backup, rel)
			d := filepath.Join(root, rel)
			if st, e := os.Stat(b); e == nil {
				_ = copyFileAtomic(b, d, st.Mode().Perm())
			}
		}
	}
	for _, rel := range files {
		src := filepath.Join(stageDir, rel)
		dst := filepath.Join(root, rel)
		st, err := os.Stat(src)
		if err != nil {
			rollback()
			return err
		}
		if old, err := os.Stat(dst); err == nil && !old.IsDir() {
			b := filepath.Join(backup, rel)
			if err := copyFileAtomic(dst, b, old.Mode().Perm()); err != nil {
				rollback()
				return err
			}
			backed[rel] = true
		} else if os.IsNotExist(err) {
			created[rel] = true
		}
		mode := st.Mode().Perm()
		if rel == "binance-gia" || rel == "launch.sh" {
			mode = 0755
		}
		if err := copyFileAtomic(src, dst, mode); err != nil {
			rollback()
			return err
		}
        completed++
        reportOTA(progress,"install",completed,int64(len(files)),rel)
	}
	return nil
}

func performOTA(client *http.Client, m otaManifest) error {return performOTAWithProgress(client,m,nil)}

func performOTAWithProgress(client *http.Client,m otaManifest, progress func(otaProgressEvent)) error {
	parent := filepath.Dir(appDir())
	work, err := os.MkdirTemp(parent, ".binance-ota-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	zipPath := filepath.Join(work, "update.zip")
	stage := filepath.Join(work, "stage")
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	if err := downloadOTAWithProgress(client, m, zipPath,progress); err != nil {
		return err
	}
	if err := extractOTAWithProgress(zipPath, stage,progress); err != nil {
		return err
	}
	if err:=applyOTAWithProgress(stage,progress);err!=nil{return err}
 reportOTA(progress,"done",1,1,"")
 return nil
}

type asset struct{ img image.Image }

func loadAsset(name string) *asset {
	f, err := os.Open(filepath.Join(appDir(), "assets", name))
	if err != nil {
		return nil
	}
	defer f.Close()
	im, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return &asset{img: im}
}
func drawAsset(fb *framebuffer, a *asset, x, y int) {
	if a == nil {
		return
	}
	b := a.img.Bounds()
	for yy := b.Min.Y; yy < b.Max.Y; yy++ {
		for xx := b.Min.X; xx < b.Max.X; xx++ {
			r, g, bv, aa := a.img.At(xx, yy).RGBA()
			if aa < 0x1000 {
				continue
			}
			fb.putPixel(x+xx-b.Min.X, y+yy-b.Min.Y, color{uint8(r >> 8), uint8(g >> 8), uint8(bv >> 8)})
		}
	}
}

type tickerRaw struct {
	Symbol             string `json:"symbol"`
	LastPrice          string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
	Count              int64  `json:"count"`
}
type ticker struct {
	Symbol      string  `json:"symbol"`
	BaseAsset   string  `json:"baseAsset,omitempty"`
	QuoteAsset  string  `json:"quoteAsset,omitempty"`
	Price       float64 `json:"price"`
	Change      float64 `json:"change"`
	High        float64 `json:"high"`
	Low         float64 `json:"low"`
	Volume      float64 `json:"volume"`
	QuoteVolume float64 `json:"quoteVolume"`
	Count       int64   `json:"count"`
}
func num(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }

type pairInfo struct {
	Base  string
	Quote string
}

type exchangeInfoResponse struct {
	Symbols []struct {
		Symbol               string   `json:"symbol"`
		Status               string   `json:"status"`
		BaseAsset            string   `json:"baseAsset"`
		QuoteAsset           string   `json:"quoteAsset"`
		IsSpotTradingAllowed bool     `json:"isSpotTradingAllowed"`
		Permissions          []string `json:"permissions"`
	} `json:"symbols"`
}

var pairInfoCache struct {
	sync.Mutex
	at    time.Time
	pairs map[string]pairInfo
}

func isSpotSymbol(status string, allowed bool, permissions []string) bool {
	if status != "TRADING" {
		return false
	}
	if allowed {
		return true
	}
	for _, p := range permissions {
		if p == "SPOT" {
			return true
		}
	}
	return false
}

func fetchPairInfo(client *http.Client) (map[string]pairInfo, error) {
	pairInfoCache.Lock()
	if len(pairInfoCache.pairs) > 0 && time.Since(pairInfoCache.at) < 6*time.Hour {
		out := pairInfoCache.pairs
		pairInfoCache.Unlock()
		return out, nil
	}
	pairInfoCache.Unlock()

	bases := []string{
		"https://data-api.binance.vision",
		"https://api.binance.com",
		"https://api1.binance.com",
	}
	var lastErr error
	for _, base := range bases {
		// showPermissionSets=false makes the response much smaller on current Binance Spot API.
		// If a mirror does not support the parameter, retry the plain endpoint.
		paths := []string{"/api/v3/exchangeInfo?showPermissionSets=false", "/api/v3/exchangeInfo"}
		for _, path := range paths {
			req, _ := http.NewRequest("GET", base+path, nil)
			req.Header.Set("User-Agent", "BinanceGia-TrimUI/0.11")
			resp, err := client.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			if resp.StatusCode != 200 {
				io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				continue
			}
			var info exchangeInfoResponse
			err = json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(&info)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				continue
			}
			pairs := make(map[string]pairInfo, len(info.Symbols))
			for _, x := range info.Symbols {
				if !isSpotSymbol(x.Status, x.IsSpotTradingAllowed, x.Permissions) || x.Symbol == "" || x.BaseAsset == "" || x.QuoteAsset == "" {
					continue
				}
				pairs[x.Symbol] = pairInfo{Base: x.BaseAsset, Quote: x.QuoteAsset}
			}
			if len(pairs) == 0 {
				lastErr = errors.New("không có cặp Spot đang giao dịch")
				continue
			}
			pairInfoCache.Lock()
			pairInfoCache.pairs = pairs
			pairInfoCache.at = time.Now()
			pairInfoCache.Unlock()
			return pairs, nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New("không tải được danh sách cặp Spot")
	}
	return nil, lastErr
}

func normalizePairFields(t ticker) ticker {
	if t.BaseAsset != "" && t.QuoteAsset != "" {
		return t
	}
	// Fallback cho cache cũ hoặc khi exchangeInfo chưa tải xong.
	pi := inferPairFromSymbol(t.Symbol)
	if t.BaseAsset == "" {
		t.BaseAsset = pi.Base
	}
	if t.QuoteAsset == "" {
		t.QuoteAsset = pi.Quote
	}
	return t
}

func pairLabel(t ticker) string {
	t = normalizePairFields(t)
	if t.BaseAsset != "" && t.QuoteAsset != "" {
		return t.BaseAsset + "/" + t.QuoteAsset
	}
	return t.Symbol
}

// inferPairFromSymbol is only a fallback for when exchangeInfo cannot be loaded.
// Search still works from the raw Binance Spot symbol even if no split is found.
func inferPairFromSymbol(symbol string) pairInfo {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	quotes := []string{
		"FDUSD", "USDT", "USDC", "TUSD", "BUSD", "USDP",
		"BTC", "ETH", "BNB", "DAI", "EUR", "TRY", "BRL",
		"GBP", "AUD", "RUB", "UAH", "ZAR", "NGN", "PLN",
		"RON", "ARS", "JPY", "MXN", "IDRT", "BIDR",
	}
	for _, q := range quotes {
		if strings.HasSuffix(s, q) && len(s) > len(q) {
			return pairInfo{Base: strings.TrimSuffix(s, q), Quote: q}
		}
	}
	return pairInfo{Base: s}
}

func cachedPairInfo() map[string]pairInfo {
	pairInfoCache.Lock()
	defer pairInfoCache.Unlock()
	if len(pairInfoCache.pairs) == 0 {
		return nil
	}
	return pairInfoCache.pairs
}

func fetchTickers(client *http.Client) ([]ticker, error) {
	// IMPORTANT: ticker/24hr is the primary data source and must return quickly.
	// exchangeInfo is fetched separately in the background so search suggestions are not blocked.
	pairs := cachedPairInfo()
	urls := []string{
		"https://data-api.binance.vision/api/v3/ticker/24hr",
		"https://api.binance.com/api/v3/ticker/24hr",
		"https://api1.binance.com/api/v3/ticker/24hr",
	}
	var lastErr error
	for _, apiURL := range urls {
		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("User-Agent", "BinanceGia-TrimUI/0.11")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		var raw []tickerRaw
		err = json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&raw)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		out := make([]ticker, 0, len(raw))
		for _, r := range raw {
			p := num(r.LastPrice)
			if p <= 0 || r.Symbol == "" {
				continue
			}
			pi, ok := pairs[r.Symbol]
			if !ok {
				pi = inferPairFromSymbol(r.Symbol)
			}
			out = append(out, ticker{
				Symbol: r.Symbol, BaseAsset: pi.Base, QuoteAsset: pi.Quote,
				Price: p, Change: num(r.PriceChangePercent), High: num(r.HighPrice), Low: num(r.LowPrice),
				Volume: num(r.Volume), QuoteVolume: num(r.QuoteVolume), Count: r.Count,
			})
		}
		if len(out) > 0 {
			return out, nil
		}
		lastErr = errors.New("không có dữ liệu Spot")
	}
	if lastErr == nil {
		lastErr = errors.New("không tải được dữ liệu")
	}
	return nil, lastErr
}

type cacheFile struct {
	UpdatedAt int64    `json:"updatedAt"`
	Tickers   []ticker `json:"tickers"`
}

type favoritesFile struct {
	Symbols []string `json:"symbols"`
}

type settingsFile struct {
	SortMode      int  `json:"sortMode"`
	FavoritesOnly bool `json:"favoritesOnly"`
	ChartRange    int  `json:"chartRange"`
	RefreshIndex  int  `json:"refreshIndex"`
}

func normalizeSettings(s settingsFile) settingsFile {
	if s.SortMode < 0 || s.SortMode > 3 {
		s.SortMode = sortVolume
	}
	if s.ChartRange < 0 || s.ChartRange >= len(chartRanges) {
		s.ChartRange = 0
	}
	if s.RefreshIndex < 0 || s.RefreshIndex >= len(refreshOptions) {
		s.RefreshIndex = defaultRefreshIndex
	}
	return s
}

func loadSettings() settingsFile {
	s := settingsFile{SortMode: sortVolume, FavoritesOnly: false, ChartRange: 0, RefreshIndex: defaultRefreshIndex}
	b, err := os.ReadFile(filepath.Join(appDir(), "settings.json"))
	if err != nil {
		return s
	}
	if json.Unmarshal(b, &s) != nil {
		return settingsFile{SortMode: sortVolume, RefreshIndex: defaultRefreshIndex}
	}
	return normalizeSettings(s)
}

func saveSettings(s settingsFile) error {
	return writeJSONAtomic(filepath.Join(appDir(), "settings.json"), normalizeSettings(s))
}

func writeJSONAtomic(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	err = enc.Encode(v)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func loadCache() ([]ticker, time.Time) {
	b, err := os.ReadFile(filepath.Join(appDir(), "market-cache.json"))
	if err != nil {
		return nil, time.Time{}
	}
	var c cacheFile
	if json.Unmarshal(b, &c) != nil || len(c.Tickers) == 0 {
		return nil, time.Time{}
	}
	for i := range c.Tickers {
		c.Tickers[i] = normalizePairFields(c.Tickers[i])
	}
	return c.Tickers, time.Unix(c.UpdatedAt, 0)
}
func saveCache(xs []ticker, updated time.Time) error {
	return writeJSONAtomic(filepath.Join(appDir(), "market-cache.json"), cacheFile{UpdatedAt: updated.Unix(), Tickers: xs})
}
func loadFavorites() map[string]bool {
	m := map[string]bool{}
	b, err := os.ReadFile(filepath.Join(appDir(), "favorites.json"))
	if err != nil {
		return m
	}
	var f favoritesFile
	if json.Unmarshal(b, &f) != nil {
		return m
	}
	for _, s := range f.Symbols {
		if s != "" {
			m[s] = true
		}
	}
	return m
}
func saveFavorites(m map[string]bool) error {
	xs := make([]string, 0, len(m))
	for s, on := range m {
		if on {
			xs = append(xs, s)
		}
	}
	sort.Strings(xs)
	return writeJSONAtomic(filepath.Join(appDir(), "favorites.json"), favoritesFile{Symbols: xs})
}

type chartPoint struct{ Open, High, Low, Close float64 }

type chartRangeSpec struct {
	Label    string
	Interval string
	Limit    int
}

var chartRanges = []chartRangeSpec{
	{Label: "15P", Interval: "15m", Limit: 80},
	{Label: "30P", Interval: "30m", Limit: 80},
	{Label: "1H", Interval: "1h", Limit: 80},
	{Label: "4H", Interval: "4h", Limit: 80},
	{Label: "1D", Interval: "1d", Limit: 90},
	{Label: "1T", Interval: "1w", Limit: 52},
	{Label: "1TH", Interval: "1M", Limit: 36},
}

func chartKey(symbol string, rangeIndex int) string {
	return fmt.Sprintf("%s:%d", symbol, rangeIndex)
}

func fetchChart(client *http.Client, symbol string, rangeIndex int) ([]chartPoint, error) {
	if rangeIndex < 0 || rangeIndex >= len(chartRanges) {
		rangeIndex = 0
	}
	spec := chartRanges[rangeIndex]
	bases := []string{
		"https://data-api.binance.vision",
		"https://api.binance.com",
		"https://api1.binance.com",
	}
	var lastErr error
	for _, base := range bases {
		apiURL := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s&limit=%d", base, symbol, spec.Interval, spec.Limit)
		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("User-Agent", "BinanceGia-TrimUI/0.11")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != 200 {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		var raw [][]json.RawMessage
		err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&raw)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		out := make([]chartPoint, 0, len(raw))
		for _, row := range raw {
			if len(row) < 5 {
				continue
			}
			var openS, highS, lowS, closeS string
			if json.Unmarshal(row[1], &openS) != nil || json.Unmarshal(row[2], &highS) != nil || json.Unmarshal(row[3], &lowS) != nil || json.Unmarshal(row[4], &closeS) != nil {
				continue
			}
			o, h, l, c := num(openS), num(highS), num(lowS), num(closeS)
			if o > 0 && h > 0 && l > 0 && c > 0 {
				out = append(out, chartPoint{Open: o, High: h, Low: l, Close: c})
			}
		}
		if len(out) >= 2 {
			return out, nil
		}
		lastErr = errors.New("không có dữ liệu biểu đồ")
	}
	if lastErr == nil {
		lastErr = errors.New("không tải được biểu đồ")
	}
	return nil, lastErr
}

type marketSummary struct {
	BTC, ETH float64
	Up, Down int
}

func summarizeMarket(xs []ticker) marketSummary {
	var m marketSummary
	for _, t := range xs {
		switch t.Symbol {
		case "BTCUSDT":
			m.BTC = t.Price
		case "ETHUSDT":
			m.ETH = t.Price
		}
		if t.Change > 0 {
			m.Up++
		} else if t.Change < 0 {
			m.Down++
		}
	}
	return m
}

func buildView(all []ticker, favorites map[string]bool, favoritesOnly bool, mode int) []ticker {
	xs := make([]ticker, 0, len(all))
	for _, t := range all {
		if favoritesOnly && !favorites[t.Symbol] {
			continue
		}
		xs = append(xs, t)
	}
	sortTickers(xs, mode)
	return xs
}
func findTicker(xs []ticker, symbol string) (ticker, bool) {
	for _, t := range xs {
		if t.Symbol == symbol {
			return t, true
		}
	}
	return ticker{}, false
}
func findIndex(xs []ticker, symbol string) int {
	for i, t := range xs {
		if t.Symbol == symbol {
			return i
		}
	}
	return -1
}

const (
	sortVolume  = 0
	sortGainers = 1
	sortLosers  = 2
	sortAZ      = 3
)

func sortTickers(xs []ticker, mode int) {
	switch mode {
	case sortGainers:
		sort.Slice(xs, func(i, j int) bool { return xs[i].Change > xs[j].Change })
	case sortLosers:
		sort.Slice(xs, func(i, j int) bool { return xs[i].Change < xs[j].Change })
	case sortAZ:
		sort.Slice(xs, func(i, j int) bool { return xs[i].Symbol < xs[j].Symbol })
	default:
		sort.Slice(xs, func(i, j int) bool { return xs[i].QuoteVolume > xs[j].QuoteVolume })
	}
}
func fmtPrice(v float64) string {
	switch {
	case v >= 1000:
		return fmt.Sprintf("%.2f", v)
	case v >= 1:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
	case v >= 0.01:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
	default:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", v), "0"), ".")
	}
}
func fmtCompact(v float64) string {
	a := v
	suf := ""
	if a >= 1e9 {
		a /= 1e9
		suf = "B"
	} else if a >= 1e6 {
		a /= 1e6
		suf = "M"
	} else if a >= 1e3 {
		a /= 1e3
		suf = "K"
	}
	if suf != "" {
		return fmt.Sprintf("%.2f%s", a, suf)
	}
	return fmt.Sprintf("%.0f", a)
}

type inputReader struct {
	fds                      []int
	hatX, hatY               int32
	nextRepeatX, nextRepeatY time.Time
}

func openInput() *inputReader {
	ir := &inputReader{}
	for i := 0; i < 16; i++ {
		p := fmt.Sprintf("/dev/input/event%d", i)
		fd, err := syscall.Open(p, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			ir.fds = append(ir.fds, fd)
		}
	}
	return ir
}
func (ir *inputReader) close() {
	for _, fd := range ir.fds {
		_ = syscall.Close(fd)
	}
}

type action int

const (
	actNone action = iota
	actUp
	actDown
	actLeft
	actRight
	actA
	actB
	actX
	actY
	actSelect
	actStart
	actL1
	actR1
	actExit
	actCharBase action = 1000
)

func linuxKeyRune(code uint16) (rune, bool) {
	// Linux input-event keycodes cho ban phim USB/Bluetooth neu nguoi dung co gan them.
	letters := map[uint16]rune{
		16: 'Q', 17: 'W', 18: 'E', 19: 'R', 20: 'T', 21: 'Y', 22: 'U', 23: 'I', 24: 'O', 25: 'P',
		30: 'A', 31: 'S', 32: 'D', 33: 'F', 34: 'G', 35: 'H', 36: 'J', 37: 'K', 38: 'L',
		44: 'Z', 45: 'X', 46: 'C', 47: 'V', 48: 'B', 49: 'N', 50: 'M',
		2: '1', 3: '2', 4: '3', 5: '4', 6: '5', 7: '6', 8: '7', 9: '8', 10: '9', 11: '0',
	}
	r, ok := letters[code]
	return r, ok
}

func (ir *inputReader) poll() []action {
	var out []action
	buf := make([]byte, 24*16)
	for _, fd := range ir.fds {
		for {
			n, err := syscall.Read(fd, buf)
			if err != nil {
				if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
					break
				}
				break
			}
			if n < 24 {
				break
			}
			for off := 0; off+24 <= n; off += 24 {
				typ := binary.LittleEndian.Uint16(buf[off+16 : off+18])
				code := binary.LittleEndian.Uint16(buf[off+18 : off+20])
				val := int32(binary.LittleEndian.Uint32(buf[off+20 : off+24]))
				if typ == EV_KEY && val == 1 {
					if r, ok := linuxKeyRune(code); ok {
						out = append(out, action(int(actCharBase)+int(r)))
						continue
					}
					switch code {
					case 14: // KEY_BACKSPACE
						out = append(out, actB)
					case 28: // KEY_ENTER
						out = append(out, actStart)
					case KEY_UP:
						out = append(out, actUp)
					case KEY_DOWN:
						out = append(out, actDown)
					case KEY_LEFT:
						out = append(out, actLeft)
					case KEY_RIGHT:
						out = append(out, actRight)
					case BTN_EAST:
						out = append(out, actA)
					case BTN_SOUTH:
						out = append(out, actB)
					case BTN_NORTH:
						out = append(out, actY)
					case BTN_WEST:
						out = append(out, actX)
					case BTN_TL:
						out = append(out, actL1)
					case BTN_TR:
						out = append(out, actR1)
					case BTN_SELECT:
						out = append(out, actSelect)
					case BTN_START:
						out = append(out, actStart)
					case BTN_MODE, KEY_ESC, KEY_HOMEPAGE:
						out = append(out, actExit)
					}
				} else if typ == EV_ABS {
					if code == ABS_HAT0X {
						if val != ir.hatX {
							if val < 0 {
								out = append(out, actLeft)
								ir.nextRepeatX = time.Now().Add(110 * time.Millisecond)
							} else if val > 0 {
								out = append(out, actRight)
								ir.nextRepeatX = time.Now().Add(110 * time.Millisecond)
							} else {
								ir.nextRepeatX = time.Time{}
							}
							ir.hatX = val
						}
					}
					if code == ABS_HAT0Y {
						if val != ir.hatY {
							if val < 0 {
								out = append(out, actUp)
								ir.nextRepeatY = time.Now().Add(110 * time.Millisecond)
							} else if val > 0 {
								out = append(out, actDown)
								ir.nextRepeatY = time.Now().Add(110 * time.Millisecond)
							} else {
								ir.nextRepeatY = time.Time{}
							}
							ir.hatY = val
						}
					}
				}
			}
			if n < len(buf) {
				break
			}
		}
	}
	// Lặp D-pad nhanh khi giữ phím: phản hồi bàn phím ảo nhanh hơn đáng kể.
	now := time.Now()
	if ir.hatX != 0 && !ir.nextRepeatX.IsZero() && !now.Before(ir.nextRepeatX) {
		if ir.hatX < 0 {
			out = append(out, actLeft)
		} else {
			out = append(out, actRight)
		}
		ir.nextRepeatX = now.Add(45 * time.Millisecond)
	}
	if ir.hatY != 0 && !ir.nextRepeatY.IsZero() && !now.Before(ir.nextRepeatY) {
		if ir.hatY < 0 {
			out = append(out, actUp)
		} else {
			out = append(out, actDown)
		}
		ir.nextRepeatY = now.Add(45 * time.Millisecond)
	}
	return out
}

type uiAssets struct {
	title, colPair, colPrice, col24h, footerList, footerDetail, detailTitle, statusLoading, statusError *asset
	sortVol, sortGain, sortLose, sortAZ                                                                 *asset
	labelPrice, labelChange, labelHigh, labelLow, labelVolume, labelQuote, labelTrades                  *asset
}

func loadAssets() uiAssets {
	return uiAssets{
		title: loadAsset("title.png"), colPair: loadAsset("col_pair.png"), colPrice: loadAsset("col_price.png"), col24h: loadAsset("col_24h.png"), footerList: loadAsset("footer_list.png"), footerDetail: loadAsset("footer_detail.png"), detailTitle: loadAsset("detail_title.png"), statusLoading: loadAsset("loading.png"), statusError: loadAsset("error.png"), sortVol: loadAsset("sort_volume.png"), sortGain: loadAsset("sort_gain.png"), sortLose: loadAsset("sort_lose.png"), sortAZ: loadAsset("sort_az.png"), labelPrice: loadAsset("label_price.png"), labelChange: loadAsset("label_change.png"), labelHigh: loadAsset("label_high.png"), labelLow: loadAsset("label_low.png"), labelVolume: loadAsset("label_volume.png"), labelQuote: loadAsset("label_quote.png"), labelTrades: loadAsset("label_trades.png"),
	}
}

var searchKeyboard = [][]string{
	{"1", "2", "3", "4", "5", "6", "7", "8", "9", "0"},
	{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"},
	{"A", "S", "D", "F", "G", "H", "J", "K", "L"},
	{"Z", "X", "C", "V", "B", "N", "M"},
}

// moveKeyboardBounded di chuyển trong bàn phím A-Z mà không vòng lại đầu hàng.
// Nếu người dùng đẩy D-pad ra khỏi vùng phím hợp lệ, exited=true để UI chuyển
// focus lên vùng gợi ý cặp coin.
func moveKeyboardBounded(row, col, dr, dc int) (newRow, newCol int, exited bool) {
	rows := len(searchKeyboard)
	if rows == 0 {
		return row, col, true
	}
	if row < 0 || row >= rows {
		row = 0
	}
	if col < 0 || col >= len(searchKeyboard[row]) {
		col = 0
	}
	if dr != 0 {
		nr := row + dr
		if nr < 0 || nr >= rows {
			return row, col, true
		}
		// Giữ vị trí tương đối theo chiều ngang khi chuyển giữa hàng 10/9/7 phím.
		oldN, newN := len(searchKeyboard[row]), len(searchKeyboard[nr])
		nc := 0
		if oldN > 1 && newN > 1 {
			nc = int(math.Round(float64(col) * float64(newN-1) / float64(oldN-1)))
		}
		if nc < 0 {
			nc = 0
		}
		if nc >= newN {
			nc = newN - 1
		}
		return nr, nc, false
	}
	if dc != 0 {
		nc := col + dc
		if nc < 0 || nc >= len(searchKeyboard[row]) {
			return row, col, true
		}
		return row, nc, false
	}
	return row, col, false
}

func backspaceQuery(query string) string {
	rs := []rune(query)
	if len(rs) == 0 {
		return query
	}
	return string(rs[:len(rs)-1])
}

func normalizeSearch(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	// Dấu phân cách không được tính là ký tự tìm kiếm, nhưng vẫn giữ cả BASE và QUOTE.
	// Ví dụ: BNB/BTC và BNBBTC đều chuẩn hóa thành BNBBTC.
	r := strings.NewReplacer("/", "", "-", "", "_", "", " ", "")
	return r.Replace(s)
}

func quotePriority(q string) int {
	switch q {
	case "USDT":
		return 0
	case "USDC":
		return 1
	case "FDUSD":
		return 2
	case "BTC":
		return 3
	case "ETH":
		return 4
	case "BNB":
		return 5
	case "EUR":
		return 6
	case "TRY":
		return 7
	case "BRL":
		return 8
	default:
		return 20
	}
}

func searchPairs(all []ticker, query string, limit int) []ticker {
	q := normalizeSearch(query)
	if q == "" || limit <= 0 {
		return nil
	}
	type ranked struct {
		t                ticker
		score, quoteRank int
	}
	matches := make([]ranked, 0, 64)
	for _, raw := range all {
		t := normalizePairFields(raw)
		base := strings.ToUpper(t.BaseAsset)
		quote := strings.ToUpper(t.QuoteAsset)
		if base == "" {
			continue
		}
		compact := base + quote
		score := 99
		switch {
		case q == compact:
			score = 0
		case q == base:
			score = 1
		case strings.HasPrefix(base, q):
			score = 2
		case strings.HasPrefix(compact, q):
			score = 3
		case strings.Contains(base, q):
			score = 4
		case strings.Contains(compact, q):
			score = 5
		default:
			continue
		}
		matches = append(matches, ranked{t: t, score: score, quoteRank: quotePriority(quote)})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		if matches[i].quoteRank != matches[j].quoteRank {
			return matches[i].quoteRank < matches[j].quoteRank
		}
		if matches[i].t.QuoteVolume != matches[j].t.QuoteVolume {
			return matches[i].t.QuoteVolume > matches[j].t.QuoteVolume
		}
		return pairLabel(matches[i].t) < pairLabel(matches[j].t)
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	out := make([]ticker, len(matches))
	for i := range matches {
		out[i] = matches[i].t
	}
	return out
}

func applySearchKey(query, key string) string {
	switch key {
	case "LÙI", "DEL":
		if len(query) > 0 {
			query = query[:len(query)-1]
		}
	case "XÓA", "XOA":
		query = ""
	default:
		if key != "" && len(query) < 12 {
			query += key
		}
	}
	return query
}

const (
	pageFavorites = 0
	pageSearch    = 1
	pageBusiness  = 2
	pageHitech    = 3
)

func bottomTabsHeight(fb *framebuffer) int {
	return max(118, fb.h*15/100)
}

func drawBottomTabs(fb *framebuffer, active int, hint string) {
 footerH:=bottomTabsHeight(fb)
 y:=fb.h-footerH
 fb.rect(0,y,fb.w,footerH,cPanel)
 margin:=max(18,fb.w/50)
 if hint!="" {drawASCIIRatio(fb,margin,y+10,3,2,hint,cMuted)}
 tabs:=[]struct{page int;label string}{
  {pageFavorites,"BINANCE"}, {pageBusiness,"KINH DOANH"}, {pageHitech,"HI-TECH"},
 }
 tabY:=y+58
 for i,t:=range tabs {
  center:=fb.w*(i*2+1)/6
  x:=center-asciiWidth(2,t.label)/2
  clr:=cMuted
  activeTab:=active==t.page || (active==pageSearch&&t.page==pageFavorites)
  if activeTab {clr=cYellow;fb.rect(fb.w*i/3+7,tabY+24,fb.w/3-14,4,cYellow)}
  drawASCII(fb,x,tabY,2,t.label,clr)
 }
}

func cycleMainPage(page,dir int)int {
 ordered:=[]int{pageFavorites,pageBusiness,pageHitech}
 if page==pageSearch {page=pageFavorites}
 for i,p:=range ordered {
  if p==page{return ordered[(i+dir+len(ordered))%len(ordered)]}
 }
 return pageFavorites
}

func favoriteTickers(all []ticker, favorites map[string]bool) []ticker {
	xs := make([]ticker, 0, len(favorites))
	for _, t := range all {
		if favorites[t.Symbol] {
			xs = append(xs, normalizePairFields(t))
		}
	}
	sort.SliceStable(xs, func(i, j int) bool {
		qi, qj := quotePriority(xs[i].QuoteAsset), quotePriority(xs[j].QuoteAsset)
		if qi != qj {
			return qi < qj
		}
		if xs[i].QuoteVolume != xs[j].QuoteVolume {
			return xs[i].QuoteVolume > xs[j].QuoteVolume
		}
		return pairLabel(xs[i]) < pairLabel(xs[j])
	})
	return xs
}

func favoriteGridWindow(count, sel int) (start, end int) {
	const cols = 2
	const visibleRows = 4
	if count <= 0 {
		return 0, 0
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= count {
		sel = count - 1
	}
	row := sel / cols
	startRow := row - visibleRows/2
	if startRow < 0 {
		startRow = 0
	}
	maxStartRow := (count+cols-1)/cols - visibleRows
	if maxStartRow < 0 {
		maxStartRow = 0
	}
	if startRow > maxStartRow {
		startRow = maxStartRow
	}
	start = startRow * cols
	end = start + visibleRows*cols
	if end > count {
		end = count
	}
	return start, end
}

func renderSparkline(fb *framebuffer, x, y, w, h int, pts []chartPoint, change float64) {
	if w < 8 || h < 8 {
		return
	}
	cc := cGreen
	if change < 0 {
		cc = cRed
	}
	if len(pts) < 2 {
		mid := y + h/2
		fb.line(x, mid, x+w, mid, color{55, 61, 70})
		return
	}
	minV, maxV := pts[0].Close, pts[0].Close
	for _, p := range pts[1:] {
		if p.Close < minV {
			minV = p.Close
		}
		if p.Close > maxV {
			maxV = p.Close
		}
	}
	if maxV <= minV {
		maxV = minV + 1
	}
	px := func(i int) int {
		if len(pts) <= 1 {
			return x
		}
		return x + i*(w-1)/(len(pts)-1)
	}
	py := func(v float64) int {
		return y + h - 1 - int((v-minV)/(maxV-minV)*float64(h-1))
	}
	for i := 1; i < len(pts); i++ {
		fb.line(px(i-1), py(pts[i-1].Close), px(i), py(pts[i].Close), cc)
	}
}

func weekdayVI(t time.Time) string {
	switch t.Weekday() {
	case time.Monday:
		return "T2"
	case time.Tuesday:
		return "T3"
	case time.Wednesday:
		return "T4"
	case time.Thursday:
		return "T5"
	case time.Friday:
		return "T6"
	case time.Saturday:
		return "T7"
	default:
		return "CN"
	}
}

func searchSuggestionWindow(total, sel, visible int) (start, end int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= total {
		sel = total - 1
	}
	start = sel - visible/2
	if start < 0 {
		start = 0
	}
	maxStart := total - visible
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	end = start + visible
	if end > total {
		end = total
	}
	return start, end
}

func renderHeader(fb *framebuffer, active int, updated time.Time, stale bool, loading bool, errMsg string) int {
	_ = active
	margin := max(18, fb.w/50)
	// Thu nhỏ thanh trên để nhường không gian cho nội dung.
	headerH := max(112, fb.h*15/100)
	fb.rect(0, 0, fb.w, headerH, cPanel)

	// Version đặt sát bên phải chữ E và cao hơn để không bị chồng lên chữ.
	titleY := 10
	drawASCIIRatio(fb, margin, titleY, 9, 2, "BINANCE", cYellow)
	verX := margin + asciiWidthRatio(9, 2, "BINANCE") + 5
	drawASCII(fb, verX, titleY+18, 1, appVersion, cMuted)
	now := time.Now()
	dateTxt := weekdayVI(now) + " " + now.Format("02/01/2006")
	timeTxt := now.Format("15:04")
	// Giảm 50% kích thước đồng hồ/ngày so với v0.14; thêm thứ T2..T7/CN.
	drawASCII(fb, fb.w-asciiWidth(4, timeTxt)-margin, 8, 4, timeTxt, cText)
	drawASCII(fb, fb.w-asciiWidth(2, dateTxt)-margin, 48, 2, dateTxt, cMuted)

	status := "ĐANG TẢI"
	sc := cMuted
	if !updated.IsZero() {
		if stale {
			status = "DỮ LIỆU CŨ"
			sc = cRed
		} else {
			status = "TRỰC TUYẾN"
			sc = cGreen
		}
	}
	if errMsg != "" {
		status = "MẤT MẠNG"
		sc = cRed
	} else if loading {
		status = "ĐANG CẬP NHẬT"
		sc = cMuted
	}
	// Đưa trạng thái lên ngay dưới/chân chữ BINANCE.
	drawASCII(fb, margin, 68, 2, status, sc)
	return headerH
}

func renderExitConfirm(fb *framebuffer, choice int) {
	w := fb.w * 58 / 100
	h := fb.h * 30 / 100
	if w < 360 {
		w = 360
	}
	if h < 180 {
		h = 180
	}
	x := (fb.w - w) / 2
	y := (fb.h - h) / 2
	fb.rect(x-4, y-4, w+8, h+8, cYellow)
	fb.rect(x, y, w, h, cPanel)
	title := "THOÁT"
	drawASCII(fb, x+w/2-asciiWidth(3, title)/2, y+26, 3, title, cText)
	msg := "BẠN CÓ MUỐN THOÁT?"
	drawASCII(fb, x+w/2-asciiWidth(1, msg)/2, y+72, 1, msg, cMuted)
	btnW := (w - 54) / 2
	btnH := 48
	by := y + h - 70
	labels := []string{"HỦY", "ĐỒNG Ý"}
	for i := 0; i < 2; i++ {
		bx := x + 18 + i*(btnW+18)
		bg := cPanel2
		tc := cText
		if choice == i {
			bg = cYellow
			tc = cBg
		}
		fb.rect(bx, by, btnW, btnH, bg)
		tw := asciiWidth(2, labels[i])
		drawASCII(fb, bx+(btnW-tw)/2, by+14, 2, labels[i], tc)
	}
}

func renderOTAPrompt(fb *framebuffer, m otaManifest, choice int) {
	w := fb.w * 66 / 100
	h := fb.h * 36 / 100
	if w < 420 {
		w = 420
	}
	if h < 220 {
		h = 220
	}
	x := (fb.w - w) / 2
	y := (fb.h - h) / 2
	fb.rect(x-4, y-4, w+8, h+8, cYellow)
	fb.rect(x, y, w, h, cPanel)
	title := "CẬP NHẬT OTA"
	drawASCII(fb, x+w/2-asciiWidth(3, title)/2, y+24, 3, title, cText)
	ver := appVersion + "  →  " + m.Version
	drawASCII(fb, x+w/2-asciiWidth(2, ver)/2, y+68, 2, ver, cYellow)
	msg := "ĐÃ CÓ PHIÊN BẢN MỚI"
	drawASCII(fb, x+w/2-asciiWidth(1, msg)/2, y+102, 1, msg, cMuted)
	btnW := (w - 54) / 2
	btnH := 50
	by := y + h - 72
	labels := []string{"HỦY", "CẬP NHẬT"}
	for i := 0; i < 2; i++ {
		bx := x + 18 + i*(btnW+18)
		bg := cPanel2
		tc := cText
		if choice == i {
			bg, tc = cYellow, cBg
		}
		fb.rect(bx, by, btnW, btnH, bg)
		tw := asciiWidth(2, labels[i])
		drawASCII(fb, bx+(btnW-tw)/2, by+14, 2, labels[i], tc)
	}
}

func renderOTAStatus(fb *framebuffer, title, msg string, restart bool) {
	w := fb.w * 68 / 100
	h := fb.h * 32 / 100
	if w < 440 {
		w = 440
	}
	if h < 200 {
		h = 200
	}
	x := (fb.w - w) / 2
	y := (fb.h - h) / 2
	border := cYellow
	if strings.Contains(title, "THẤT BẠI") {
		border = cRed
	}
	fb.rect(x-4, y-4, w+8, h+8, border)
	fb.rect(x, y, w, h, cPanel)
	drawASCII(fb, x+w/2-asciiWidth(2, title)/2, y+28, 2, title, cText)
	if msg != "" {
		if len([]rune(msg)) > 48 {
			msg = string([]rune(msg)[:48])
		}
		drawASCII(fb, x+22, y+78, 1, msg, cMuted)
	}
	if restart {
		hint := "A: KHỞI ĐỘNG LẠI"
		drawASCII(fb, x+w/2-asciiWidth(2, hint)/2, y+h-54, 2, hint, cYellow)
	} else {
		hint := "A/B: ĐÓNG"
		drawASCII(fb, x+w/2-asciiWidth(2, hint)/2, y+h-54, 2, hint, cYellow)
	}
}

func renderFavoritesGrid(fb *framebuffer, xs []ticker, sel int, charts map[string]chartCacheEntry, loading bool, errMsg string, updated time.Time, stale bool) {
	fb.fill(cBg)
	margin := max(18, fb.w/50)
	headerH := renderHeader(fb, pageFavorites, updated, stale, loading, errMsg)
	footerH := bottomTabsHeight(fb)

	if len(xs) == 0 {
		msg := "CHƯA CÓ CẶP YÊU THÍCH"
		drawASCII(fb, fb.w/2-asciiWidth(2, msg)/2, headerH+90, 2, msg, cMuted)
		drawFavoriteMark(fb, fb.w/2-14, headerH+135, 28, false)
		drawASCII(fb, fb.w/2-asciiWidth(1, "START: TÌM KIẾM ĐỂ GẮN SAO")/2, headerH+185, 1, "START: TÌM KIẾM ĐỂ GẮN SAO", cYellow)
		drawBottomTabs(fb, pageFavorites, "A: MỞ START: TÌM KIẾM SELECT: BỎ SAO")
		return
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= len(xs) {
		sel = len(xs) - 1
	}
	start, end := favoriteGridWindow(len(xs), sel)
	const cols = 2
	rows := 4
	gap := max(10, fb.w/90)
	top := headerH + 14
	availH := fb.h - top - footerH - gap
	cardH := (availH - gap*(rows-1)) / rows
	cardW := (fb.w - 2*margin - gap) / cols
	for idx := start; idx < end; idx++ {
		local := idx - start
		r := local / cols
		c := local % cols
		x := margin + c*(cardW+gap)
		y := top + r*(cardH+gap)
		selected := idx == sel
		bg := cPanel
		if selected {
			bg = cPanel2
		}
		fb.rect(x, y, cardW, cardH, bg)
		if selected {
			fb.rect(x, y, cardW, 3, cYellow)
			fb.rect(x, y, 3, cardH, cYellow)
			fb.rect(x+cardW-3, y, 3, cardH, cYellow)
			fb.rect(x, y+cardH-3, cardW, 3, cYellow)
		}
		t := xs[idx]
		cc := cGreen
		if t.Change < 0 {
			cc = cRed
		}
		fb.rect(x, y, 4, cardH, cc)
		drawASCII(fb, x+14, y+10, 1, "BINANCE", cMuted)
		sym := pairLabel(t)
		drawASCII(fb, x+14, y+28, 2, sym, cText)
		drawFavoriteMark(fb, x+cardW-28, y+10, 16, true)
		chartX := x + cardW*55/100
		chartY := y + 32
		chartW := cardW - (chartX - x) - 14
		chartH := max(28, cardH/3)
		ce := charts[chartKey(t.Symbol, 0)]
		renderSparkline(fb, chartX, chartY, chartW, chartH, ce.pts, t.Change)
		priceScale := 2
		if cardW > 430 {
			priceScale = 3
		}
		drawASCII(fb, x+14, y+cardH-48, priceScale, fmtPrice(t.Price), cText)
		chg := fmt.Sprintf("%+.2f%%", t.Change)
		drawASCIIRatio(fb, x+cardW-asciiWidthRatio(3, 2, chg)-14, y+cardH-28, 3, 2, chg, cc)
	}
	if loading {
		drawASCII(fb, margin, headerH-18, 1, "ĐANG CẬP NHẬT...", cMuted)
	}
	drawBottomTabs(fb, pageFavorites, "A: MỞ START: TÌM KIẾM SELECT: BỎ SAO")
}

func renderSearch(fb *framebuffer, a uiAssets, query string, suggestions []ticker, suggSel int, focusSuggestions bool, kbRow, kbCol int, cursorVisible bool, loading bool, errMsg string, updated time.Time, stale bool, favorites map[string]bool) {
	fb.fill(cBg)
	margin := max(18, fb.w/50)
	headerH := renderHeader(fb, pageSearch, updated, stale, loading, errMsg)
	footerH := bottomTabsHeight(fb)

	searchY := headerH + 16
	searchH := max(76, fb.h/10)
	fb.rect(margin, searchY, fb.w-2*margin, searchH, cPanel)
	fb.rect(margin, searchY, 6, searchH, cYellow)
	label := "TÌM KIẾM:"
	searchScale := 2
	input := query
	inputColor := cText
	gapX := 14
	// Cụm chữ căn trái, nhưng căn giữa theo chiều cao của ô tìm kiếm.
	groupX := margin + 24
	textY := searchY + max(6, (searchH-7*searchScale)/2)
	// TÌM KIẾM in đậm; placeholder/input giữ nét thường.
	drawASCII(fb, groupX, textY, searchScale, label, cText)
	drawASCII(fb, groupX+1, textY, searchScale, label, cText)
	inputX := groupX + asciiWidth(searchScale, label) + gapX
	if query == "" {
		cur := " "
		if cursorVisible {
			cur = "|"
		}
		input = cur + " " + "NHẬP TÊN COIN"
		inputColor = color{105, 112, 124}
	} else {
		if cursorVisible {
			input += "|"
		} else {
			input += " "
		}
	}
	drawASCII(fb, inputX, textY, searchScale, input, inputColor)

	kbH := max(150, fb.h*20/100)
	kbY := fb.h - footerH - kbH - 8
	suggTop := searchY + searchH + 14
	suggBottom := kbY - 10
	if query == "" {
		// Placeholder đã nằm ngay trong ô tìm kiếm.
	} else if len(suggestions) == 0 {
		if loading && updated.IsZero() {
			msg := "ĐANG TẢI DANH SÁCH CẶP..."
			drawASCII(fb, fb.w/2-asciiWidth(2, msg)/2, suggTop+30, 2, msg, cYellow)
			drawASCII(fb, margin, suggTop+70, 1, "GỢI Ý SẼ HIỆN NGAY KHI CÓ DỮ LIỆU", cMuted)
		} else {
			msg := "KHÔNG TÌM THẤY CẶP PHÙ HỢP"
			drawASCII(fb, fb.w/2-asciiWidth(2, msg)/2, suggTop+30, 2, msg, cRed)
			if loading {
				drawASCII(fb, margin, suggTop+70, 1, "ĐANG CẬP NHẬT BINANCE...", cMuted)
			}
		}
	} else {
		visibleRows := max(6, (suggBottom-suggTop)/40)
		rowH := max(28, (suggBottom-suggTop)/visibleRows)
		if rowH > 52 {
			rowH = 52
		}
		visibleRows = max(1, (suggBottom-suggTop)/rowH)
		if suggSel < 0 {
			suggSel = 0
		}
		if suggSel >= len(suggestions) {
			suggSel = len(suggestions) - 1
		}
		start, end := searchSuggestionWindow(len(suggestions), suggSel, visibleRows)
		for i := start; i < end; i++ {
			t := suggestions[i]
			row := i - start
			y := suggTop + row*rowH
			selected := i == suggSel
			if selected {
				bg := cPanel2
				if focusSuggestions {
					bg = color{45, 50, 58}
				}
				fb.rect(margin, y, fb.w-2*margin, rowH-4, bg)
				fb.rect(margin, y, 5, rowH-4, cYellow)
			}
			drawFavoriteMark(fb, margin+14, y+12, 16, favorites[t.Symbol])
			sym := pairLabel(t)
			drawASCII(fb, margin+40, y+10, 2, sym, cText)
			price := fmtPrice(t.Price)
			drawASCII(fb, fb.w*52/100, y+10, 2, price, cText)
			chg := fmt.Sprintf("%+.2f%%", t.Change)
			cc := cGreen
			if t.Change < 0 {
				cc = cRed
			}
			drawASCII(fb, fb.w-asciiWidth(2, chg)-margin-12, y+10, 2, chg, cc)
		}
	}

	// Bàn phím ảo 0-9 + A-Z: D-pad + A. Đi ra khỏi biên bàn phím sẽ chuyển lên gợi ý.
	fb.rect(margin, kbY, fb.w-2*margin, kbH, cPanel)
	rows := len(searchKeyboard)
	gap := max(5, margin/3)
	cellH := (kbH - gap*(rows+1)) / rows
	for r := 0; r < rows; r++ {
		cols := len(searchKeyboard[r])
		rowW := fb.w - 2*margin
		cellW := (rowW - gap*(cols+1)) / cols
		for c, key := range searchKeyboard[r] {
			x := margin + gap + c*(cellW+gap)
			y := kbY + gap + r*(cellH+gap)
			selected := !focusSuggestions && r == kbRow && c == kbCol
			bg := cPanel2
			tc := cText
			if selected {
				bg = cYellow
				tc = cBg
			}
			fb.rect(x, y, cellW, cellH, bg)
			scale := 2
			tw := asciiWidth(scale, key)
			tx := x + max(2, (cellW-tw)/2)
			ty := y + max(2, (cellH-7*scale)/2)
			drawASCII(fb, tx, ty, scale, key, tc)
		}
	}
	// Thanh tab vẫn luôn hiện ở màn Tìm kiếm, không có dòng chú thích chọn gợi ý.
	drawBottomTabs(fb, pageSearch, "A: NHẬP Y: ĐỔI VÙNG START: ẨN  SELECT: YÊU THÍCH")
	if errMsg != "" && len(suggestions) == 0 && query != "" {
		drawASCII(fb, fb.w-asciiWidth(1, "MẤT MẠNG")-margin, headerH-18, 1, "MẤT MẠNG", cRed)
	}
}

func renderList(fb *framebuffer, a uiAssets, xs []ticker, all []ticker, sel, mode int, loading bool, errMsg string, favorites map[string]bool, favoritesOnly bool, updated time.Time, stale bool, refreshEvery time.Duration) {
	fb.fill(cBg)
	margin := max(18, fb.w/50)
	headerH := max(88, fb.h/9)
	footerH := max(48, fb.h/13)
	fb.rect(0, 0, fb.w, headerH, cPanel)
	drawAsset(fb, a.title, margin, 14)
	var sa *asset
	switch mode {
	case sortGainers:
		sa = a.sortGain
	case sortLosers:
		sa = a.sortLose
	case sortAZ:
		sa = a.sortAZ
	default:
		sa = a.sortVol
	}
	if sa != nil {
		drawAsset(fb, sa, fb.w-sa.img.Bounds().Dx()-margin, 16)
	}
	ms := summarizeMarket(all)
	summary := ""
	if ms.BTC > 0 {
		summary += "BTC " + fmtPrice(ms.BTC)
	}
	if ms.ETH > 0 {
		if summary != "" {
			summary += "   "
		}
		summary += "ETH " + fmtPrice(ms.ETH)
	}
	if summary != "" {
		drawASCII(fb, margin, headerH-43, 1, summary, cText)
	}
	trend := fmt.Sprintf("XANH %d  DO %d  AUTO %ds", ms.Up, ms.Down, int(refreshEvery/time.Second))
	drawASCII(fb, fb.w-asciiWidth(1, trend)-margin, headerH-43, 1, trend, cMuted)
	status := "CHƯA CẬP NHẬT"
	statusColor := cMuted
	if !updated.IsZero() {
		if stale {
			status = "DỮ LIỆU CŨ " + updated.Format("15:04")
			statusColor = cRed
		} else {
			status = "CẬP NHẬT " + updated.Format("15:04")
			statusColor = cGreen
		}
	}
	drawASCII(fb, margin, headerH-24, 1, status, statusColor)
	if favoritesOnly {
		txt := "CHỈ YÊU THÍCH"
		drawASCII(fb, fb.w-asciiWidth(1, txt)-margin, headerH-24, 1, txt, cYellow)
	}
	if errMsg != "" && len(xs) > 0 {
		txt := "MẤT MẠNG - ĐANG HIỆN DỮ LIỆU CŨ"
		drawASCII(fb, fb.w/2-asciiWidth(1, txt)/2, headerH-24, 1, txt, cRed)
	}
	top := headerH + 8
	drawAsset(fb, a.colPair, margin+28, top)
	drawAsset(fb, a.colPrice, fb.w*50/100, top)
	drawAsset(fb, a.col24h, fb.w*78/100, top)
	top += 30
	rowH := max(38, (fb.h-top-footerH-8)/11)
	visible := max(1, (fb.h-top-footerH)/rowH)
	if len(xs) == 0 {
		if loading {
			drawAsset(fb, a.statusLoading, margin, top+30)
		} else if favoritesOnly {
			drawASCII(fb, margin, top+35, 3, "CHƯA CÓ YÊU THÍCH", cYellow)
			drawASCII(fb, margin, top+80, 2, "SELECT ĐỂ THÊM COIN", cMuted)
		} else {
			drawAsset(fb, a.statusError, margin, top+30)
			if errMsg != "" {
				drawASCII(fb, margin, top+70, 2, errMsg, cRed)
			}
		}
	} else {
		if sel < 0 {
			sel = 0
		}
		if sel >= len(xs) {
			sel = len(xs) - 1
		}
		start := sel - visible/2
		if start < 0 {
			start = 0
		}
		if start+visible > len(xs) {
			start = max(0, len(xs)-visible)
		}
		for i := 0; i < visible && start+i < len(xs); i++ {
			idx := start + i
			y := top + i*rowH
			if idx == sel {
				fb.rect(margin/2, y-3, fb.w-margin, rowH, cPanel2)
				fb.rect(margin/2, y-3, 5, rowH, cYellow)
			}
			t := xs[idx]
			drawFavoriteMark(fb, margin+8, y+9, 14, favorites[t.Symbol])
			symbol := pairLabel(t)
			drawASCII(fb, margin+30, y+8, 2, symbol, cText)
			drawASCII(fb, fb.w*50/100, y+8, 2, fmtPrice(t.Price), cText)
			change := fmt.Sprintf("%+.2f%%", t.Change)
			cc := cGreen
			if t.Change < 0 {
				cc = cRed
			}
			drawASCII(fb, fb.w*78/100, y+8, 2, change, cc)
		}
	}
	fb.rect(0, fb.h-footerH, fb.w, footerH, cPanel)
	drawAsset(fb, a.footerList, margin, fb.h-footerH+max(8, (footerH-22)/2))
	if loading && len(xs) > 0 {
		drawASCII(fb, fb.w-120, fb.h-footerH+12, 1, "ĐANG TẢI", cMuted)
	}
}

func renderChart(fb *framebuffer, x, y, w, h int, pts []chartPoint, loading bool, errMsg, rangeLabel string) {
	fb.rect(x, y, w, h, cPanel)
	fb.rect(x+1, y+1, w-2, 1, color{55, 61, 70})
	fb.rect(x+1, y+h-2, w-2, 1, color{55, 61, 70})
	drawASCII(fb, x+12, y+10, 2, "NẾN "+rangeLabel, cYellow)
	if loading && len(pts) == 0 {
		drawASCII(fb, x+12, y+50, 2, "ĐANG TẢI...", cMuted)
		return
	}
	if len(pts) < 2 {
		drawASCII(fb, x+12, y+50, 2, "KHÔNG CÓ DỮ LIỆU", cRed)
		if errMsg != "" {
			drawASCII(fb, x+12, y+80, 1, errMsg, cMuted)
		}
		return
	}
	minV, maxV := pts[0].Low, pts[0].High
	for _, p := range pts[1:] {
		if p.Low < minV {
			minV = p.Low
		}
		if p.High > maxV {
			maxV = p.High
		}
	}
	if maxV <= minV {
		maxV = minV + 1
	}
	plotX, plotY := x+12, y+48
	plotW, plotH := w-24, h-76
	if plotW < 20 || plotH < 20 {
		return
	}
	// Luoi ngang nhe de doc gia.
	for i := 1; i <= 3; i++ {
		fb.hline(plotX, plotY+i*plotH/4, plotW, color{45, 50, 58})
	}
	step := float64(plotW) / float64(len(pts))
	bodyW := int(step * 0.62)
	if bodyW < 2 {
		bodyW = 2
	}
	if bodyW > 10 {
		bodyW = 10
	}
	priceY := func(v float64) int {
		return plotY + plotH - int((v-minV)/(maxV-minV)*float64(plotH))
	}
	for i, p := range pts {
		cx := plotX + int((float64(i)+0.5)*step)
		yHigh, yLow := priceY(p.High), priceY(p.Low)
		yOpen, yClose := priceY(p.Open), priceY(p.Close)
		cc := cGreen
		if p.Close < p.Open {
			cc = cRed
		}
		fb.line(cx, yHigh, cx, yLow, cc)
		top := yOpen
		bottom := yClose
		if top > bottom {
			top, bottom = bottom, top
		}
		if bottom-top < 2 {
			bottom = top + 2
		}
		fb.rect(cx-bodyW/2, top, bodyW, bottom-top+1, cc)
	}
	drawASCII(fb, plotX, y+h-20, 1, "THẤP "+fmtPrice(minV), cMuted)
	mx := "CAO " + fmtPrice(maxV)
	drawASCII(fb, x+w-asciiWidth(1, mx)-12, y+h-20, 1, mx, cMuted)
}

func closeSeries(pts []chartPoint) []float64 {
	out := make([]float64, len(pts))
	for i, p := range pts {
		out[i] = p.Close
	}
	return out
}

func smaSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 0 {
		return out
	}
	sum := 0.0
	for i, v := range values {
		sum += v
		if i >= period {
			sum -= values[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

func rollingStdSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if period <= 1 {
		return out
	}
	for i := period - 1; i < len(values); i++ {
		mean := 0.0
		for j := i - period + 1; j <= i; j++ {
			mean += values[j]
		}
		mean /= float64(period)
		varsum := 0.0
		for j := i - period + 1; j <= i; j++ {
			d := values[j] - mean
			varsum += d * d
		}
		out[i] = math.Sqrt(varsum / float64(period))
	}
	return out
}

func emaSeries(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 || period <= 0 {
		return out
	}
	alpha := 2.0 / float64(period+1)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = alpha*values[i] + (1-alpha)*out[i-1]
	}
	return out
}

func macdSeries(values []float64) (dif, dea, hist []float64) {
	if len(values) == 0 {
		return []float64{}, []float64{}, []float64{}
	}
	ema12 := emaSeries(values, 12)
	ema26 := emaSeries(values, 26)
	dif = make([]float64, len(values))
	for i := range values {
		dif[i] = ema12[i] - ema26[i]
	}
	dea = emaSeries(dif, 9)
	hist = make([]float64, len(values))
	for i := range values {
		hist[i] = dif[i] - dea[i]
	}
	return
}

func drawPolyline(fb *framebuffer, xs, ys []int, start int, cc color) {
	for i := start + 1; i < len(xs); i++ {
		if xs[i-1] == 0 && ys[i-1] == 0 {
			continue
		}
		if xs[i] == 0 && ys[i] == 0 {
			continue
		}
		fb.line(xs[i-1], ys[i-1], xs[i], ys[i], cc)
	}
}

func renderDetail(fb *framebuffer, a uiAssets, t ticker, pts []chartPoint, chartLoading bool, chartErr string, favorite bool, updated time.Time, stale bool, rangeLabel string, refreshEvery time.Duration) {
	fb.fill(color{10, 11, 16})
	margin := max(18, fb.w/44)
	footerH := max(44, fb.h/14)
	headerH := max(96, fb.h/7)
	contentTop := headerH + 10
	contentBottom := fb.h - footerH - 10

	// Header kiểu trading app.
	fb.rect(0, 0, fb.w, headerH, color{14, 15, 20})
	sym := pairLabel(t)
	drawASCII(fb, margin, 10, 1, "BTC"[:0]+sym, cText)
	priceTxt := fmtPrice(t.Price)
	chgTxt := fmt.Sprintf("%+.2f%%", t.Change)
	chgC := cGreen
	if t.Change < 0 {
		chgC = cRed
	}
	drawASCII(fb, margin, 26, 3, priceTxt, cText)
	drawASCII(fb, margin+asciiWidth(3, priceTxt)+10, 32, 2, chgTxt, chgC)
	if favorite {
		drawFavoriteMark(fb, margin+asciiWidth(3, priceTxt)+asciiWidth(2, chgTxt)+20, 28, 18, true)
	}
	status := "TRỰC TUYẾN"
	sc := cGreen
	if stale {
		status = "DỮ LIỆU CŨ"
		sc = cRed
	}
	if chartLoading {
		status = "ĐANG TẢI"
		sc = cMuted
	}
	drawASCII(fb, margin, 56, 1, status, sc)
	if !updated.IsZero() {
		drawASCII(fb, margin+asciiWidth(1, status)+12, 56, 1, updated.Format("15:04 02/01"), cMuted)
	}
	metricY := 74
	metric1 := "H " + fmtPrice(t.High)
	metric2 := "L " + fmtPrice(t.Low)
	metric3 := "VOL " + fmtCompact(t.QuoteVolume)
	metric4 := "AUTO " + fmt.Sprintf("%ds", int(refreshEvery/time.Second))
	drawASCII(fb, margin, metricY, 1, metric1, cMuted)
	drawASCII(fb, margin+asciiWidth(1, metric1)+12, metricY, 1, metric2, cMuted)
	drawASCII(fb, margin+asciiWidth(1, metric1)+asciiWidth(1, metric2)+24, metricY, 1, metric3, cMuted)
	drawASCII(fb, fb.w-asciiWidth(1, metric4)-margin, metricY, 1, metric4, cMuted)
	// Tabs row
	tabsY := headerH - 12
	tabs := []string{"Biểu đồ", "Sổ lệnh", "Giao dịch", "24H"}
	tx := margin
	for i, tab := range tabs {
		cc := cMuted
		if i == 0 {
			cc = cText
		}
		drawASCII(fb, tx, tabsY, 1, tab, cc)
		if i == 0 {
			fb.rect(tx, tabsY+10, asciiWidth(1, tab), 2, cYellow)
		}
		tx += asciiWidth(1, tab) + 16
	}

	panelX := margin
	panelY := contentTop
	panelW := fb.w - 2*margin
	panelH := contentBottom - contentTop
	fb.rect(panelX, panelY, panelW, panelH, color{16, 18, 24})
	// Main candle chart area + Volume + MACD
	mainH := panelH * 62 / 100
	volH := panelH * 14 / 100
	macdH := panelH - mainH - volH - 18
	mainX, mainY, mainW := panelX+10, panelY+12, panelW-20
	renderTradingMainChart(fb, mainX, mainY, mainW, mainH, pts, t.Price, chartLoading, chartErr, rangeLabel)
	renderVolumePanel(fb, mainX, mainY+mainH+4, mainW, volH, pts)
	renderMACDPanel(fb, mainX, mainY+mainH+volH+8, mainW, macdH, pts)

	fb.rect(0, fb.h-footerH, fb.w, footerH, color{14, 15, 20})
	hint := "B: QUAY LẠI   Y: KHUNG TG   SELECT: GẮN SAO   MENU: THOÁT"
	drawASCII(fb, margin, fb.h-footerH+14, 1, hint, cMuted)
}

func renderTradingMainChart(fb *framebuffer, x, y, w, h int, pts []chartPoint, currentPrice float64, loading bool, errMsg, rangeLabel string) {
	fb.rect(x, y, w, h, color{14, 16, 22})
	drawASCII(fb, x+8, y+6, 1, "Chart  "+rangeLabel, cMuted)
	if loading && len(pts) == 0 {
		drawASCII(fb, x+8, y+28, 2, "ĐANG TẢI...", cMuted)
		return
	}
	if len(pts) < 2 {
		drawASCII(fb, x+8, y+28, 2, "KHÔNG CÓ DỮ LIỆU", cRed)
		if errMsg != "" {
			drawASCII(fb, x+8, y+54, 1, errMsg, cMuted)
		}
		return
	}
	minV, maxV := pts[0].Low, pts[0].High
	for _, p := range pts[1:] {
		if p.Low < minV {
			minV = p.Low
		}
		if p.High > maxV {
			maxV = p.High
		}
	}
	if currentPrice > 0 {
		if currentPrice < minV {
			minV = currentPrice
		}
		if currentPrice > maxV {
			maxV = currentPrice
		}
	}
	if maxV <= minV {
		maxV = minV + 1
	}
	pad := (maxV - minV) * 0.05
	minV -= pad
	maxV += pad
	plotX, plotY := x+8, y+22
	plotW, plotH := w-56, h-30
	for i := 0; i < 4; i++ {
		fb.hline(plotX, plotY+i*plotH/3, plotW, color{32, 35, 44})
	}
	for i := 1; i <= 3; i++ {
		fb.line(plotX+i*plotW/4, plotY, plotX+i*plotW/4, plotY+plotH, color{22, 24, 31})
	}
	denom := len(pts) - 1
	if denom < 1 {
		denom = 1
	}
	step := float64(plotW-2) / float64(denom)
	bodyW := int(step * 0.82)
	if bodyW < 3 {
		bodyW = 3
	}
	if bodyW > 10 {
		bodyW = 10
	}
	priceY := func(v float64) int { return plotY + plotH - int((v-minV)/(maxV-minV)*float64(plotH)) }
	closes := closeSeries(pts)
	ma5 := smaSeries(closes, 5)
	ma10 := smaSeries(closes, 10)
	mid := smaSeries(closes, 20)
	std := rollingStdSeries(closes, 20)
	bx5, by5 := make([]int, len(pts)), make([]int, len(pts))
	bx10, by10 := make([]int, len(pts)), make([]int, len(pts))
	bu, bmid, bl := make([]int, len(pts)), make([]int, len(pts)), make([]int, len(pts))
	for i, p := range pts {
		cx := plotX + int(float64(i)*step)
		yh, yl := priceY(p.High), priceY(p.Low)
		yo, yc := priceY(p.Open), priceY(p.Close)
		cc := cGreen
		if p.Close < p.Open {
			cc = cRed
		}
		fb.line(cx, yh, cx, yl, cc)
		top, bottom := yo, yc
		if top > bottom {
			top, bottom = bottom, top
		}
		if bottom-top < 2 {
			bottom = top + 2
		}
		fb.rect(cx-bodyW/2, top, bodyW, bottom-top+1, cc)
		if ma5[i] != 0 {
			bx5[i], by5[i] = cx, priceY(ma5[i])
		}
		if ma10[i] != 0 {
			bx10[i], by10[i] = cx, priceY(ma10[i])
		}
		if mid[i] != 0 {
			bu[i] = cx
			bmid[i] = cx
			bl[i] = cx
		}
	}
	// BOLL upper/mid/lower
	ux, uy := make([]int, len(pts)), make([]int, len(pts))
	mx, my := make([]int, len(pts)), make([]int, len(pts))
	lx, ly := make([]int, len(pts)), make([]int, len(pts))
	for i := range pts {
		cx := plotX + int(float64(i)*step)
		if mid[i] != 0 {
			upper := mid[i] + 2*std[i]
			lower := mid[i] - 2*std[i]
			ux[i], uy[i] = cx, priceY(upper)
			mx[i], my[i] = cx, priceY(mid[i])
			lx[i], ly[i] = cx, priceY(lower)
		}
	}
	drawPolyline(fb, ux, uy, 19, color{112, 142, 255})
	drawPolyline(fb, mx, my, 19, color{76, 110, 220})
	drawPolyline(fb, lx, ly, 19, color{112, 142, 255})
	drawPolyline(fb, bx5, by5, 4, color{255, 214, 90})
	drawPolyline(fb, bx10, by10, 9, color{255, 160, 78})
	drawASCII(fb, x+w-asciiWidth(1, "MA5 MA10 BOLL")-62, y+6, 1, "MA5", color{255, 214, 90})
	drawASCII(fb, x+w-asciiWidth(1, "MA10 BOLL")-34, y+6, 1, "MA10", color{255, 160, 78})
	drawASCII(fb, x+w-asciiWidth(1, "BOLL")-4, y+6, 1, "BOLL", color{112, 142, 255})
	// Đường xanh ngang biểu thị giá hiện tại, cập nhật theo ticker hiện tại.
	curr := currentPrice
	if curr <= 0 {
		curr = pts[len(pts)-1].Close
	}
	currY := priceY(curr)
	lineColor := color{82, 210, 120}
	fb.hline(plotX, currY, plotW, lineColor)
	currLbl := fmtPrice(curr)
	labelW := asciiWidth(1, currLbl) + 8
	labelX := x + w - labelW - 4
	labelY := currY - 6
	if labelY < y+22 {
		labelY = y + 22
	}
	if labelY > y+h-18 {
		labelY = y + h - 18
	}
	fb.rect(labelX, labelY, labelW, 12, lineColor)
	drawASCII(fb, labelX+4, labelY+2, 1, currLbl, cBg)
	// Price labels on right
	for i := 0; i < 4; i++ {
		v := maxV - (maxV-minV)*float64(i)/3
		lbl := fmtPrice(v)
		drawASCII(fb, x+w-asciiWidth(1, lbl)-8, plotY+i*plotH/3-3, 1, lbl, cMuted)
	}
}

func renderVolumePanel(fb *framebuffer, x, y, w, h int, pts []chartPoint) {
	fb.rect(x, y, w, h, color{18, 20, 27})
	if len(pts) < 2 {
		return
	}
	plotX, plotY := x+8, y+4
	plotW, plotH := w-16, h-8
	maxMag := 0.0
	mags := make([]float64, len(pts))
	for i, p := range pts {
		m := math.Abs(p.Close-p.Open) + (p.High-p.Low)*0.2
		mags[i] = m
		if m > maxMag {
			maxMag = m
		}
	}
	if maxMag <= 0 {
		maxMag = 1
	}
	step := float64(plotW) / float64(len(pts))
	barW := int(step * 0.7)
	if barW < 1 {
		barW = 1
	}
	for i, p := range pts {
		bh := int(mags[i] / maxMag * float64(plotH))
		cx := plotX + int(float64(i)*step)
		cc := color{125, 130, 146}
		if p.Close >= p.Open {
			cc = color{96, 154, 116}
		} else {
			cc = color{154, 89, 89}
		}
		fb.rect(cx, plotY+plotH-bh, barW, bh, cc)
	}
}

func renderMACDPanel(fb *framebuffer, x, y, w, h int, pts []chartPoint) {
	fb.rect(x, y, w, h, color{18, 18, 24})
	drawASCII(fb, x+8, y+4, 1, "MACD", cMuted)
	if len(pts) < 2 {
		return
	}
	closes := closeSeries(pts)
	dif, dea, hist := macdSeries(closes)
	plotX, plotY := x+6, y+16
	plotW, plotH := w-12, h-20
	zeroY := plotY + plotH/2
	fb.hline(plotX, zeroY, plotW, color{72, 72, 82})
	maxAbs := 0.0
	for i := range hist {
		if math.Abs(hist[i]) > maxAbs {
			maxAbs = math.Abs(hist[i])
		}
		if math.Abs(dif[i]) > maxAbs {
			maxAbs = math.Abs(dif[i])
		}
		if math.Abs(dea[i]) > maxAbs {
			maxAbs = math.Abs(dea[i])
		}
	}
	if maxAbs <= 0 {
		maxAbs = 1
	}
	denom := len(pts) - 1
	if denom < 1 {
		denom = 1
	}
	step := float64(plotW-2) / float64(denom)
	barW := int(step * 0.75)
	if barW < 1 {
		barW = 1
	}
	xd, yd := make([]int, len(pts)), make([]int, len(pts))
	xe, ye := make([]int, len(pts)), make([]int, len(pts))
	toY := func(v float64) int { return zeroY - int((v/maxAbs)*float64(plotH/2-2)) }
	for i := range pts {
		cx := plotX + int(float64(i)*step)
		bh := int(math.Abs(hist[i]) / maxAbs * float64(plotH/2-2))
		if hist[i] >= 0 {
			fb.rect(cx-barW/2, zeroY-bh, barW, bh, color{52, 220, 142})
		} else {
			fb.rect(cx-barW/2, zeroY, barW, bh, color{255, 82, 116})
		}
		xd[i], yd[i] = cx, toY(dif[i])
		xe[i], ye[i] = cx, toY(dea[i])
	}
	drawPolyline(fb, xd, yd, 1, color{255, 214, 90})
	drawPolyline(fb, xe, ye, 1, color{94, 161, 255})
	drawASCII(fb, x+w-asciiWidth(1, "DIF DEA")-8, y+4, 1, "DIF", color{255, 214, 90})
	drawASCII(fb, x+w-asciiWidth(1, "DEA")-8, y+4, 1, "DEA", color{94, 161, 255})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type fetchResult struct {
	xs  []ticker
	err error
}
type pairResult struct {
	pairs map[string]pairInfo
	err   error
}
type chartResult struct {
	symbol     string
	rangeIndex int
	pts        []chartPoint
	err        error
}
type chartCacheEntry struct {
	pts []chartPoint
	at  time.Time
	err string
}

func main() {
	fb, err := openFramebuffer()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer fb.close()
	ir := openInput()
	defer ir.close()

	assets := loadAssets()
	client := &http.Client{Timeout: 10 * time.Second}
	otaClient := &http.Client{Timeout: 5 * time.Minute}
	otaCfg := loadOTAConfig()
	otaCheckCh := make(chan otaCheckResult, 1)
	otaUpdateCh := make(chan otaUpdateResult, 1)
	otaProgressCh := make(chan otaProgressEvent, 64)
	otaCurrentProgress := otaProgressEvent{Stage: "download"}
	var otaChecking atomic.Bool
	var otaUpdating atomic.Bool
	otaPrompt := false
	otaChoice := 0 // 0 = HỦY, 1 = CẬP NHẬT
	otaAvailable := otaManifest{}
	otaStatusVisible := false
	otaStatusTitle := ""
	otaStatusMsg := ""
	otaRestartReady := false

	favorites := loadFavorites()
	settings := loadSettings()
	all, lastUpdated := loadCache()
	stale := len(all) > 0
	lastCacheSave := lastUpdated
	chartRange := settings.ChartRange
	refreshIndex := settings.RefreshIndex
	refreshEvery := refreshOptions[refreshIndex]
 gold:=goldSnapshot{}
 goldSel:=0
 goldErr:=""
 var goldLastAttempt time.Time
 var goldFetching atomic.Bool
 goldCh:=make(chan goldResult,1)
 newsCategorySel:=0
 newsArticleSel:=0
 newsView:=0 // grid, list, preview, gold
 newsItems:=[]newsArticle(nil)
 newsArticleActive:=newsArticle{}
 newsLastFetch:=time.Time{}
 newsLoading:=false
 newsErr:=""
 newsURL:=""
 newsKind:=""
 newsCh:=make(chan news24Result,8)
 doNewsFetch:=func(address,kind string) {
  if newsLoading && address==newsURL && kind==newsKind{return}
  newsURL=address;newsKind=kind;newsLoading=true;newsErr=""
  go func(){newsCh<-fetch24hNews(client,address,kind)}()
 }
 doGoldFetch:=func(){
  if goldFetching.Swap(true){return}
  goldLastAttempt=time.Now()
  go func(){
   g,e:=fetchGold24H(client)
   goldCh<-goldResult{Snapshot:g,Err:e}
   goldFetching.Store(false)
  }()
 }

	// Search is a temporary view within BINANCE; START always returns to its grid.
	page := pageFavorites
	favoriteSel := 0
	query := ""
	suggestions := []ticker(nil)
	suggSel := 0
	focusSuggestions := false
	kbRow, kbCol := 0, 0
	cursorVisible := true
	nextCursorBlink := time.Now().Add(500 * time.Millisecond)
	detail := false
	detailFrom := pageFavorites
	detailSymbol := ""
	errMsg := ""
	exitConfirm := false
	exitChoice := 0 // 0 = HỦY, 1 = ĐỒNG Ý

	refreshSuggestions := func() {
		suggestions = searchPairs(all, query, len(all))
		if suggSel >= len(suggestions) {
			suggSel = max(0, len(suggestions)-1)
		}
		if len(suggestions) == 0 {
			suggSel = 0
			focusSuggestions = false
		}
	}

	var fetching atomic.Bool
	fetchCh := make(chan fetchResult, 1)
	doFetch := func() {
		if fetching.Swap(true) {
			return
		}
		go func() {
			x, e := fetchTickers(client)
			fetchCh <- fetchResult{x, e}
			fetching.Store(false)
		}()
	}

	var pairFetching atomic.Bool
	pairCh := make(chan pairResult, 1)
	doPairFetch := func() {
		if pairFetching.Swap(true) {
			return
		}
		go func() {
			p, e := fetchPairInfo(client)
			pairCh <- pairResult{pairs: p, err: e}
			pairFetching.Store(false)
		}()
	}

	doOTACheck := func() {
		if !otaCfg.CheckOnStart || strings.TrimSpace(otaCfg.ManifestURL) == "" {
			return
		}
		if otaChecking.Swap(true) {
			return
		}
		go func() {
			m, e := fetchOTAManifest(otaClient, otaCfg.ManifestURL)
			otaCheckCh <- otaCheckResult{Manifest: m, Err: e}
			otaChecking.Store(false)
		}()
	}

	startOTAUpdate := func(m otaManifest) {
		if otaUpdating.Swap(true) {
			return
		}
		go func() {
            report := func(p otaProgressEvent) {
                select {
                case otaProgressCh <- p:
                default:
                    // Keep the newest progress event if the UI lags behind network.
                    select {case <-otaProgressCh: default:}
                    select {case otaProgressCh <- p: default:}
                }
            }
			e := performOTAWithProgress(otaClient, m, report)
			otaUpdateCh <- otaUpdateResult{Version: m.Version, Err: e}
			otaUpdating.Store(false)
		}()
	}

	chartCh := make(chan chartResult, 16)
	charts := map[string]chartCacheEntry{}
	chartInFlight := map[string]bool{}
	doChartFetch := func(symbol string, rangeIndex int, force bool) {
		if symbol == "" {
			return
		}
		if rangeIndex < 0 || rangeIndex >= len(chartRanges) {
			rangeIndex = 0
		}
		key := chartKey(symbol, rangeIndex)
		if !force {
			if c, ok := charts[key]; ok && len(c.pts) > 1 && time.Since(c.at) < 2*time.Minute {
				return
			}
		}
		if chartInFlight[key] {
			return
		}
		chartInFlight[key] = true
		go func(sym string, ri int) {
			p, e := fetchChart(client, sym, ri)
			chartCh <- chartResult{symbol: sym, rangeIndex: ri, pts: p, err: e}
		}(symbol, rangeIndex)
	}

	openDetail := func(sym string) {
		if sym == "" {
			return
		}
		if _, ok := findTicker(all, sym); !ok {
			return
		}
		detailFrom = page
		detail = true
		detailSymbol = sym
		doChartFetch(detailSymbol, chartRange, false)
	}

	dirty := true
	doFetch()
	doPairFetch()
	doOTACheck()
	nextFetch := time.Now().Add(refreshEvery)
	tick := time.NewTicker(16 * time.Millisecond)
	defer tick.Stop()

	saveAndExit := func() {
		if !lastUpdated.IsZero() && (lastCacheSave.IsZero() || lastUpdated.After(lastCacheSave)) {
			_ = saveCache(all, lastUpdated)
		}
		_ = saveSettings(settings)
	}

	for {
		select {
		case oc := <-otaCheckCh:
			if oc.Err == nil && isNewerVersion(oc.Manifest.Version, appVersion) {
				otaAvailable = oc.Manifest
				otaPrompt = true
				otaChoice = 0
				dirty = true
			}

		case p := <-otaProgressCh:
            if otaUpdating.Load() {
                otaCurrentProgress = p
                dirty = true
            }

		case ou := <-otaUpdateCh:
			otaStatusVisible = true
			if ou.Err != nil {
				otaStatusTitle = "CẬP NHẬT THẤT BẠI"
				otaStatusMsg = ou.Err.Error()
				otaRestartReady = false
			} else {
				otaStatusTitle = "CẬP NHẬT THÀNH CÔNG"
				otaStatusMsg = appVersion + " → " + ou.Version
				otaRestartReady = true
			}
			dirty = true

		case r := <-fetchCh:
			if r.err != nil {
				errMsg = r.err.Error()
				if len(all) > 0 {
					stale = true
				}
			} else {
				errMsg = ""
				all = r.xs
				lastUpdated = time.Now()
				stale = false
				if lastCacheSave.IsZero() || time.Since(lastCacheSave) >= cacheSaveInterval {
					if saveCache(all, lastUpdated) == nil {
						lastCacheSave = lastUpdated
					}
				}
			}
			refreshSuggestions()
			fav := favoriteTickers(all, favorites)
			if favoriteSel >= len(fav) {
				favoriteSel = max(0, len(fav)-1)
			}
			dirty = true

		case nr := <-newsCh:
            if nr.Key==newsURL && nr.Kind==newsKind {
                newsLoading=false
                newsLastFetch=nr.At
                if nr.Err!=nil {newsErr=nr.Err.Error()} else {
                    newsErr=""
                    if nr.Kind=="list" {
                       newsItems=nr.Items
                       newsArticleSel=0
                    } else {
                       if nr.Article.Title!="" {newsArticleActive.Title=nr.Article.Title}
                       newsArticleActive.Excerpt=nr.Article.Excerpt
                    }
                }
                dirty=true
            }

		case gr := <-goldCh:
   if gr.Err!=nil {goldErr=gr.Err.Error()} else {
     gold=gr.Snapshot
     goldErr=""
     if goldSel>=len(gold.Quotes){goldSel=max(0,len(gold.Quotes)-1)}
   }
   dirty=true

		case pr := <-pairCh:
			if pr.err == nil && len(pr.pairs) > 0 && len(all) > 0 {
				changed := false
				for i := range all {
					if pi, ok := pr.pairs[all[i].Symbol]; ok {
						if all[i].BaseAsset != pi.Base || all[i].QuoteAsset != pi.Quote {
							all[i].BaseAsset = pi.Base
							all[i].QuoteAsset = pi.Quote
							changed = true
						}
					}
				}
				if changed {
					refreshSuggestions()
					_ = saveCache(all, lastUpdated)
					dirty = true
				}
			}

		case cr := <-chartCh:
			key := chartKey(cr.symbol, cr.rangeIndex)
			delete(chartInFlight, key)
			entry := chartCacheEntry{pts: cr.pts, at: time.Now()}
			if cr.err != nil {
				entry.err = cr.err.Error()
				if old, ok := charts[key]; ok && len(old.pts) > 1 {
					entry.pts = old.pts
				}
			}
			charts[key] = entry
			dirty = true

		case <-tick.C:
			now := time.Now()
			if !detail && page == pageSearch && !now.Before(nextCursorBlink) {
				cursorVisible = !cursorVisible
				nextCursorBlink = now.Add(500 * time.Millisecond)
				dirty = true
			}
			if !detail && page==pageBusiness && newsView==3 && now.Sub(goldLastAttempt)>=goldRefreshInterval {doGoldFetch()}
			if now.After(nextFetch) {
				doFetch()
				nextFetch = time.Now().Add(refreshEvery)
			}
			for _, ac := range ir.poll() {
				if otaStatusVisible {
					if otaUpdating.Load() {
						continue
					}
					if otaRestartReady {
						switch ac {
						case actA, actSelect:
							saveAndExit()
							exe, e := os.Executable()
							if e == nil {
								e = syscall.Exec(exe, []string{exe}, os.Environ())
							}
							otaStatusTitle = "KHỞI ĐỘNG LẠI THẤT BẠI"
							if e != nil {
								otaStatusMsg = e.Error()
							}
							otaRestartReady = false
							dirty = true
						case actB, actExit:
							otaStatusVisible = false
							dirty = true
						}
					} else {
						switch ac {
						case actA, actB, actSelect, actExit:
							otaStatusVisible = false
							dirty = true
						}
					}
					continue
				}

				if otaPrompt {
					switch ac {
					case actLeft, actUp:
						otaChoice = 0
						dirty = true
					case actRight, actDown:
						otaChoice = 1
						dirty = true
					case actA, actSelect:
						if otaChoice == 1 {
							otaPrompt = false
							otaStatusVisible = true
							otaStatusTitle = "ĐANG CẬP NHẬT OTA"
							otaStatusMsg = "ĐANG TẢI VÀ KIỂM TRA SHA-256..."
							otaRestartReady = false
                            otaCurrentProgress = otaProgressEvent{Stage:"download",Done:0,Total:0}
							startOTAUpdate(otaAvailable)
						} else {
							otaPrompt = false
						}
						dirty = true
					case actB, actExit:
						otaPrompt = false
						dirty = true
					}
					continue
				}

				// Bàn phím USB: chỉ nhập trực tiếp khi đang ở tab Tìm kiếm.
				if ac >= actCharBase && !detail && page == pageSearch {
					r := rune(int(ac) - int(actCharBase))
					if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
						if len(query) < 12 {
							query += string(r)
						}
						suggSel = 0
						focusSuggestions = false
						refreshSuggestions()
						dirty = true
					}
					continue
				}

				if exitConfirm {
					switch ac {
					case actLeft, actUp:
						exitChoice = 0
						dirty = true
					case actRight, actDown:
						exitChoice = 1
						dirty = true
					case actA, actSelect:
						if exitChoice == 1 {
							saveAndExit()
							return
						}
						exitConfirm = false
						dirty = true
					case actB:
						exitConfirm = false
						dirty = true
					case actExit:
						exitConfirm = false
						dirty = true
					}
					continue
				}

				switch ac {
				case actExit:
					if exitConfirm {
						exitConfirm = false
					} else {
						exitConfirm = true
						exitChoice = 0
					}
					dirty = true
					continue


				case actUp:
					if detail {
						// Keep selected coin while viewing chart.
					} else if page==pageBusiness || page==pageHitech {
                        switch newsView {
                        case 0: if newsCategorySel>=2 {newsCategorySel-=2;dirty=true}
                        case 1: if newsArticleSel>0 {newsArticleSel--;dirty=true}
                        case 3: if goldSel>0 {goldSel--;dirty=true}
                        }
					} else if page == pageFavorites {
						if favoriteSel >= 2 {favoriteSel -= 2;dirty = true}
					} else if focusSuggestions {
						if len(suggestions)>0 {
							if suggSel>0 {suggSel--} else {suggSel=len(suggestions)-1}
							dirty=true
						}
					} else {
						nr,nc,exited:=moveKeyboardBounded(kbRow,kbCol,-1,0)
						if exited && len(suggestions)>0 {focusSuggestions=true;suggSel=len(suggestions)-1} else {kbRow,kbCol=nr,nc}
						dirty=true
					}


				case actDown:
					if detail {
					} else if page==pageBusiness || page==pageHitech {
                        switch newsView {
                        case 0: if newsCategorySel+2<len(newsCategories(page)){newsCategorySel+=2;dirty=true}
                        case 1: if newsArticleSel+1<len(newsItems){newsArticleSel++;dirty=true}
                        case 3: if goldSel+1<len(gold.Quotes){goldSel++;dirty=true}
                        }
					} else if page==pageFavorites {
						fav:=favoriteTickers(all,favorites)
						if favoriteSel+2<len(fav){favoriteSel+=2;dirty=true}
					} else if focusSuggestions {
						if len(suggestions)>0 {suggSel=(suggSel+1)%len(suggestions);dirty=true}
					} else {
						nr,nc,exited:=moveKeyboardBounded(kbRow,kbCol,1,0)
						if exited && len(suggestions)>0 {focusSuggestions=true;suggSel=len(suggestions)-1} else {kbRow,kbCol=nr,nc}
						dirty=true
					}


				case actLeft:
					if detail {
					} else if page==pageBusiness || page==pageHitech {
                       if newsView==0 && newsCategorySel%2==1 {newsCategorySel--;dirty=true}
					} else if page==pageFavorites {
						if favoriteSel%2==1{favoriteSel--;dirty=true}
					} else if focusSuggestions {
						focusSuggestions=false;dirty=true
					} else {
						nr,nc,exited:=moveKeyboardBounded(kbRow,kbCol,0,-1)
						if exited && len(suggestions)>0 {focusSuggestions=true;suggSel=len(suggestions)-1} else {kbRow,kbCol=nr,nc}
						dirty=true
					}


				case actRight:
					if detail {
					} else if page==pageBusiness || page==pageHitech {
                       if newsView==0 && newsCategorySel%2==0 && newsCategorySel+1<len(newsCategories(page)){newsCategorySel++;dirty=true}
					} else if page==pageFavorites {
						fav:=favoriteTickers(all,favorites)
						if favoriteSel%2==0 && favoriteSel+1<len(fav){favoriteSel++;dirty=true}
					} else if focusSuggestions {
						focusSuggestions=false;dirty=true
					} else {
						nr,nc,exited:=moveKeyboardBounded(kbRow,kbCol,0,1)
						if exited && len(suggestions)>0 {focusSuggestions=true;suggSel=len(suggestions)-1} else {kbRow,kbCol=nr,nc}
						dirty=true
					}

				case actA:
					if detail {
						// Chỉ xem dữ liệu, không đặt lệnh.
					} else if page == pageBusiness {
      // Chỉ xem bảng giá, không giao dịch.
     } else if page == pageFavorites {
						fav := favoriteTickers(all, favorites)
						if len(fav) > 0 && favoriteSel < len(fav) {
							openDetail(fav[favoriteSel].Symbol)
							dirty = true
						}
					} else if page == pageSearch && focusSuggestions && len(suggestions) > 0 {
						// A opens the selected pair in-place on BINANCE, without the keyboard.
						openDetail(suggestions[suggSel].Symbol)
						detailFrom = pageFavorites
						page = pageFavorites
						query = ""
						focusSuggestions = false
						refreshSuggestions()
						dirty = true
					} else {
						key := searchKeyboard[kbRow][kbCol]
						query = applySearchKey(query, key)
						suggSel = 0
						refreshSuggestions()
						dirty = true
					}

				case actB:
					if detail {
						// Trong màn hình chi tiết, B luôn quay lại đúng màn hình đã mở chi tiết.
						detail = false
						page = detailFrom
						dirty = true
					} else if page == pageSearch && query != "" {
						// Ngoài màn hình chi tiết, B vẫn chỉ xóa một ký tự tìm kiếm.
						query = backspaceQuery(query)
						suggSel = 0
						focusSuggestions = false
						refreshSuggestions()
						dirty = true
					}

				case actX:
					if detail {
						doFetch()
						doPairFetch()
						nextFetch = time.Now().Add(refreshEvery)
						doChartFetch(detailSymbol, chartRange, true)
					} else if page == pageBusiness {
      doGoldFetch()
     } else if page == pageFavorites {
						// Tab Yêu thích dùng tự động cập nhật, X không còn chức năng.
					} else if page == pageSearch {
						query = ""
						suggSel = 0
						focusSuggestions = false
						refreshSuggestions()
					}
					dirty = true

				case actY:
					if detail {
						chartRange = (chartRange + 1) % len(chartRanges)
						settings.ChartRange = chartRange
						_ = saveSettings(settings)
						doChartFetch(detailSymbol, chartRange, false)
						dirty = true
					} else if page == pageSearch && len(suggestions) > 0 {
						// Y chuyển qua lại giữa vùng gợi ý và vùng bàn phím.
						focusSuggestions = !focusSuggestions
						if focusSuggestions && suggSel >= len(suggestions) {
							suggSel = len(suggestions) - 1
						}
						dirty = true
					}

				case actL1, actR1:
					// Only two top-level tabs. Search is an internal BINANCE view.
					detail = false
					if page == pageSearch {
						query = ""
						suggSel = 0
						focusSuggestions = false
						refreshSuggestions()
					}
					if ac == actR1 {page = cycleMainPage(page, 1)} else {page = cycleMainPage(page, -1)}
					if page == pageBusiness && (gold.FetchedAt.IsZero() || time.Since(gold.FetchedAt) > goldRefreshInterval) && time.Since(goldLastAttempt) > 10*time.Second {doGoldFetch()}
					dirty = true

				case actSelect:
					if detail {
						sym := detailSymbol
						if sym != "" {
							favorites[sym] = !favorites[sym]
							if !favorites[sym] {
								delete(favorites, sym)
							}
							_ = saveFavorites(favorites)
							dirty = true
						}
					} else if page == pageBusiness {
      // Chỉ xem bảng giá, không giao dịch.
     } else if page == pageFavorites {
						fav := favoriteTickers(all, favorites)
						if len(fav) > 0 && favoriteSel < len(fav) {
							delete(favorites, fav[favoriteSel].Symbol)
							_ = saveFavorites(favorites)
							fav = favoriteTickers(all, favorites)
							if favoriteSel >= len(fav) {
								favoriteSel = max(0, len(fav)-1)
							}
							dirty = true
						}
					} else if page == pageSearch && len(suggestions) > 0 {
						sym := suggestions[suggSel].Symbol
						if favorites[sym] {
							delete(favorites, sym)
						} else {
							favorites[sym] = true
							fav := favoriteTickers(all, favorites)
							favoriteSel = findIndex(fav, sym)
							if favoriteSel < 0 {
								favoriteSel = 0
							}
						}
						_ = saveFavorites(favorites)
						dirty = true
					}

				case actStart:
					if page == pageBusiness {break}
					if page == pageSearch {
						// START hides the keyboard and returns to the last opened pair.
						page = pageFavorites
						if detailSymbol != "" {
							if _, ok := findTicker(all, detailSymbol); ok {
								detail = true
								detailFrom = pageFavorites
								doChartFetch(detailSymbol, chartRange, false)
							}
						}
					} else {
						// START reveals the search field and keyboard without leaving BINANCE.
						detail = false
						page = pageSearch
						query = ""
						suggSel = 0
						focusSuggestions = false
						kbRow, kbCol = 0, 0
						refreshSuggestions()
					}
					dirty = true
				}
			}

			if dirty {
				if detail {
					if t, ok := findTicker(all, detailSymbol); ok {
						key := chartKey(detailSymbol, chartRange)
						ce := charts[key]
						renderDetail(fb, assets, t, ce.pts, chartInFlight[key], ce.err, favorites[detailSymbol], lastUpdated, stale, chartRanges[chartRange].Label, refreshEvery)
					} else {
						detail = false
						page = detailFrom
						dirty = true
					}
				} else if page == pageBusiness {
     renderBusiness(fb,gold,goldSel,goldFetching.Load(),goldErr)
    } else if page == pageFavorites {
					fav := favoriteTickers(all, favorites)
					if favoriteSel >= len(fav) {
						favoriteSel = max(0, len(fav)-1)
					}
					start, end := favoriteGridWindow(len(fav), favoriteSel)
					for i := start; i < end; i++ {
						doChartFetch(fav[i].Symbol, 0, false)
					}
					renderFavoritesGrid(fb, fav, favoriteSel, charts, fetching.Load(), errMsg, lastUpdated, stale)
				} else {
					renderSearch(fb, assets, query, suggestions, suggSel, focusSuggestions, kbRow, kbCol, cursorVisible, fetching.Load(), errMsg, lastUpdated, stale, favorites)
				}
				if exitConfirm {
					renderExitConfirm(fb, exitChoice)
				}
				if otaPrompt {
					renderOTAPrompt(fb, otaAvailable, otaChoice)
				}
				if otaStatusVisible {
                    if otaUpdating.Load() {
                        renderOTAProgress(fb,otaCurrentProgress,otaAvailable.Version)
                    } else {
                        renderOTAStatus(fb, otaStatusTitle, otaStatusMsg, otaRestartReady)
                    }
				}
				fb.flush()
				dirty = false
			}
		}
	}
}