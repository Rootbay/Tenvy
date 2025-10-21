package remotedesktopengine

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeClipEncoder struct {
	queueCalls []bool
	flushCalls []bool
	result     clipEncodeResult
	queueErr   error
	flushErr   error
}

func (f *fakeClipEncoder) QueueFrame(frame clipFrameBuffer, opts clipEncodeOptions, forceKey bool) error {
	f.queueCalls = append(f.queueCalls, forceKey)
	if f.queueErr != nil {
		return f.queueErr
	}
	if len(f.result.Frames) == 0 {
		data := []byte{0x01, 0x02, 0x03}
		f.result = clipEncodeResult{
			Frames: []RemoteDesktopClipFrame{{
				OffsetMs: frame.OffsetMs,
				Width:    frame.Width,
				Height:   frame.Height,
				Encoding: remoteClipEncodingHEVC,
				Data:     data,
			}},
			Bytes:       len(data),
			Encoding:    remoteClipEncodingHEVC,
			EncoderName: "fake",
		}
	}
	return nil
}

func (f *fakeClipEncoder) Flush(forceKey bool) (clipEncodeResult, error) {
	f.flushCalls = append(f.flushCalls, forceKey)
	if f.flushErr != nil {
		return clipEncodeResult{}, f.flushErr
	}
	if len(f.result.Frames) == 0 {
		return clipEncodeResult{}, nil
	}
	clone := clipEncodeResult{
		Frames:      make([]RemoteDesktopClipFrame, len(f.result.Frames)),
		Bytes:       f.result.Bytes,
		Encoding:    f.result.Encoding,
		EncoderName: f.result.EncoderName,
	}
	for i, frame := range f.result.Frames {
		clone.Frames[i] = RemoteDesktopClipFrame{
			OffsetMs: frame.OffsetMs,
			Width:    frame.Width,
			Height:   frame.Height,
			Encoding: frame.Encoding,
			Data:     append([]byte(nil), frame.Data...),
		}
	}
	return clone, nil
}

func (f *fakeClipEncoder) Close() error { return nil }

func TestAnnexBNALParserDetectsKeyframes(t *testing.T) {
	h264 := &annexBNALParser{codec: remoteClipEncodingH264}
	offsets := h264.push([]byte{0x00, 0x00, 0x01, 0x65, 0x88, 0x99})
	if len(offsets) != 1 || offsets[0] != 3 {
		t.Fatalf("expected single h264 keyframe offset, got %v", offsets)
	}

	hevc := &annexBNALParser{codec: remoteClipEncodingHEVC}
	offsets = hevc.push([]byte{0x00, 0x00, 0x01, 0x26, 0x01, 0x02})
	if len(offsets) != 1 || offsets[0] != 3 {
		t.Fatalf("expected single hevc keyframe offset, got %v", offsets)
	}

	parser := &annexBNALParser{codec: remoteClipEncodingH264}
	part1 := []byte{0x00, 0x00}
	part2 := []byte{0x01, 0x65, 0x01}
	if len(parser.push(part1)) != 0 {
		t.Fatalf("unexpected detection in partial chunk")
	}
	offsets = parser.push(part2)
	if len(offsets) != 1 {
		t.Fatalf("expected single keyframe after split chunk, got %v", offsets)
	}
}

