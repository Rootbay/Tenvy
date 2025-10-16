package remotedesktop

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type clipFrameBuffer struct {
	OffsetMs int
	Width    int
	Height   int
	Buffer   []byte
}

type clipVideoEncoder interface {
	EncodeClip(frames []clipFrameBuffer, opts clipEncodeOptions) (clipEncodeResult, error)
	Close() error
}

type clipEncodeOptions struct {
	Width         int
	Height        int
	Quality       int
	ForceKey      bool
	TargetBitrate int
	FrameInterval time.Duration
}

type clipEncodeResult struct {
	Frames      []RemoteDesktopClipFrame
	Bytes       int
	Encoding    string
	EncoderName string
}

type ffmpegHEVCEncoder struct {
	ffmpegPath string
	candidates []ffmpegHEVCCandidate
}

type ffmpegHEVCCandidate struct {
	name      string
	encoder   string
	filter    string
	extraArgs []string
}

func newHEVCVideoEncoder() (clipVideoEncoder, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg binary not found: %w", err)
	}
	encoder := &ffmpegHEVCEncoder{
		ffmpegPath: path,
		candidates: []ffmpegHEVCCandidate{
			{name: "NVENC", encoder: "hevc_nvenc", filter: "format=yuv420p"},
			{name: "QuickSync", encoder: "hevc_qsv", filter: "format=yuv420p"},
			{name: "AMF", encoder: "hevc_amf", filter: "format=yuv420p"},
		},
	}
	return encoder, nil
}

func (e *ffmpegHEVCEncoder) Close() error {
	return nil
}

func (e *ffmpegHEVCEncoder) EncodeClip(frames []clipFrameBuffer, opts clipEncodeOptions) (clipEncodeResult, error) {
	if len(frames) == 0 {
		return clipEncodeResult{}, errors.New("no frames provided for encoding")
	}

	bitrate := estimateClipBitrate(opts.Width, opts.Height, opts.Quality, opts.TargetBitrate)
	fps := estimateClipFPS(frames, opts.FrameInterval)
	gop := clampInt(int(math.Round(fps)), 1, 300)
	if opts.ForceKey {
		gop = 1
	}

	var lastErr error
	for _, candidate := range e.candidates {
		data, err := e.encodeWithCandidate(candidate, frames, opts, fps, bitrate, gop)
		if err != nil {
			lastErr = err
			continue
		}
		if len(data) == 0 {
			lastErr = fmt.Errorf("%s encoder produced no data", candidate.name)
			continue
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		return clipEncodeResult{
			Frames: []RemoteDesktopClipFrame{{
				OffsetMs: 0,
				Width:    opts.Width,
				Height:   opts.Height,
				Encoding: remoteClipEncodingHEVC,
				Data:     encoded,
			}},
			Bytes:       len(encoded),
			Encoding:    remoteClipEncodingHEVC,
			EncoderName: candidate.name,
		}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no HEVC encoder candidates succeeded")
	}
	return clipEncodeResult{}, lastErr
}

func (e *ffmpegHEVCEncoder) encodeWithCandidate(
	candidate ffmpegHEVCCandidate,
	frames []clipFrameBuffer,
	opts clipEncodeOptions,
	fps float64,
	bitrate int,
	gop int,
) ([]byte, error) {
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "rawvideo",
		"-pix_fmt", "bgra",
		"-video_size", fmt.Sprintf("%dx%d", opts.Width, opts.Height),
	}
	if fps > 0 {
		args = append(args, "-framerate", strconv.FormatFloat(fps, 'f', 3, 64))
	}
	args = append(args, "-i", "pipe:0")

	filter := strings.TrimSpace(candidate.filter)
	if filter == "" {
		filter = "format=yuv420p"
	}
	args = append(args, "-vf", filter)
	args = append(args, "-c:v", candidate.encoder)
	args = append(args, "-g", strconv.Itoa(gop))
	args = append(args, "-bf", "0")

	rate := fmt.Sprintf("%dk", bitrate)
	args = append(args, "-b:v", rate, "-maxrate", rate, "-bufsize", fmt.Sprintf("%dk", clampInt(bitrate*2, bitrate, 100000)))

	if len(candidate.extraArgs) > 0 {
		args = append(args, candidate.extraArgs...)
	}

	args = append(args, "-f", "hevc", "pipe:1")

	cmd := exec.Command(e.ffmpegPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}

	frameSize := opts.Width * opts.Height * 4
	writeErr := func(err error) ([]byte, error) {
		stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s encoder failed: %w: %s", candidate.name, err, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("%s encoder failed: %w", candidate.name, err)
	}

	for _, frame := range frames {
		if len(frame.Buffer) < frameSize {
			return writeErr(fmt.Errorf("frame buffer too small"))
		}
		if _, err := stdin.Write(frame.Buffer[:frameSize]); err != nil {
			return writeErr(err)
		}
	}
	if err := stdin.Close(); err != nil {
		return writeErr(err)
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		return writeErr(err)
	}

	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s encoder failed: %w: %s", candidate.name, err, strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("%s encoder failed: %w", candidate.name, err)
	}

	return data, nil
}

func estimateClipFPS(frames []clipFrameBuffer, interval time.Duration) float64 {
	if len(frames) <= 1 {
		if interval <= 0 {
			return 30
		}
		ms := float64(interval.Milliseconds())
		if ms <= 0 {
			return 30
		}
		return 1000 / ms
	}
	durationMs := frames[len(frames)-1].OffsetMs - frames[0].OffsetMs
	if durationMs <= 0 {
		if interval <= 0 {
			return 30
		}
		ms := float64(interval.Milliseconds())
		if ms <= 0 {
			return 30
		}
		return 1000 / ms
	}
	return float64(len(frames)-1) * 1000 / float64(durationMs)
}

func estimateClipBitrate(width, height, quality, target int) int {
	if target > 0 {
		return clampInt(target, 600, 40000)
	}
	pixels := width * height
	base := 1500
	switch {
	case pixels >= 3840*2160:
		base = 18000
	case pixels >= 2560*1440:
		base = 11000
	case pixels >= 1920*1080:
		base = 6000
	case pixels >= 1280*720:
		base = 3500
	case pixels >= 1024*768:
		base = 2500
	default:
		base = 1500
	}
	quality = clampInt(quality, minClipQuality, maxClipQuality)
	scale := float64(quality) / float64(defaultClipQuality)
	scale = math.Max(0.5, math.Min(1.6, scale))
	bitrate := int(float64(base) * scale)
	return clampInt(bitrate, 800, 40000)
}

func encodeClipFramesJPEG(frames []clipFrameBuffer, quality int) ([]RemoteDesktopClipFrame, int, error) {
	if len(frames) == 0 {
		return nil, 0, errors.New("no frames available")
	}
	encoded := make([]RemoteDesktopClipFrame, len(frames))
	total := 0
	for idx, frame := range frames {
		data, err := encodeJPEG(frame.Width, frame.Height, quality, frame.Buffer)
		if err != nil {
			return nil, 0, err
		}
		encoded[idx] = RemoteDesktopClipFrame{
			OffsetMs: frame.OffsetMs,
			Width:    frame.Width,
			Height:   frame.Height,
			Encoding: remoteClipEncodingJPEG,
			Data:     data,
		}
		total += len(data)
	}
	return encoded, total, nil
}
