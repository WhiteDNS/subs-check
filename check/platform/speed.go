package platform

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"log/slog"

	"github.com/beck-8/subs-check/config"
	"github.com/juju/ratelimit"
	"github.com/metacubex/mihomo/common/convert"
)

// networkLimitedReader limits reads using the network-layer byte counter.
type networkLimitedReader struct {
	reader       io.Reader
	bytesCounter *uint64
	startBytes   uint64
	limit        uint64
}

func (r *networkLimitedReader) Read(p []byte) (n int, err error) {
	if r.limit > 0 {
		currentBytes := atomic.LoadUint64(r.bytesCounter)
		networkRead := currentBytes - r.startBytes

		if networkRead >= r.limit {
			return 0, io.EOF
		}

		// Limit this read size. This is approximate because the network layer may read more.
		if remaining := r.limit - networkRead; remaining < uint64(len(p)) {
			p = p[:remaining]
		}
	}
	return r.reader.Read(p)
}

// CheckSpeed downloads speedTestURL through httpClient and returns the measured
// throughput. The URL is passed in explicitly (rather than read from
// config.GlobalConfig) so a run captured at pipeline start stays consistent
// even if the user edits SpeedTestUrl mid-check.
func CheckSpeed(httpClient *http.Client, bucket *ratelimit.Bucket, bytesCounter *uint64, speedTestURL string) (int, int64, error) {
	// Note: speed limiting is implemented at the network layer (statsConn), while
	// size limiting is implemented at the application layer using the network byte counter.
	// - speed limit: implemented through bucket in statsConn (network layer)
	// - size limit: implemented through networkLimitedReader using the network byte counter
	//   (application layer, but still limits network traffic)

	// Create a new speed-test client using the original client's transport layer.
	speedClient := &http.Client{
		// Use a longer timeout for speed testing.
		Timeout: time.Duration(config.GlobalConfig.DownloadTimeout) * time.Second,
		// Preserve the original transport configuration.
		Transport: httpClient.Transport,
	}

	req, err := http.NewRequest("GET", speedTestURL, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", convert.RandUserAgent())

	// Record transferred network bytes before the speed test.
	var startBytes uint64
	if bytesCounter != nil {
		startBytes = *bytesCounter
	}
	startTime := time.Now()

	resp, err := speedClient.Do(req)
	if err != nil {
		slog.Debug(fmt.Sprintf("Speed-test request failed: %v", err))
		return 0, 0, err
	}
	defer resp.Body.Close()

	// Calculate the network-layer size limit.
	var limitSize uint64
	if config.GlobalConfig.DownloadMB > 0 {
		limitSize = uint64(config.GlobalConfig.DownloadMB) * 1024 * 1024
	} else {
		limitSize = 0 // Unlimited.
	}

	// Wrap the response body with networkLimitedReader to limit size by the network byte counter.
	limitedReader := &networkLimitedReader{
		reader:       resp.Body,
		bytesCounter: bytesCounter,
		startBytes:   startBytes,
		limit:        limitSize,
	}

	// Read all data.
	totalBytes, err := io.Copy(io.Discard, limitedReader)
	// io.EOF is expected when the limit is reached; only other errors matter.
	if err != nil && err != io.EOF && totalBytes == 0 {
		slog.Debug(fmt.Sprintf("totalBytes: %d, error while reading data: %v", totalBytes, err))
		return 0, 0, err
	}

	// Calculate download time in milliseconds.
	duration := time.Since(startTime).Milliseconds()
	if duration == 0 {
		duration = 1 // Avoid division by zero.
	}

	// Calculate actual network bytes transferred (compressed data).
	var actualBytes int64
	if bytesCounter != nil {
		actualBytes = int64(*bytesCounter - startBytes)
	} else {
		// Without a byte counter, accurate data is unavailable.
		actualBytes = 0
	}

	// Calculate speed (KB/s) using actual transferred network bytes.
	speed := int(float64(actualBytes) / 1024 * 1000 / float64(duration))

	return speed, actualBytes, nil
}
