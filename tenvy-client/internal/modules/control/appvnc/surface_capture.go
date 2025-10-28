package appvnc

import (
	"context"
	"errors"
	"image"

	"github.com/kbinani/screenshot"
	"github.com/rootbay/tenvy-client/internal/modules/control/screen"
)

type screenshotSurfaceCapturer struct {
	bounds image.Rectangle
}

func defaultSurfaceCaptureFactory(*sessionState) (surfaceCapturer, error) {
	displays := screenshot.NumActiveDisplays()
	if displays <= 0 {
		return nil, errors.New("no active displays")
	}
	bounds := screenshot.GetDisplayBounds(0)
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, errors.New("invalid display bounds")
	}
	return &screenshotSurfaceCapturer{bounds: bounds}, nil
}

func (c *screenshotSurfaceCapturer) Capture(ctx context.Context) (*surfaceFrame, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
	img, err := screen.SafeCaptureRect(c.bounds)
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, errors.New("nil capture result")
	}
	frame := &surfaceFrame{
		image: &surfaceImage{
			width:  img.Rect.Dx(),
			height: img.Rect.Dy(),
			stride: img.Stride,
			data:   append([]byte(nil), img.Pix...),
		},
	}
	return frame, nil
}

func (c *screenshotSurfaceCapturer) Close() error {
	return nil
}
