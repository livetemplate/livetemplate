package upload

import "io"

// ProgressReader wraps an io.ReadCloser and tracks bytes read.
// It fires the OnProgress callback each time a new percentage threshold is crossed.
type ProgressReader struct {
	reader     io.ReadCloser
	bytesRead  int64
	total      int64
	OnProgress func(bytesRead, total int64)
	lastPct    int
}

// NewProgressReader wraps an io.ReadCloser with progress tracking.
// total is the expected total byte count (e.g., Content-Length).
// If total <= 0, progress percentage is not calculated and the callback
// is not fired.
func NewProgressReader(reader io.ReadCloser, total int64) *ProgressReader {
	return &ProgressReader{
		reader: reader,
		total:  total,
	}
}

func (p *ProgressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)
	p.bytesRead += int64(n)
	if p.total > 0 && p.OnProgress != nil {
		pct := int(p.bytesRead * 100 / p.total)
		if pct > p.lastPct {
			p.lastPct = pct
			p.OnProgress(p.bytesRead, p.total)
		}
	}
	return n, err
}

func (p *ProgressReader) Close() error {
	return p.reader.Close()
}
