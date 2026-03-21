package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"sync"
	"time"

	"gopkg.in/mail.v2"

	"github.com/zatrano/zbe/config"
	"github.com/zatrano/zbe/pkg/logger"
)

// Service is the mail sender. It queues messages and dispatches them asynchronously.
type Service struct {
	cfg    config.SMTPConfig
	appCfg config.AppConfig
	queue  chan *mail.Message
	wg     sync.WaitGroup
}

// New creates a new mail Service and starts the background sender goroutine.
func New(cfg config.SMTPConfig, appCfg config.AppConfig) *Service {
	s := &Service{
		cfg:    cfg,
		appCfg: appCfg,
		queue:  make(chan *mail.Message, 50),
	}
	s.wg.Add(1)
	go s.dispatch()
	return s
}

// dispatch reads from the queue and dials the SMTP server.
func (s *Service) dispatch() {
	defer s.wg.Done()

	d := mail.NewDialer(s.cfg.Host, s.cfg.Port, s.cfg.User, s.cfg.Password)
	d.SSL = s.cfg.Port == 465
	if !d.SSL && s.cfg.UseTLS {
		d.StartTLSPolicy = mail.MandatoryStartTLS
	}

	var (
		sc  mail.SendCloser
		err error
	)

	for {
		select {
		case m, ok := <-s.queue:
			if !ok {
				return // channel closed
			}
			if sc == nil {
				if sc, err = d.Dial(); err != nil {
					logger.Errorf("mail: dial error: %v", err)
					sc = nil
					continue
				}
			}
			if err = mail.Send(sc, m); err != nil {
				logger.Errorf("mail: send error: %v", err)
				sc = nil
			}
		case <-time.After(30 * time.Second):
			if sc != nil {
				if err = sc.Close(); err != nil {
					logger.Errorf("mail: close error: %v", err)
				}
				sc = nil
			}
		}
	}
}

// Close flushes the queue and shuts down the dispatcher.
func (s *Service) Close() {
	close(s.queue)
	s.wg.Wait()
}

// enqueue adds a message to the send queue.
func (s *Service) enqueue(m *mail.Message) {
	select {
	case s.queue <- m:
	default:
		logger.Warn("mail: queue full, dropping message")
	}
}

// buildMessage creates a mail.Message with the standard headers.
func (s *Service) buildMessage(to, subject, htmlBody string) *mail.Message {
	m := mail.NewMessage()
	m.SetAddressHeader("From", s.cfg.From, s.cfg.FromName)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)
	return m
}

// ── Public send methods ────────────────────────────────────────────────────────

// SendVerificationEmail sends the account email-verification email.
func (s *Service) SendVerificationEmail(toEmail, toName, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.appCfg.FrontendURL, token)
	data := map[string]interface{}{
		"Name":      toName,
		"VerifyURL": verifyURL,
		"AppName":   s.appCfg.Name,
		"AppURL":    s.appCfg.URL,
		"Year":      time.Now().Year(),
	}
	html, err := renderTemplate(verifyEmailTemplate, data)
	if err != nil {
		return fmt.Errorf("render verify template: %w", err)
	}
	s.enqueue(s.buildMessage(toEmail, "Verify your email address — "+s.appCfg.Name, html))
	return nil
}

// SendPasswordReset sends a password-reset email.
func (s *Service) SendPasswordReset(toEmail, toName, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appCfg.FrontendURL, token)
	data := map[string]interface{}{
		"Name":     toName,
		"ResetURL": resetURL,
		"AppName":  s.appCfg.Name,
		"AppURL":   s.appCfg.URL,
		"Year":     time.Now().Year(),
		"Expiry":   "1 hour",
	}
	html, err := renderTemplate(passwordResetTemplate, data)
	if err != nil {
		return fmt.Errorf("render reset template: %w", err)
	}
	s.enqueue(s.buildMessage(toEmail, "Reset your password — "+s.appCfg.Name, html))
	return nil
}

// SendWelcomeEmail sends a welcome email after successful registration.
func (s *Service) SendWelcomeEmail(toEmail, toName string) error {
	data := map[string]interface{}{
		"Name":       toName,
		"AppName":    s.appCfg.Name,
		"AppURL":     s.appCfg.URL,
		"DashURL":    s.appCfg.FrontendURL + "/dashboard",
		"Year":       time.Now().Year(),
	}
	html, err := renderTemplate(welcomeTemplate, data)
	if err != nil {
		return fmt.Errorf("render welcome template: %w", err)
	}
	s.enqueue(s.buildMessage(toEmail, "Welcome to "+s.appCfg.Name+"! 🎉", html))
	return nil
}

// SendNotification sends a generic notification email.
func (s *Service) SendNotification(toEmail, toName, subject, message string) error {
	data := map[string]interface{}{
		"Name":    toName,
		"Message": template.HTML(message),
		"Subject": subject,
		"AppName": s.appCfg.Name,
		"AppURL":  s.appCfg.URL,
		"Year":    time.Now().Year(),
	}
	html, err := renderTemplate(notificationTemplate, data)
	if err != nil {
		return fmt.Errorf("render notification template: %w", err)
	}
	s.enqueue(s.buildMessage(toEmail, subject, html))
	return nil
}

