package utils

import (
	"github.com/gofiber/fiber/v2"
	"github.com/zatrano/zbe/internal/domain"
)

// RespondOK sends a 200 response with a data payload.
func RespondOK(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

// RespondCreated sends a 201 response with a data payload.
func RespondCreated(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    data,
	})
}

// RespondNoContent sends a 204 response.
func RespondNoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// RespondError sends a structured error response.
func RespondError(c *fiber.Ctx, statusCode int, message string, details ...interface{}) error {
	body := fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    statusCode,
			"message": message,
		},
	}
	if len(details) > 0 && details[0] != nil {
		body["error"].(fiber.Map)["details"] = details[0]
	}
	return c.Status(statusCode).JSON(body)
}

// RespondValidationError sends a 422 with validation errors.
func RespondValidationError(c *fiber.Ctx, errs []domain.ValidationError) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    fiber.StatusUnprocessableEntity,
			"message": "validation failed",
			"details": errs,
		},
	})
}

// RespondNotFound sends a 404 response.
func RespondNotFound(c *fiber.Ctx, resource string) error {
	return RespondError(c, fiber.StatusNotFound, resource+" not found")
}

// RespondUnauthorized sends a 401 response.
func RespondUnauthorized(c *fiber.Ctx, msg ...string) error {
	message := "unauthorized"
	if len(msg) > 0 {
		message = msg[0]
	}
	return RespondError(c, fiber.StatusUnauthorized, message)
}

// RespondForbidden sends a 403 response.
func RespondForbidden(c *fiber.Ctx, msg ...string) error {
	message := "forbidden: insufficient permissions"
	if len(msg) > 0 {
		message = msg[0]
	}
	return RespondError(c, fiber.StatusForbidden, message)
}

// RespondBadRequest sends a 400 response.
func RespondBadRequest(c *fiber.Ctx, msg string) error {
	return RespondError(c, fiber.StatusBadRequest, msg)
}

// RespondInternalError sends a 500 response.
func RespondInternalError(c *fiber.Ctx) error {
	return RespondError(c, fiber.StatusInternalServerError, "an internal error occurred")
}

// RespondConflict sends a 409 response.
func RespondConflict(c *fiber.Ctx, msg string) error {
	return RespondError(c, fiber.StatusConflict, msg)
}
