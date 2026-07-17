package handler

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const imageProxyMaxBytes = 32 << 20

type imageProxyRequest struct {
	URL string `json:"url" binding:"required"`
}

// ProxyImage fetches an upstream image server-side so browser actions never expose the upstream URL.
func ProxyImage(c *gin.Context) {
	var req imageProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid image url"})
		return
	}

	parsed, err := validatePublicImageURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid image url"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid image url"})
		return
	}
	httpReq.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg,image/gif,*/*;q=0.8")
	httpReq.Header.Set("User-Agent", "sub2api-image-proxy/1.0")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "failed to fetch image"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "failed to fetch image"})
		return
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "upstream response is not an image"})
		return
	}

	limited := http.MaxBytesReader(c.Writer, resp.Body, imageProxyMaxBytes)
	data, err := io.ReadAll(limited)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": "image is too large or unreadable"})
		return
	}

	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, contentType, data)
}

func validatePublicImageURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > 4096 {
		return nil, errors.New("invalid url")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("invalid url")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, errors.New("unsupported scheme")
	}

	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return nil, errors.New("private ip denied")
		}
		return parsed, nil
	}

	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") || strings.HasSuffix(lowerHost, ".local") {
		return nil, errors.New("private host denied")
	}

	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("host lookup failed")
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return nil, errors.New("private host denied")
		}
	}
	return parsed, nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return true
}
