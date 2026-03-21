package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/zatrano/zbe/internal/domain"
	"github.com/zatrano/zbe/internal/service"
	"github.com/zatrano/zbe/pkg/utils"
)

// RoleHandler handles role management HTTP endpoints (admin-only).
type RoleHandler struct {
	roleSvc service.RoleService
}

// NewRoleHandler constructs a RoleHandler.
func NewRoleHandler(roleSvc service.RoleService) *RoleHandler {
	return &RoleHandler{roleSvc: roleSvc}
}

// List handles GET /api/v1/roles
func (h *RoleHandler) List(c *fiber.Ctx) error {
	q := &domain.PaginationQuery{
		Page:    c.QueryInt("page", 1),
		PerPage: c.QueryInt("per_page", 50),
		Search:  utils.SanitizeString(c.Query("search")),
		SortDir: "asc",
	}

	result, err := h.roleSvc.List(q)
	if err != nil {
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, result)
}

// GetByID handles GET /api/v1/roles/:id
func (h *RoleHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.RespondBadRequest(c, "invalid role id")
	}

	resp, err := h.roleSvc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			return utils.RespondNotFound(c, "role")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, resp)
}

// Create handles POST /api/v1/roles
func (h *RoleHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.roleSvc.Create(&req)
	if err != nil {
		if errors.Is(err, service.ErrRoleNameTaken) {
			return utils.RespondConflict(c, "a role with that name already exists")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondCreated(c, resp)
}

// Update handles PUT /api/v1/roles/:id
func (h *RoleHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.RespondBadRequest(c, "invalid role id")
	}

	var req domain.UpdateRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.RespondBadRequest(c, "invalid request body")
	}

	if errs := utils.ValidateStruct(req); errs != nil {
		return utils.RespondValidationError(c, errs)
	}

	resp, err := h.roleSvc.Update(id, &req)
	if err != nil {
		if errors.Is(err, service.ErrRoleNotFound) {
			return utils.RespondNotFound(c, "role")
		}
		return utils.RespondInternalError(c)
	}

	return utils.RespondOK(c, resp)
}

// Delete handles DELETE /api/v1/roles/:id
func (h *RoleHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.RespondBadRequest(c, "invalid role id")
	}

	if err := h.roleSvc.Delete(id); err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotFound):
			return utils.RespondNotFound(c, "role")
		case err.Error() == "cannot delete system role":
			return utils.RespondForbidden(c, "system roles cannot be deleted")
		default:
			return utils.RespondInternalError(c)
		}
	}

	return utils.RespondNoContent(c)
}
