package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/middleware"
	"github.com/zatrano/zbe/internal/service"
	"github.com/zatrano/zbe/pkg/utils"
)

// ProfileHandler handles the authenticated user's own profile endpoints.
type ProfileHandler struct {
	userSvc service.UserService
}

// NewProfileHandler constructs a ProfileHandler.
func NewProfileHandler(userSvc service.UserService) *ProfileHandler {
	return &ProfileHandler{userSvc: userSvc}
}

// GetProfile handles GET /api/v1/profile
func (h *ProfileHandler) GetProfile(c *fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return utils.RespondUnauthorized(c)
	}

	resp, err := h.userSvc.GetByID(userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return utils.RespondUnauthorized(c)
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, resp)
}

// UpdateProfile handles PATCH /api/v1/profile
func (h *ProfileHandler) UpdateProfile(c *fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return utils.RespondUnauthorized(c)
	}

	var req domain.UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	utils.SanitizeStringPtr(req.Name)

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.userSvc.UpdateProfile(userID, &req)
	if err != nil {
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, resp)
}

// ChangePassword handles POST /api/v1/profile/change-password
func (h *ProfileHandler) ChangePassword(c *fiber.Ctx) error {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return utils.RespondUnauthorized(c)
	}

	var req domain.ChangePasswordDTO
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	if err := h.userSvc.ChangePassword(userID, &req); err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return utils.RespondError(c, fiber.StatusUnauthorized, "current password is incorrect")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, fiber.Map{"message": "password changed successfully"})
}
