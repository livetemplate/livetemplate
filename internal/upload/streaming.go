package upload

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/livetemplate/livetemplate/internal/uploadtypes"
)

// valuePartCap bounds how many bytes are read from a single non-file form part
// (lvt-action, lvt-submitter, the data JSON envelope, plain fields). Form values
// are small; this caps memory without affecting file streaming.
const valuePartCap = 1 << 20 // 1 MB

// StreamMultipart iterates a multipart/form-data body via r.MultipartReader()
// WITHOUT calling ParseMultipartForm, so file bytes are never staged to disk by
// the stdlib. Non-file parts are buffered into the returned values map (so the
// caller can rebuild the action message). File parts are routed by
// isStreaming(field): streaming fields go to onFile (consumed inline as an
// io.Reader), all other file parts go to onStaged (may be nil).
//
// onFile/onStaged receive the live *multipart.Part. A callback that returns an
// error aborts iteration and the error is returned verbatim, so a *ValidationError
// from validation surfaces to the caller for per-field error mapping.
func StreamMultipart(
	r *http.Request,
	isStreaming func(field string) bool,
	onFile func(part *multipart.Part) error,
	onStaged func(part *multipart.Part) error,
) (map[string][]string, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, fmt.Errorf("failed to read multipart body: %w", err)
	}

	values := make(map[string][]string)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return values, fmt.Errorf("failed to read multipart part: %w", err)
		}

		// Value part (no filename): buffer it for action-message reconstruction.
		if part.FileName() == "" {
			name := part.FormName()
			b, readErr := io.ReadAll(io.LimitReader(part, valuePartCap))
			if readErr != nil {
				return values, fmt.Errorf("failed to read form value %q: %w", name, readErr)
			}
			values[name] = append(values[name], string(b))
			continue
		}

		// File part.
		if isStreaming(part.FormName()) {
			if err := onFile(part); err != nil {
				return values, err
			}
		} else if onStaged != nil {
			if err := onStaged(part); err != nil {
				return values, err
			}
		}
		// A non-streaming file part with no staged sink is intentionally dropped
		// (e.g. Preview/Direct fields never send bytes here).
	}

	return values, nil
}

// LimitGuard wraps a reader and enforces a maximum byte count mid-stream. It
// passes through at most max bytes; once the source is found to exceed max it
// returns uploadtypes.ErrUploadTooLarge (with zero bytes) instead of io.EOF, so
// a streaming copy to remote storage aborts WITHOUT first writing the oversize
// bytes — no truncated/partial object is committed. A max of 0 means unlimited.
type LimitGuard struct {
	r   io.Reader
	n   int64
	max int64
}

func (l *LimitGuard) Read(p []byte) (int, error) {
	if l.max <= 0 {
		n, err := l.r.Read(p)
		l.n += int64(n)
		return n, err
	}

	remaining := l.max - l.n
	if remaining <= 0 {
		// We've already delivered max bytes. Peek one more to tell "exactly max"
		// (EOF → success) from "more than max" (ErrUploadTooLarge) without ever
		// handing the oversize byte to the consumer. Loop to tolerate a reader
		// that returns (0, nil) (allowed by io.Reader); returning (0, nil) here
		// would spin io.Copy.
		var probe [1]byte
		for {
			m, err := l.r.Read(probe[:])
			if m > 0 {
				return 0, uploadtypes.ErrUploadTooLarge
			}
			if errors.Is(err, io.EOF) {
				return 0, io.EOF
			}
			if err != nil {
				return 0, err
			}
			// (0, nil): no data yet, not EOF — retry the probe.
		}
	}

	// Never read more than the remaining budget so the consumer sees ≤ max bytes.
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := l.r.Read(p)
	l.n += int64(n)
	return n, err
}

// NewLimitGuard returns a reader over src that fails with
// uploadtypes.ErrUploadTooLarge once more than max bytes are read (max 0 =
// unlimited). It also reports the number of bytes read so far via Count.
func NewLimitGuard(src io.Reader, max int64) *LimitGuard {
	return &LimitGuard{r: src, max: max}
}

// Count returns the number of bytes read through the guard so far.
func (l *LimitGuard) Count() int64 { return l.n }

// ValidateFileHeader validates a streaming file part's Accept rule from its
// header alone (filename + Content-Type), before any bytes are read. MaxFileSize
// is enforced separately mid-stream by LimitGuard, since the part carries no
// size under MultipartReader.
func ValidateFileHeader(filename, contentType string, config uploadtypes.UploadConfig) error {
	return validateFileType(filename, contentType, config.Accept)
}