func TestStreamStateQueuesAndFlushes(t *testing.T) {
	controller := &remoteDesktopSessionController{}
	controller.updateConfig(Config{})

	state := newStreamLoopState(RemoteStreamModeVideo)
	fake := &fakeClipEncoder{}
	state.clipEncoders[remoteClipEncodingHEVC] = &clipEncoderState{encoder: fake, init: true}
	state.activeClipKind = remoteClipEncodingHEVC
	state.clipKeyPending = true

	frame := clipFrameBuffer{
		OffsetMs: 0,
		Width:    4,
		Height:   4,
		Buffer:   make([]byte, 4*4*4),
	}
	state.clipFrames = append(state.clipFrames, frame)

	session := &RemoteDesktopSession{TargetBitrateKbps: 1200}
	snapshot := sessionSnapshot{
		width:           frame.Width,
		height:          frame.Height,
		clipQuality:     80,
		negotiatedCodec: RemoteEncoderHEVC,
		activeEncoder:   RemoteEncoderHEVC,
	}
	interval := 33 * time.Millisecond

	state.queueClipFrame(session, snapshot, interval, frame)
	status := state.encoderState(remoteClipEncodingHEVC)
	if status == nil || status.queued != 1 {
		t.Fatalf("expected queued frame count 1, got %+v", status)
	}
	if len(fake.queueCalls) != 1 || !fake.queueCalls[0] {
		t.Fatalf("expected force key queue call, got %v", fake.queueCalls)
	}

	var framesPayload []RemoteDesktopClipFrame
	duration := time.Duration(0)
	selected := controller.tryClipEncoders(state, session, snapshot, interval, &framesPayload, &duration, remoteClipEncodingHEVC)
	if selected != RemoteEncoderHEVC {
		t.Fatalf("unexpected encoder selected: %s", selected)
	}
	if len(framesPayload) != 1 {
		t.Fatalf("expected single encoded frame, got %d", len(framesPayload))
	}
	if len(fake.flushCalls) != 1 || !fake.flushCalls[0] {
		t.Fatalf("expected flush with keyframe, got %v", fake.flushCalls)
	}
	if session.EncoderHardware != "fake" {
		t.Fatalf("expected encoder hardware to be fake, got %q", session.EncoderHardware)
	}
	status = state.encoderState(remoteClipEncodingHEVC)
	if status.queued != 0 {
		t.Fatalf("expected queued count reset to zero, got %d", status.queued)
	}
}

func TestClipEncoderProfilerHook(t *testing.T) {
	defer SetClipEncoderProfiler(nil)
	var (
		mu     sync.Mutex
		called int
	)
	SetClipEncoderProfiler(clipEncoderProfilerFunc(func(event ClipEncoderEvent) {
		mu.Lock()
		called++
		mu.Unlock()
	}))
	recordClipEncoderEvent(ClipEncoderEvent{Encoder: "h264", Event: "queue"})
	mu.Lock()
	defer mu.Unlock()
	if called != 1 {
		t.Fatalf("expected profiler to capture event, got %d", called)
	}
}

func TestEnsureClipEncoderUsesNativeWhenAvailable(t *testing.T) {
	state := newStreamLoopState(RemoteStreamModeVideo)
	fake := &fakeClipEncoder{}

	originalNative := nativeHEVCFactory
	originalFFmpeg := ffmpegHEVCFactory
	nativeHEVCFactory = func() (clipVideoEncoder, error) {
		return fake, nil
	}
	ffmpegHEVCFactory = func(ffmpegEnvProvider) (clipVideoEncoder, error) {
		t.Fatalf("unexpected ffmpeg fallback")
		return nil, errors.New("unexpected")
	}
	defer func() {
		nativeHEVCFactory = originalNative
		ffmpegHEVCFactory = originalFFmpeg
	}()

	encoder := state.ensureClipEncoder(remoteClipEncodingHEVC)
	if encoder == nil {
		t.Fatalf("expected native encoder instance")
	}
	if encoder != fake {
		t.Fatalf("expected native encoder to be returned")
	}
	if state.ffmpegEnv != nil {
		t.Fatalf("ffmpeg environment should not be initialized when native encoder succeeds")
	}
}

func TestNewHEVCVideoEncoderFallsBackWhenNativeUnavailable(t *testing.T) {
	originalNative := nativeHEVCFactory
	originalFFmpeg := ffmpegHEVCFactory
	defer func() {
		nativeHEVCFactory = originalNative
		ffmpegHEVCFactory = originalFFmpeg
	}()

	nativeHEVCFactory = func() (clipVideoEncoder, error) {
		return nil, ErrNativeEncoderUnavailable
	}

	var providerCalled bool
	fake := &fakeClipEncoder{}
	ffmpegHEVCFactory = func(provider ffmpegEnvProvider) (clipVideoEncoder, error) {
		providerCalled = true
		if provider == nil {
			t.Fatalf("expected provider to be forwarded to ffmpeg factory")
		}
		env, err := provider()
		if err != nil {
			return nil, err
		}
		if env == nil {
			t.Fatalf("expected ffmpeg environment from provider")
		}
		return fake, nil
	}

	env := &ffmpegEnvironment{path: "fake-ffmpeg", caps: &ffmpegEncoderCapabilities{encoders: map[string]struct{}{}}}
	encoder, err := newHEVCVideoEncoder(func() (*ffmpegEnvironment, error) {
		return env, nil
	})
	if err != nil {
		t.Fatalf("expected ffmpeg fallback to succeed, got %v", err)
	}
	if encoder != fake {
		t.Fatalf("expected ffmpeg encoder to be returned")
	}
	if !providerCalled {
		t.Fatalf("expected ffmpeg factory to be invoked")
	}
}

