package services

import qrcode "github.com/skip2/go-qrcode"

type QRCodeRenderer struct {
	size int
}

func NewQRCodeRenderer(size int) QRCodeRenderer {
	if size <= 0 {
		size = 320
	}
	return QRCodeRenderer{size: size}
}

func (r QRCodeRenderer) RenderPNG(payload string) ([]byte, error) {
	return qrcode.Encode(payload, qrcode.Medium, r.size)
}
