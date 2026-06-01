package platform

import (
	"bytes"
	"encoding/json"
	"io"
	"sync"
)

// bodyBufPool reuses HTTP response body buffers to amortize io.ReadAll growth costs.
var bodyBufPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 64<<10))
	},
}

// getPooledBuf borrows a len=0 buffer from the pool.
//
// Contract:
//   - callers must defer putPooledBuf(buf) to return the buffer.
//   - after returning the buffer, slices from buf.Bytes() are invalid and must
//     not escape the function, including return values, struct fields, closures,
//     or substrings.
func getPooledBuf() *bytes.Buffer {
	buf := bodyBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putPooledBuf returns buf to the pool. Oversized buffers are dropped to avoid pool growth.
func putPooledBuf(buf *bytes.Buffer) {
	if buf == nil || buf.Cap() > 4<<20 {
		return
	}
	bodyBufPool.Put(buf)
}

// readJSONPooled reads r, decodes JSON into v, and returns the buffer before exiting.
func readJSONPooled(r io.Reader, v any) error {
	buf := getPooledBuf()
	defer putPooledBuf(buf)
	if _, err := buf.ReadFrom(r); err != nil {
		return err
	}
	return json.Unmarshal(buf.Bytes(), v)
}