func TestNewHEVCVideoEncoderNativeTelemetry(t *testing.T) {
	defer SetClipEncoderProfiler(nil)

	originalNative := nativeHEVCFactory
	originalFFmpeg := ffmpegHEVCFactory
	defer func() {
		nativeHEVCFactory = originalNative
		ffmpegHEVCFactory = originalFFmpeg
	}()

	nativeErr := errors.New("native backend failed")
	nativeHEVCFactory = func() (clipVideoEncoder, error) {
		return nil, nativeErr
	}

	fake := &fakeClipEncoder{}
	ffmpegHEVCFactory = func(provider ffmpegEnvProvider) (clipVideoEncoder, error) {
		if provider == nil {
			t.Fatalf("expected provider for ffmpeg fallback")
		}
		if _, err := provider(); err != nil {
			return nil, err
		}
		return fake, nil
	}

	var (
		mu     sync.Mutex
		events []ClipEncoderEvent
	)
	SetClipEncoderProfiler(clipEncoderProfilerFunc(func(event ClipEncoderEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}))

	env := &ffmpegEnvironment{path: "fake-ffmpeg", caps: &ffmpegEncoderCapabilities{encoders: map[string]struct{}{}}}
	encoder, err := newHEVCVideoEncoder(func() (*ffmpegEnvironment, error) {
		return env, nil
	})
	if err != nil {
		t.Fatalf("expected encoder initialization to succeed, got %v", err)
	}
	if encoder != fake {
		t.Fatalf("expected ffmpeg encoder to be returned after telemetry test setup")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(events) < 2 {
		t.Fatalf("expected at least two telemetry events, got %d", len(events))
	}

	var nativeEvent, ffmpegEvent *ClipEncoderEvent
	for i := range events {
		event := events[i]
		if event.Candidate == "native" && nativeEvent == nil {
			nativeEvent = &event
		}
		if event.Candidate == "ffmpeg" {
			ffmpegEvent = &event
		}
	}
	if nativeEvent == nil {
		t.Fatalf("expected native telemetry event to be recorded")
	}
	if !errors.Is(nativeEvent.Err, nativeErr) {
		t.Fatalf("expected native telemetry error %v, got %v", nativeErr, nativeEvent.Err)
	}
	if ffmpegEvent == nil {
		t.Fatalf("expected ffmpeg telemetry event to be recorded")
	}
	if ffmpegEvent.Err != nil {
		t.Fatalf("expected successful ffmpeg telemetry event, got error %v", ffmpegEvent.Err)
	}
}

func BenchmarkFFmpegClipEncoderQueueFlush(b *testing.B) {
	env, err := newFFmpegEnvironment()
	if err != nil {
		b.Skipf("ffmpeg not available: %v", err)
	}
	encoder, err := newAVCVideoEncoder(func() (*ffmpegEnvironment, error) {
		return env, nil
	})
	if err != nil {
		b.Skipf("unable to initialize encoder: %v", err)
	}
	defer encoder.Close()

	frame := clipFrameBuffer{
		OffsetMs: 0,
		Width:    128,
		Height:   72,
		Buffer:   make([]byte, 128*72*4),
	}
	baseOpts := clipEncodeOptions{
		Width:         frame.Width,
		Height:        frame.Height,
		Quality:       80,
		TargetBitrate: 1500,
		FrameInterval: time.Second / 30,
	}
	warm := baseOpts
	warm.ForceKey = true
	if err := encoder.QueueFrame(frame, warm, true); err != nil {
		b.Fatalf("warmup queue failed: %v", err)
	}
	if _, err := encoder.Flush(true); err != nil {
		b.Fatalf("warmup flush failed: %v", err)
	}

	b.ResetTimer()
	var totalBytes int64
	for i := 0; i < b.N; i++ {
		forceKey := i == 0
		opts := baseOpts
		opts.ForceKey = forceKey
		if err := encoder.QueueFrame(frame, opts, forceKey); err != nil {
			b.Fatalf("queue failed: %v", err)
		}
		result, err := encoder.Flush(forceKey)
		if err != nil {
			b.Fatalf("flush failed: %v", err)
		}
		if len(result.Frames) > 0 {
			totalBytes += int64(len(result.Frames[0].Data))
		}
	}
	if b.N > 0 {
		b.SetBytes(totalBytes / int64(b.N))
	}
}
