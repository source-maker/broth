package account

import (
	"encoding/json"
	"net/http"

	"github.com/source-maker/broth/auth"
	"github.com/source-maker/broth/form"
	"github.com/source-maker/broth/middleware"
	"github.com/source-maker/broth/render"
	"github.com/source-maker/broth/session"
)

// Handler is the HTTP handler for account operations.
type Handler struct {
	svc      *Service
	renderer *render.Renderer
}

// NewHandler creates an account Handler.
func NewHandler(svc *Service, renderer *render.Renderer) *Handler {
	return &Handler{svc: svc, renderer: renderer}
}

// ShowLogin renders the login page.
// GET /account/login
func (h *Handler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	h.renderer.HTML(w, r, "account/login.html", map[string]any{
		"CSRFToken": middleware.GetCSRFToken(r.Context()),
	})
}

// Login processes the login form.
// POST /account/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var f LoginForm
	if err := form.Bind(r, &f); err != nil {
		h.renderer.Error(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	if errs := form.Validate(&f); errs != nil {
		h.renderer.HTML(w, r, "account/login.html", map[string]any{
			"Errors":    errs,
			"Form":      f,
			"CSRFToken": middleware.GetCSRFToken(r.Context()),
		})
		return
	}

	user, err := h.svc.Authenticate(r.Context(), LoginInput{
		Email:    f.Email,
		Password: f.Password,
	})
	if err != nil {
		h.renderer.HTML(w, r, "account/login.html", map[string]any{
			"Error":     "Invalid email or password",
			"Form":      f,
			"CSRFToken": middleware.GetCSRFToken(r.Context()),
		})
		return
	}

	// Set session
	sess := session.FromContext(r.Context())
	if sess != nil {
		sess.Regenerate()
		sess.Set("user_id", user.ID)
	}

	next := r.URL.Query().Get("next")
	if next == "" {
		next = "/account/profile"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// ShowRegister renders the registration page.
// GET /account/register
func (h *Handler) ShowRegister(w http.ResponseWriter, r *http.Request) {
	h.renderer.HTML(w, r, "account/register.html", map[string]any{
		"CSRFToken": middleware.GetCSRFToken(r.Context()),
	})
}

// Register processes the registration form.
// POST /account/register
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var f RegisterForm
	if err := form.Bind(r, &f); err != nil {
		h.renderer.Error(w, r, http.StatusBadRequest, "Invalid form data")
		return
	}

	if errs := form.Validate(&f); errs != nil {
		h.renderer.HTML(w, r, "account/register.html", map[string]any{
			"Errors":    errs,
			"Form":      f,
			"CSRFToken": middleware.GetCSRFToken(r.Context()),
		})
		return
	}

	user, err := h.svc.Register(r.Context(), RegisterInput{
		Email:    f.Email,
		Name:     f.Name,
		Password: f.Password,
	})
	if err != nil {
		h.renderer.HTML(w, r, "account/register.html", map[string]any{
			"Error":     "Registration failed: " + err.Error(),
			"Form":      f,
			"CSRFToken": middleware.GetCSRFToken(r.Context()),
		})
		return
	}

	// Auto-login after registration
	sess := session.FromContext(r.Context())
	if sess != nil {
		sess.Regenerate()
		sess.Set("user_id", user.ID)
	}

	http.Redirect(w, r, "/account/profile", http.StatusSeeOther)
}

// Logout destroys the session and redirects to login.
// POST /account/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sess := session.FromContext(r.Context())
	if sess != nil {
		sess.Destroy()
	}
	http.Redirect(w, r, "/account/login", http.StatusSeeOther)
}

// Profile renders the user profile page.
// GET /account/profile
func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	h.renderer.HTML(w, r, "account/profile.html", map[string]any{
		"User":      user,
		"CSRFToken": middleware.GetCSRFToken(r.Context()),
	})
}

// APICurrentUser returns the current authenticated user as JSON.
// GET /account/api/me
func (h *Handler) APICurrentUser(w http.ResponseWriter, r *http.Request) {
	user := auth.MustGetUser(r.Context())
	h.renderer.JSON(w, http.StatusOK, apiUserResponse{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
		Roles: user.Roles,
	})
}

// APILogin authenticates via JSON and returns a JWT token pair.
// POST /account/api/login
func (h *Handler) APILogin(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.renderer.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	if err := input.Validate(); err != nil {
		h.renderer.JSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": err.Error(),
		})
		return
	}

	user, err := h.svc.Authenticate(r.Context(), input)
	if err != nil {
		h.renderer.JSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid credentials",
		})
		return
	}

	if h.svc.tokenService == nil {
		h.renderer.JSON(w, http.StatusInternalServerError, map[string]string{
			"error": "token service not configured",
		})
		return
	}

	authUser := &auth.User{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
		Roles: user.Roles,
	}
	token, err := h.svc.tokenService.GenerateAccessToken(authUser)
	if err != nil {
		h.renderer.JSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to generate token",
		})
		return
	}

	h.renderer.JSON(w, http.StatusOK, map[string]string{
		"access_token": token,
		"token_type":   "Bearer",
	})
}

// apiUserResponse is the JSON response for the current user API endpoint.
type apiUserResponse struct {
	ID    int64    `json:"id"`
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}
