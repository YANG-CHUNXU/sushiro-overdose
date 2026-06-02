package app

import (
	"fmt"
	"html"
	"strings"
)

const (
	qrVersion       = 5
	qrSize          = 21 + 4*(qrVersion-1)
	qrDataCodewords = 108
	qrECCCodewords  = 26
)

var (
	qrGFExp [512]byte
	qrGFLog [256]int
)

func init() {
	x := 1
	for i := 0; i < 255; i++ {
		qrGFExp[i] = byte(x)
		qrGFLog[x] = i
		x <<= 1
		if x&0x100 != 0 {
			x ^= 0x11D
		}
	}
	for i := 255; i < len(qrGFExp); i++ {
		qrGFExp[i] = qrGFExp[i-255]
	}
}

func qrSVG(text string) string {
	data := []byte(text)
	if len(data) > 106 {
		return ""
	}
	codewords := qrCodewords(data)
	modules, reserved := qrBaseMatrix()
	qrDrawCodewords(modules, reserved, codewords)
	qrDrawFormat(modules, reserved)
	return qrRenderSVG(modules)
}

func qrCodewords(data []byte) []byte {
	var bits []bool
	qrAppendBits(&bits, 0b0100, 4)
	qrAppendBits(&bits, len(data), 8)
	for _, b := range data {
		qrAppendBits(&bits, int(b), 8)
	}
	capBits := qrDataCodewords * 8
	for i := 0; i < 4 && len(bits) < capBits; i++ {
		bits = append(bits, false)
	}
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	dataCodewords := make([]byte, 0, qrDataCodewords)
	for i := 0; i < len(bits); i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			b <<= 1
			if bits[i+j] {
				b |= 1
			}
		}
		dataCodewords = append(dataCodewords, b)
	}
	for pad := 0; len(dataCodewords) < qrDataCodewords; pad++ {
		if pad%2 == 0 {
			dataCodewords = append(dataCodewords, 0xEC)
		} else {
			dataCodewords = append(dataCodewords, 0x11)
		}
	}
	ecc := qrReedSolomon(dataCodewords, qrECCCodewords)
	return append(dataCodewords, ecc...)
}

func qrAppendBits(bits *[]bool, value, count int) {
	for i := count - 1; i >= 0; i-- {
		*bits = append(*bits, ((value>>i)&1) != 0)
	}
}

func qrReedSolomon(data []byte, degree int) []byte {
	gen := []byte{1}
	for i := 0; i < degree; i++ {
		next := make([]byte, len(gen)+1)
		for j, coef := range gen {
			next[j] ^= coef
			next[j+1] ^= qrGFMul(coef, qrGFExp[i])
		}
		gen = next
	}
	result := make([]byte, degree)
	for _, b := range data {
		factor := b ^ result[0]
		copy(result, result[1:])
		result[degree-1] = 0
		for i := 0; i < degree; i++ {
			result[i] ^= qrGFMul(gen[i+1], factor)
		}
	}
	return result
}

func qrGFMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return qrGFExp[qrGFLog[a]+qrGFLog[b]]
}

func qrBaseMatrix() ([][]bool, [][]bool) {
	modules := make([][]bool, qrSize)
	reserved := make([][]bool, qrSize)
	for i := range modules {
		modules[i] = make([]bool, qrSize)
		reserved[i] = make([]bool, qrSize)
	}
	set := func(x, y int, value bool) {
		if x < 0 || y < 0 || x >= qrSize || y >= qrSize {
			return
		}
		modules[y][x] = value
		reserved[y][x] = true
	}
	qrDrawFinder(set, 0, 0)
	qrDrawFinder(set, qrSize-7, 0)
	qrDrawFinder(set, 0, qrSize-7)
	for i := 8; i < qrSize-8; i++ {
		black := i%2 == 0
		set(i, 6, black)
		set(6, i, black)
	}
	qrDrawAlignment(set, 30, 30)
	qrReserveFormat(reserved)
	set(8, qrSize-8, true)
	return modules, reserved
}