// ── Template rendering ─────────────────────────────────────────────────────────

func renderTemplate(tplStr string, data interface{}) (string, error) {
	tpl, err := template.New("email").Parse(tplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ── Email templates ────────────────────────────────────────────────────────────

const baseStyle = `
body{margin:0;padding:0;background:#f8f9fc;font-family:'Inter',sans-serif;}
.wrap{max-width:600px;margin:40px auto;background:#fff;border-radius:16px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,.08);}
.header{background:#6366f1;padding:32px;text-align:center;}
.header h1{color:#fff;margin:0;font-size:24px;font-weight:800;letter-spacing:-.03em;}
.logo{display:inline-flex;align-items:center;justify-content:center;width:44px;height:44px;border-radius:12px;background:rgba(255,255,255,.2);color:#fff;font-weight:900;font-size:22px;margin-bottom:12px;}
.body{padding:40px 48px;}
.body p{color:#374151;line-height:1.6;margin:0 0 16px;}
.btn{display:inline-block;padding:14px 32px;background:#6366f1;color:#fff;text-decoration:none;border-radius:10px;font-weight:600;font-size:15px;margin:16px 0;}
.btn:hover{background:#4f46e5;}
.divider{border:none;border-top:1px solid #e5e7eb;margin:24px 0;}
.footer{background:#f9fafb;padding:24px 48px;text-align:center;color:#9ca3af;font-size:13px;}
.footer a{color:#6366f1;text-decoration:none;}
`

const htmlWrapper = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style>` + baseStyle + `</style></head>
<body>
<div class="wrap">
  <div class="header">
    <div class="logo">Z</div>
    <h1>{{.AppName}}</h1>
  </div>
  <div class="body">
    {{block "content" .}}{{end}}
  </div>
  <div class="footer">
    <p>&copy; {{.Year}} <a href="{{.AppURL}}">{{.AppName}}</a>. All rights reserved.</p>
  </div>
</div>
</body></html>`

const verifyEmailTemplate = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><style>` + baseStyle + `</style></head>
<body><div class="wrap">
<div class="header"><div class="logo">Z</div><h1>{{.AppName}}</h1></div>
<div class="body">
<p>Hi <strong>{{.Name}}</strong>,</p>
<p>Thanks for signing up! Please verify your email address to get started.</p>
<p style="text-align:center"><a href="{{.VerifyURL}}" class="btn">Verify Email Address</a></p>
<hr class="divider"/>
<p style="font-size:13px;color:#6b7280;">If the button doesn't work, copy and paste this link:<br>
<a href="{{.VerifyURL}}" style="color:#6366f1;word-break:break-all;">{{.VerifyURL}}</a></p>
<p style="font-size:13px;color:#6b7280;">If you didn't create an account, you can safely ignore this email.</p>
</div>
<div class="footer"><p>&copy; {{.Year}} <a href="{{.AppURL}}">{{.AppName}}</a></p></div>
</div></body></html>`

const passwordResetTemplate = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><style>` + baseStyle + `</style></head>
<body><div class="wrap">
<div class="header"><div class="logo">Z</div><h1>{{.AppName}}</h1></div>
<div class="body">
<p>Hi <strong>{{.Name}}</strong>,</p>
<p>We received a request to reset your password. Click the button below to create a new one.</p>
<p style="text-align:center"><a href="{{.ResetURL}}" class="btn">Reset Password</a></p>
<hr class="divider"/>
<p style="font-size:13px;color:#6b7280;">This link expires in <strong>{{.Expiry}}</strong>.</p>
<p style="font-size:13px;color:#6b7280;">If you didn't request a password reset, please ignore this email and your password will remain unchanged.</p>
<p style="font-size:13px;color:#6b7280;">Link: <a href="{{.ResetURL}}" style="color:#6366f1;word-break:break-all;">{{.ResetURL}}</a></p>
</div>
<div class="footer"><p>&copy; {{.Year}} <a href="{{.AppURL}}">{{.AppName}}</a></p></div>
</div></body></html>`

const welcomeTemplate = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><style>` + baseStyle + `</style></head>
<body><div class="wrap">
<div class="header"><div class="logo">Z</div><h1>Welcome to {{.AppName}}!</h1></div>
<div class="body">
<p>Hi <strong>{{.Name}}</strong>,</p>
<p>Your account is all set up and ready to go. Start building powerful forms and collecting data effortlessly.</p>
<p style="text-align:center"><a href="{{.DashURL}}" class="btn">Go to Dashboard</a></p>
<hr class="divider"/>
<p>Need help? Reply to this email and we'll get back to you shortly.</p>
</div>
<div class="footer"><p>&copy; {{.Year}} <a href="{{.AppURL}}">{{.AppName}}</a></p></div>
</div></body></html>`

const notificationTemplate = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><style>` + baseStyle + `</style></head>
<body><div class="wrap">
<div class="header"><div class="logo">Z</div><h1>{{.AppName}}</h1></div>
<div class="body">
<p>Hi <strong>{{.Name}}</strong>,</p>
<p>{{.Message}}</p>
</div>
<div class="footer"><p>&copy; {{.Year}} <a href="{{.AppURL}}">{{.AppName}}</a></p></div>
</div></body></html>`
