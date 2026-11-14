package relay

import (
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lingyuins/octopus/internal/conf"
)

var errRelayRequestBodyTooLarge = errors.New("request body too large")
var errRelayRequestBodyTimeout = errors.New("request body read timeout")

func getMaxRelayJSONBodyBytes() int64 {
	if v := conf.AppConfig.Relay.MaxJSONBodyBytes; v > 0 {
		return v
	}
	return 64 << 20
}

func getMaxRelayMultipartBodyBytes() int64 {
	if v := conf.AppConfig.Relay.MaxMultipartBodyBytes; v > 0 {
		return v
	}
	return 64 << 20
}

func readLimitedRequestBody(c *gin.Context, limit int64) ([]byte, error) {
	limitRequestBody(c, limit)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, normalizeRelayRequestBodyError(err)
	}
	return body, nil
}

func limitRequestBody(c *gin.Context, limit int64) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
}

func normalizeRelayRequestBodyError(err error) error {
	if err == nil {
		return nil
	}

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return errRelayRequestBodyTooLarge
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return errors.Join(errRelayRequestBodyTimeout, err)
	}
	return err
}

func relayRequestBodyErrorStatus(err error) int {
	if errors.Is(err, errRelayRequestBodyTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	if errors.Is(err, errRelayRequestBodyTimeout) {
		return http.StatusRequestTimeout
	}
	return http.StatusBadRequest
}