func qrDrawFinder(set func(int, int, bool), x, y int) {
	for dy := -1; dy <= 7; dy++ {
		for dx := -1; dx <= 7; dx++ {
			xx, yy := x+dx, y+dy
			if dx < 0 || dx > 6 || dy < 0 || dy > 6 {
				set(xx, yy, false)
				continue
			}
			black := dx == 0 || dx == 6 || dy == 0 || dy == 6 || (dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4)
			set(xx, yy, black)
		}
	}
}

func qrDrawAlignment(set func(int, int, bool), cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			black := dx == -2 || dx == 2 || dy == -2 || dy == 2 || (dx == 0 && dy == 0)
			set(cx+dx, cy+dy, black)
		}
	}
}

func qrReserveFormat(reserved [][]bool) {
	positions := [][2]int{}
	for i := 0; i <= 5; i++ {
		positions = append(positions, [2]int{8, i})
	}
	positions = append(positions, [][2]int{{8, 7}, {8, 8}, {7, 8}}...)
	for i := 9; i < 15; i++ {
		positions = append(positions, [2]int{14 - i, 8})
	}
	for i := 0; i < 8; i++ {
		positions = append(positions, [2]int{qrSize - 1 - i, 8})
	}
	for i := 8; i < 15; i++ {
		positions = append(positions, [2]int{8, qrSize - 15 + i})
	}
	for _, p := range positions {
		x, y := p[0], p[1]
		if x >= 0 && y >= 0 && x < qrSize && y < qrSize {
			reserved[y][x] = true
		}
	}
}

func qrDrawCodewords(modules, reserved [][]bool, codewords []byte) {
	bits := make([]bool, 0, len(codewords)*8)
	for _, b := range codewords {
		for i := 7; i >= 0; i-- {
			bits = append(bits, ((b>>i)&1) != 0)
		}
	}
	bitIndex := 0
	upward := true
	for right := qrSize - 1; right >= 1; right -= 2 {
		if right == 6 {
			right--
		}
		for vert := 0; vert < qrSize; vert++ {
			y := vert
			if upward {
				y = qrSize - 1 - vert
			}
			for j := 0; j < 2; j++ {
				x := right - j
				if reserved[y][x] {
					continue
				}
				bit := false
				if bitIndex < len(bits) {
					bit = bits[bitIndex]
					bitIndex++
				}
				if (x+y)%2 == 0 {
					bit = !bit
				}
				modules[y][x] = bit
			}
		}
		upward = !upward
	}
}

func qrDrawFormat(modules, reserved [][]bool) {
	format := qrFormatBits()
	set := func(x, y, bit int) {
		modules[y][x] = ((format >> bit) & 1) != 0
		reserved[y][x] = true
	}
	for i := 0; i <= 5; i++ {
		set(8, i, i)
	}
	set(8, 7, 6)
	set(8, 8, 7)
	set(7, 8, 8)
	for i := 9; i < 15; i++ {
		set(14-i, 8, i)
	}
	for i := 0; i < 8; i++ {
		set(qrSize-1-i, 8, i)
	}
	for i := 8; i < 15; i++ {
		set(8, qrSize-15+i, i)
	}
	modules[qrSize-8][8] = true
	reserved[qrSize-8][8] = true
}

func qrFormatBits() int {
	data := 0b01 << 3 // error correction L, mask 0.
	bits := data << 10
	for i := 14; i >= 10; i-- {
		if ((bits >> i) & 1) != 0 {
			bits ^= 0x537 << (i - 10)
		}
	}
	return ((data << 10) | bits) ^ 0x5412
}

func qrRenderSVG(modules [][]bool) string {
	const border = 4
	size := qrSize + border*2
	var path strings.Builder
	for y := 0; y < qrSize; y++ {
		for x := 0; x < qrSize; x++ {
			if modules[y][x] {
				path.WriteString(fmt.Sprintf("M%d %dh1v1h-1z", x+border, y+border))
			}
		}
	}
	return `<svg class="ua-qr" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` + fmt.Sprint(size) + ` ` + fmt.Sprint(size) + `" role="img" aria-label="UA capture QR"><rect width="100%" height="100%" fill="#fff"/><path fill="#111" d="` + html.EscapeString(path.String()) + `"/></svg>`
}
