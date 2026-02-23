package account

import "github.com/source-maker/broth/form"

// LoginForm is the form struct for user login.
type LoginForm struct {
	Email    string `form:"email"`
	Password string `form:"password"`
}

// Validate checks the login form for errors.
func (f LoginForm) Validate() form.Errors {
	errs := make(form.Errors)
	form.RequiredString(errs, "email", f.Email, "Email")
	form.RequiredString(errs, "password", f.Password, "Password")
	return errs
}

// RegisterForm is the form struct for user registration.
type RegisterForm struct {
	Email           string `form:"email"`
	Name            string `form:"name"`
	Password        string `form:"password"`
	PasswordConfirm string `form:"password_confirm"`
}

// Validate checks the registration form for errors.
func (f RegisterForm) Validate() form.Errors {
	errs := make(form.Errors)
	form.RequiredString(errs, "email", f.Email, "Email")
	form.RequiredString(errs, "name", f.Name, "Name")
	form.RequiredString(errs, "password", f.Password, "Password")
	if len(f.Password) > 0 && len(f.Password) < 8 {
		errs.Add("password", "Password must be at least 8 characters")
	}
	if f.Password != f.PasswordConfirm {
		errs.Add("password_confirm", "Passwords do not match")
	}
	return errs
}
