package screen

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"sync"

	"github.com/kbinani/screenshot"
)

var (
	pngEncoder      = png.Encoder{CompressionLevel: png.BestSpeed}
	imageBufferPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
)

// SafeCaptureRect captures the specified screen rectangle, recovering from
// underlying platform panics that can be triggered by transient display driver
// changes. A nil image with an error is returned when capture fails.
func SafeCaptureRect(bounds image.Rectangle) (img *image.RGBA, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("capture panic: %v", r)
			img = nil
		}
	}()

	return screenshot.CaptureRect(bounds)
}

// EncodeRGBAAsPNG encodes the provided RGBA buffer to a base64 PNG payload.
func EncodeRGBAAsPNG(width, height int, data []byte) (string, error) {
	if len(data) == 0 || width <= 0 || height <= 0 {
		return "", errors.New("invalid frame data")
	}

	img := &image.RGBA{
		Pix:    data,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	bufPtr := imageBufferPool.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer imageBufferPool.Put(bufPtr)

	if err := pngEncoder.Encode(bufPtr, img); err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(bufPtr.Bytes())
	return encoded, nil
}

// EncodeRGBAAsJPEG encodes the provided RGBA buffer to a base64 JPEG payload
// using the supplied quality value.
func EncodeRGBAAsJPEG(width, height, quality int, data []byte) (string, error) {
	if len(data) == 0 || width <= 0 || height <= 0 {
		return "", errors.New("invalid frame data")
	}

	if quality <= 0 {
		quality = jpeg.DefaultQuality
	}
	img := &image.RGBA{
		Pix:    data,
		Stride: width * 4,
		Rect:   image.Rect(0, 0, width, height),
	}
	bufPtr := imageBufferPool.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer imageBufferPool.Put(bufPtr)

	if err := jpeg.Encode(bufPtr, img, &jpeg.Options{Quality: quality}); err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(bufPtr.Bytes())
	return encoded, nil
}
