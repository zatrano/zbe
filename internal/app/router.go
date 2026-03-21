package app

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"gorm.io/gorm"

	appconfig "github.com/zatrano/zbe/config"
	"github.com/zatrano/zbe/internal/handler"
	"github.com/zatrano/zbe/internal/middleware"
	"github.com/zatrano/zbe/internal/repository"
	"github.com/zatrano/zbe/internal/service"
	"github.com/zatrano/zbe/pkg/mail"
)

// SetupRouter wires together all dependencies and returns a configured Fiber app.
func SetupRouter(cfg *appconfig.Config, db *gorm.DB, mailSvc *mail.Service) *fiber.App {
	// ── Fiber app ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:               cfg.App.Name + " API",
		ReadTimeout:           cfg.Server.ReadTimeout,
		WriteTimeout:          cfg.Server.WriteTimeout,
		IdleTimeout:           cfg.Server.IdleTimeout,
		BodyLimit:             10 * 1024 * 1024, // 10 MB
		EnableTrustedProxyCheck: true,
		DisableStartupMessage: false,
		ErrorHandler:          globalErrorHandler,
	})

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo  := repository.NewUserRepository(db)
	roleRepo  := repository.NewRoleRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	oauthRepo := repository.NewOAuthRepository(db)

	// ── Services ──────────────────────────────────────────────────────────────
	authSvc := service.NewAuthService(userRepo, roleRepo, tokenRepo, oauthRepo, mailSvc, cfg)
	userSvc := service.NewUserService(userRepo, roleRepo, cfg.Security.BcryptCost)
	roleSvc := service.NewRoleService(roleRepo)

	// ── Handlers ──────────────────────────────────────────────────────────────
	authHandler    := handler.NewAuthHandler(authSvc, cfg)
	userHandler    := handler.NewUserHandler(userSvc)
	roleHandler    := handler.NewRoleHandler(roleSvc)
	profileHandler := handler.NewProfileHandler(userSvc)
	healthHandler  := handler.NewHealthHandler(db)

	// ── Global middleware ──────────────────────────────────────────────────────
	app.Use(middleware.Recovery())
	app.Use(middleware.RequestID())
	app.Use(middleware.RequestLogger())
	app.Use(middleware.SecurityHeaders())

	// CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(cfg.Security.AllowedOrigins, ","),
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Request-ID",
		ExposeHeaders:    "X-Request-ID",
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// Compression
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	// Helmet (additional security headers)
	app.Use(helmet.New())

	// ── Health endpoints ───────────────────────────────────────────────────────
	app.Get("/health", healthHandler.Health)
	app.Get("/ready",  healthHandler.Ready)

	// ── API v1 ─────────────────────────────────────────────────────────────────
	api := app.Group("/api/v1")

	// ── Auth routes (public, rate-limited) ─────────────────────────────────────
	auth := api.Group("/auth")
	auth.Post("/register",        middleware.AuthRateLimit(),   authHandler.Register)
	auth.Post("/login",           middleware.AuthRateLimit(),   authHandler.Login)
	auth.Post("/logout",          middleware.RequireAuth(cfg, tokenRepo), authHandler.Logout)
	auth.Post("/refresh-token",   authHandler.RefreshToken)
	auth.Get("/verify-email",     authHandler.VerifyEmail)
	auth.Get("/google/login",     authHandler.GoogleLogin)
	auth.Get("/google/callback",  authHandler.GoogleCallback)

	// ── Password reset routes (public, strict rate-limited) ────────────────────
	pw := api.Group("/password-reset")
	pw.Post("/request", middleware.StrictRateLimit(), authHandler.RequestPasswordReset)
	pw.Post("/confirm", middleware.StrictRateLimit(), authHandler.ConfirmPasswordReset)

	// ── Profile routes (authenticated user) ───────────────────────────────────
	profile := api.Group("/profile",
		middleware.RequireAuth(cfg, tokenRepo),
	)
	profile.Get("/",              profileHandler.GetProfile)
	profile.Patch("/",            profileHandler.UpdateProfile)
	profile.Post("/change-password", middleware.StrictRateLimit(), profileHandler.ChangePassword)

	// ── User management routes (admin only) ────────────────────────────────────
	users := api.Group("/users",
		middleware.RequireAuth(cfg, tokenRepo),
		middleware.RequireAdmin(),
	)
	users.Get("/",         userHandler.List)
	users.Post("/",        userHandler.Create)
	users.Get("/:id",     userHandler.GetByID)
	users.Put("/:id",     userHandler.Update)
	users.Delete("/:id",  userHandler.Delete)
	users.Put("/:id/roles", userHandler.AssignRoles)

	// ── Role management routes (admin only) ────────────────────────────────────
	roles := api.Group("/roles",
		middleware.RequireAuth(cfg, tokenRepo),
		middleware.RequireAdmin(),
	)
	roles.Get("/",        roleHandler.List)
	roles.Post("/",       roleHandler.Create)
	roles.Get("/:id",    roleHandler.GetByID)
	roles.Put("/:id",    roleHandler.Update)
	roles.Delete("/:id", roleHandler.Delete)

	// ── 404 fallback ───────────────────────────────────────────────────────────
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    404,
				"message": "the requested endpoint does not exist",
			},
		})
	})

	return app
}

// globalErrorHandler is Fiber's centralised error handler.
func globalErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg  := "an internal error occurred"

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		msg  = fiberErr.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    code,
			"message": msg,
		},
	})
}
