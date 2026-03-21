package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// HealthHandler provides system health endpoints.
type HealthHandler struct {
	db        *gorm.DB
	startTime time.Time
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db, startTime: time.Now()}
}

// Health handles GET /health
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "ok",
		"service": "zbe",
		"version": "1.0.0",
		"uptime":  time.Since(h.startTime).String(),
	})
}

// Ready handles GET /ready — checks that the database is reachable.
func (h *HealthHandler) Ready(c *fiber.Ctx) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"error":  "database connection unavailable",
		})
	}

	if err := sqlDB.Ping(); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"error":  "database ping failed",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":   "ready",
		"database": "connected",
	})
}
