package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/service"
	"github.com/zatrano/zbe/pkg/utils"
)

// UserHandler handles user management HTTP endpoints (admin-only).
type UserHandler struct {
	userSvc service.UserService
}

// NewUserHandler constructs a UserHandler.
func NewUserHandler(userSvc service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

// List handles GET /api/v1/users
func (h *UserHandler) List(c *fiber.Ctx) error {
	q := &domain.PaginationQuery{
		Page:    c.QueryInt("page", 1),
		PerPage: c.QueryInt("per_page", 20),
		Search:  utils.SanitizeString(c.Query("search")),
		SortBy:  c.Query("sort_by", "created_at"),
		SortDir: c.Query("sort_dir", "desc"),
	}

	if errs := utils.ValidateStruct(q); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	result, err := h.userSvc.List(q)
	if err != nil {
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, result)
}

// GetByID handles GET /api/v1/users/:id
func (h *UserHandler) GetByID(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return utils.RespondBadRequest(c, "invalid user id")
	}

	resp, err := h.userSvc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return utils.RespondNotFound(c, "user")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, resp)
}

// Create handles POST /api/v1/users
func (h *UserHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	req.Name  = utils.SanitizeString(req.Name)
	req.Email = utils.SanitizeString(req.Email)

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.userSvc.Create(&req)
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			return utils.RespondConflict(c, "email address is already registered")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondCreated(c, resp)
}

// Update handles PUT /api/v1/users/:id
func (h *UserHandler) Update(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return utils.RespondBadRequest(c, "invalid user id")
	}

	var req domain.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	if req.Name != nil {
		sanitized := utils.SanitizeString(*req.Name)
		req.Name = &sanitized
	}

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.userSvc.Update(id, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			return utils.RespondNotFound(c, "user")
		case errors.Is(err, service.ErrEmailAlreadyExists):
			return utils.RespondConflict(c, "email address is already in use")
		default:
			return utils.RespondInternalError(c)
		}
	}

	return utils.RespondOK(c, resp)
}

// Delete handles DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return utils.RespondBadRequest(c, "invalid user id")
	}

	if err := h.userSvc.Delete(id); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return utils.RespondNotFound(c, "user")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondNoContent(c)
}

// AssignRoles handles PUT /api/v1/users/:id/roles
func (h *UserHandler) AssignRoles(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return utils.RespondBadRequest(c, "invalid user id")
	}

	var req domain.AssignRolesRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.userSvc.AssignRoles(id, &req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return utils.RespondNotFound(c, "user")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, resp)
}

// ── shared helpers ─────────────────────────────────────────────────────────────

func parseUUIDParam(c *fiber.Ctx, param string) (uuid.UUID, error) {
	return uuid.Parse(c.Params(param))
}
