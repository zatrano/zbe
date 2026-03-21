package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/zatrano/zbe/pkg/logger"
)

// RequestLogger logs every incoming request and its outcome using zap.
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		fields := []zap.Field{
			zap.String("method",     c.Method()),
			zap.String("path",       c.Path()),
			zap.String("ip",         c.IP()),
			zap.Int("status",        status),
			zap.Duration("latency",  duration),
			zap.String("ua",         c.Get(fiber.HeaderUserAgent)),
			zap.String("request_id", c.GetRespHeader("X-Request-ID")),
		}

		if qs := c.Request().URI().QueryString(); len(qs) > 0 {
			fields = append(fields, zap.ByteString("query", qs))
		}

		switch {
		case status >= 500:
			logger.Error("request", fields...)
		case status >= 400:
			logger.Warn("request", fields...)
		default:
			logger.Info("request", fields...)
		}

		return err
	}
}

// Recovery catches panics, logs them, and returns a 500 response.
func Recovery() fiber.Handler {
	return func(c *fiber.Ctx) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					zap.Any("error",   r),
					zap.String("method", c.Method()),
					zap.String("path",   c.Path()),
					zap.String("ip",     c.IP()),
				)
				retErr = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    500,
						"message": "an unexpected error occurred",
					},
				})
			}
		}()
		return c.Next()
	}
}

// RequestID injects a unique X-Request-ID header into every response.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			}
		}
		c.Set("X-Request-ID", id)
		return c.Next()
	}
}

// SecurityHeaders sets common security-related HTTP response headers.
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options",    "nosniff")
		c.Set("X-Frame-Options",           "DENY")
		c.Set("X-XSS-Protection",          "1; mode=block")
		c.Set("Referrer-Policy",           "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy",        "geolocation=(), microphone=(), camera=()")
		if c.Protocol() == "https" {
			c.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		return c.Next()
	}
}
