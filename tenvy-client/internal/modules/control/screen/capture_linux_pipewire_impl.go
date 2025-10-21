//go:build linux && pipewire

package screen

/*
#cgo pkg-config: libpipewire-0.3 libspa-0.2
#include <errno.h>
#include <pipewire/pipewire.h>
#include <spa/buffer/alloc.h>
#include <spa/buffer/buffer.h>
#include <spa/buffer/types.h>
#include <spa/param/video/format-utils.h>
#include <spa/param/video/raw-utils.h>
#include <spa/utils/defs.h>
#include <spa/utils/result.h>
#include <spa/pod/builder.h>
#include <spa/param/param.h>
#include <spa/utils/names.h>
#include <spa/utils/string.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>

struct tenvy_pipewire {
        struct pw_thread_loop *loop;
        struct pw_context *context;
        struct pw_core *core;
        struct pw_stream *stream;
        struct spa_video_info_raw format;
        uintptr_t handle;
        uint32_t node_id;
        uint32_t width;
        uint32_t height;
};

extern void tenvyPipewireDeliverFrame(uintptr_t handle, void *data, size_t size, uint32_t stride, uint32_t width, uint32_t height);
extern void tenvyPipewireDeliverError(uintptr_t handle, const char *msg);

static void tenvy_on_process(void *data);
static void tenvy_on_state_changed(void *data, enum pw_stream_state old, enum pw_stream_state state, const char *error);

static const struct pw_stream_events tenvy_stream_events = {
        PW_VERSION_STREAM_EVENTS,
        .state_changed = tenvy_on_state_changed,
        .process = tenvy_on_process,
};

static struct tenvy_pipewire *tenvy_pipewire_new(uintptr_t handle) {
        pw_init(NULL, NULL);
        struct tenvy_pipewire *pw = calloc(1, sizeof(*pw));
        if (pw == NULL) {
                return NULL;
        }
        pw->handle = handle;

        pw->loop = pw_thread_loop_new("tenvy-pipewire", NULL);
        if (pw->loop == NULL) {
                tenvyPipewireDeliverError(handle, "failed to create PipeWire thread loop");
                goto fail;
        }
        pw->context = pw_context_new(pw_thread_loop_get_loop(pw->loop), NULL, 0);
        if (pw->context == NULL) {
                tenvyPipewireDeliverError(handle, "failed to create PipeWire context");
                goto fail;
        }
        pw->core = pw_context_connect(pw->context, NULL, 0);
        if (pw->core == NULL) {
                tenvyPipewireDeliverError(handle, "failed to connect to PipeWire core");
                goto fail;
        }
        if (pw_thread_loop_start(pw->loop) < 0) {
                tenvyPipewireDeliverError(handle, "failed to start PipeWire thread loop");
                goto fail;
        }
        return pw;
fail:
        if (pw != NULL) {
                if (pw->core != NULL) {
                        pw_core_disconnect(pw->core);
                }
                if (pw->context != NULL) {
                        pw_context_destroy(pw->context);
                }
                if (pw->loop != NULL) {
                        pw_thread_loop_destroy(pw->loop);
                }
                free(pw);
        }
        pw_deinit();
        return NULL;
}

static void tenvy_pipewire_destroy(struct tenvy_pipewire *pw) {
        if (pw == NULL) {
                return;
        }
        if (pw->loop != NULL) {
                pw_thread_loop_lock(pw->loop);
                if (pw->stream != NULL) {
                        pw_stream_destroy(pw->stream);
                        pw->stream = NULL;
                }
                pw_thread_loop_unlock(pw->loop);
                pw_thread_loop_stop(pw->loop);
        }
        if (pw->core != NULL) {
                pw_core_disconnect(pw->core);
                pw->core = NULL;
        }
        if (pw->context != NULL) {
                pw_context_destroy(pw->context);
                pw->context = NULL;
        }
        if (pw->loop != NULL) {
                pw_thread_loop_destroy(pw->loop);
                pw->loop = NULL;
        }
        free(pw);
        pw_deinit();
}

static int tenvy_pipewire_configure(struct tenvy_pipewire *pw, uint32_t node_id, uint32_t width, uint32_t height) {
        if (pw == NULL || pw->loop == NULL) {
                return -EINVAL;
        }
        if (width == 0 || height == 0) {
                return -EINVAL;
        }

        pw_thread_loop_lock(pw->loop);

        if (pw->stream != NULL) {
                if (pw->node_id == node_id && pw->width == width && pw->height == height) {
                        pw_thread_loop_unlock(pw->loop);
                        return 0;
                }
                pw_stream_destroy(pw->stream);
                pw->stream = NULL;
        }

        struct pw_properties *props = pw_properties_new(
                PW_KEY_MEDIA_TYPE, "Video",
                PW_KEY_MEDIA_CATEGORY, "Capture",
                PW_KEY_MEDIA_ROLE, "Screen",
                PW_KEY_APP_NAME, "tenvy-agent",
                NULL);
        if (props == NULL) {
                pw_thread_loop_unlock(pw->loop);
                return -errno;
        }
        if (node_id != 0) {
                char target[32];
                snprintf(target, sizeof(target), "%u", node_id);
                pw_properties_set(props, PW_KEY_TARGET_OBJECT, target);
        }

        pw->stream = pw_stream_new_simple(
                pw_thread_loop_get_loop(pw->loop),
                "tenvy-screen",
                props,
                &tenvy_stream_events,
                pw);
        if (pw->stream == NULL) {
                pw_thread_loop_unlock(pw->loop);
                return -errno;
        }

        pw->node_id = node_id;
        pw->width = width;
        pw->height = height;
        spa_zero(pw->format);
        pw->format.format = SPA_VIDEO_FORMAT_BGRA;
        pw->format.size.width = width;
        pw->format.size.height = height;
        pw->format.framerate = SPA_FRACTION(30, 1);
        pw->format.max_framerate = SPA_FRACTION(30, 1);

        uint8_t buffer[512];
        struct spa_pod_builder builder = SPA_POD_BUILDER_INIT(buffer, sizeof(buffer));
        struct spa_rectangle size = SPA_RECTANGLE(width, height);
        struct spa_fraction fps = SPA_FRACTION(30, 1);

        const struct spa_pod *params[1];
        params[0] = spa_pod_builder_add_object(&builder,
                SPA_TYPE_OBJECT_Format, SPA_PARAM_EnumFormat,
                SPA_FORMAT_mediaType, SPA_POD_Id(SPA_MEDIA_TYPE_video),
                SPA_FORMAT_mediaSubtype, SPA_POD_Id(SPA_MEDIA_SUBTYPE_raw),
                SPA_FORMAT_VIDEO_format, SPA_POD_Id(SPA_VIDEO_FORMAT_BGRA),
                SPA_FORMAT_VIDEO_size, SPA_POD_Rectangle(&size),
                SPA_FORMAT_VIDEO_maxSize, SPA_POD_Rectangle(&size),
                SPA_FORMAT_VIDEO_framerate, SPA_POD_Fraction(&fps),
                SPA_FORMAT_VIDEO_maxFramerate, SPA_POD_Fraction(&fps));

        int res = pw_stream_connect(pw->stream,
                PW_DIRECTION_INPUT,
                node_id == 0 ? PW_ID_ANY : node_id,
                PW_STREAM_FLAG_AUTOCONNECT | PW_STREAM_FLAG_MAP_BUFFERS | PW_STREAM_FLAG_RT_PROCESS,
                params,
                1);

        pw_thread_loop_unlock(pw->loop);
        return res;
}

static void tenvy_on_state_changed(void *data, enum pw_stream_state old, enum pw_stream_state state, const char *error) {
        if (state == PW_STREAM_STATE_ERROR && error != NULL) {
                struct tenvy_pipewire *pw = data;
                if (pw != NULL) {
                        tenvyPipewireDeliverError(pw->handle, error);
                }
        }
}

static void tenvy_on_process(void *data) {
        struct tenvy_pipewire *pw = data;
        if (pw == NULL || pw->stream == NULL) {
                return;
        }
        struct pw_buffer *buffer = pw_stream_dequeue_buffer(pw->stream);
        if (buffer == NULL) {
                return;
        }
        struct spa_buffer *spa_buf = buffer->buffer;
        if (spa_buf == NULL || spa_buf->datas == NULL) {
                pw_stream_queue_buffer(pw->stream, buffer);
                return;
        }
        struct spa_data *d = &spa_buf->datas[0];
        if (d->chunk == NULL || d->chunk->size == 0) {
                pw_stream_queue_buffer(pw->stream, buffer);
                return;
        }

        size_t size = d->chunk->size;
        uint32_t stride = d->chunk->stride != 0 ? d->chunk->stride : pw->width * 4;
        void *map_base = NULL;
        void *ptr = NULL;

        if (d->type == SPA_DATA_MemPtr || d->type == SPA_DATA_MemId) {
                ptr = SPA_MEMBER(d->data, d->chunk->offset, void);
        } else if ((d->type == SPA_DATA_DmaBuf || d->type == SPA_DATA_MemFd) && d->fd >= 0) {
                size_t map_size = d->maxsize + d->mapoffset;
                if (map_size < size + d->chunk->offset) {
                        map_size = size + d->chunk->offset;
                }
                map_base = mmap(NULL, map_size, PROT_READ, MAP_PRIVATE, d->fd, d->mapoffset);
                if (map_base != MAP_FAILED) {
                        ptr = SPA_MEMBER(map_base, d->chunk->offset, void);
                }
        }

        if (ptr != NULL) {
                tenvyPipewireDeliverFrame(pw->handle, ptr, size, stride, pw->width, pw->height);
        }

        if (map_base != NULL && map_base != MAP_FAILED) {
                munmap(map_base, d->maxsize + d->mapoffset);
        }

        pw_stream_queue_buffer(pw->stream, buffer);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"os"
	"strconv"
	"sync"
	"unsafe"

	"runtime"
	"runtime/cgo"
)

func defaultPlatformCaptureCandidates() []backendCandidate {
	if err := ensurePipewireAvailable(); err != nil {
		return nil
	}
	return []backendCandidate{{name: "pipewire", factory: newPipewireCaptureBackend}}
}

type pipewireCaptureBackend struct {
	handle cgo.Handle
	native *C.struct_tenvy_pipewire
	nodeID uint32

	mu      sync.Mutex
	cond    *sync.Cond
	closed  bool
	width   int
	height  int
	pending *image.RGBA
	err     error
}

func newPipewireCaptureBackend() (captureBackend, error) {
	if err := ensurePipewireAvailable(); err != nil {
		return nil, err
	}
	nodeID, err := pipewireNodeIDFromEnv()
	if err != nil {
		return nil, err
	}

	backend := &pipewireCaptureBackend{nodeID: nodeID}
	backend.cond = sync.NewCond(&backend.mu)
	backend.handle = cgo.NewHandle(backend)
	native := C.tenvy_pipewire_new(C.uintptr_t(backend.handle))
	if native == nil {
		backend.handle.Delete()
		return nil, errors.New("failed to initialize PipeWire backend")
	}
	backend.native = native
	runtime.SetFinalizer(backend, (*pipewireCaptureBackend).Close)
	return backend, nil
}

func (b *pipewireCaptureBackend) Name() string { return "pipewire" }

func (b *pipewireCaptureBackend) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	native := b.native
	b.native = nil
	handle := b.handle
	b.handle = cgo.Handle{}
	b.cond.Broadcast()
	b.mu.Unlock()

	if handle != 0 {
		handle.Delete()
	}
	if native != nil {
		C.tenvy_pipewire_destroy(native)
	}
}

func (b *pipewireCaptureBackend) ensureConfiguredLocked(width, height int) error {
	if b.native == nil {
		return errors.New("pipewire backend not initialised")
	}
	if b.width == width && b.height == height {
		return nil
	}
	res := C.tenvy_pipewire_configure(b.native, C.uint32_t(b.nodeID), C.uint32_t(width), C.uint32_t(height))
	if res < 0 {
		return fmt.Errorf("pipewire configure: %w", pipewireError(res))
	}
	b.width = width
	b.height = height
	return nil
}

func (b *pipewireCaptureBackend) Capture(bounds image.Rectangle) (*image.RGBA, error) {
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid capture bounds")
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, errors.New("pipewire backend closed")
	}
	if err := b.ensureConfiguredLocked(width, height); err != nil {
		b.mu.Unlock()
		return nil, err
	}
	b.pending = nil
	b.err = nil
	b.mu.Unlock()

	b.mu.Lock()
	for !b.closed && b.pending == nil && b.err == nil {
		b.cond.Wait()
	}
	if b.closed {
		b.mu.Unlock()
		return nil, errors.New("pipewire backend closed")
	}
	if b.err != nil {
		err := b.err
		b.err = nil
		b.mu.Unlock()
		return nil, err
	}
	frame := b.pending
	b.pending = nil
	b.mu.Unlock()

	if frame == nil {
		return nil, errors.New("pipewire produced no frame")
	}
	frame.Rect = bounds
	return frame, nil
}

func (b *pipewireCaptureBackend) deliverFrame(buf []byte, stride int, width, height int) {
	rowBytes := width * 4
	if len(buf) < stride*height || rowBytes > stride {
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		srcRow := buf[y*stride : y*stride+rowBytes]
		dstRow := img.Pix[y*img.Stride : y*img.Stride+rowBytes]
		for x := 0; x < width; x++ {
			base := x * 4
			bVal := srcRow[base]
			gVal := srcRow[base+1]
			rVal := srcRow[base+2]
			aVal := srcRow[base+3]
			dstRow[base] = rVal
			dstRow[base+1] = gVal
			dstRow[base+2] = bVal
			dstRow[base+3] = aVal
		}
	}

	b.mu.Lock()
	if !b.closed {
		b.pending = img
		b.cond.Broadcast()
	}
	b.mu.Unlock()
}

func (b *pipewireCaptureBackend) setError(err error) {
	b.mu.Lock()
	if !b.closed {
		b.err = err
		b.cond.Broadcast()
	}
	b.mu.Unlock()
}

func pipewireNodeIDFromEnv() (uint32, error) {
	val := os.Getenv("TENVY_PIPEWIRE_NODE_ID")
	if val == "" {
		return 0, nil
	}
	id, err := strconv.ParseUint(val, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid TENVY_PIPEWIRE_NODE_ID: %w", err)
	}
	return uint32(id), nil
}

func pipewireError(code C.int) error {
	errCode := C.int(-code)
	msg := C.GoString(C.spa_strerror(errCode))
	if msg == "" {
		msg = fmt.Sprintf("error %d", int(errCode))
	}
	return errors.New(msg)
}

//export tenvyPipewireDeliverFrame
func tenvyPipewireDeliverFrame(handle C.uintptr_t, data unsafe.Pointer, size C.size_t, stride C.uint32_t, width C.uint32_t, height C.uint32_t) {
	if handle == 0 || data == nil || size == 0 {
		return
	}
	h := cgo.Handle(handle)
	backend, ok := h.Value().(*pipewireCaptureBackend)
	if !ok {
		return
	}
	raw := unsafe.Slice((*byte)(data), int(size))
	frame := make([]byte, len(raw))
	copy(frame, raw)
	backend.deliverFrame(frame, int(stride), int(width), int(height))
}

//export tenvyPipewireDeliverError
func tenvyPipewireDeliverError(handle C.uintptr_t, msg *C.char) {
	if handle == 0 {
		return
	}
	h := cgo.Handle(handle)
	backend, ok := h.Value().(*pipewireCaptureBackend)
	if !ok {
		return
	}
	var err error
	if msg != nil {
		err = errors.New(C.GoString(msg))
	} else {
		err = errors.New("pipewire stream error")
	}
	backend.setError(err)
}
