// The MIT License (MIT)

// Copyright (c) 2017 Fabian Wenzelmann

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package mailwebadmin

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/FabianWe/goauth"
	"github.com/gorilla/csrf"
	"github.com/sirupsen/logrus"
)

const (
	// getMethod is the constant for http GET.
	getMethod = "GET"
	// postMethod is the constant for http POST.
	postMethod = "POST"
	// deleteMethod is the constant for http DELETE.
	deleteMethod = "DELETE"
	// updateMethod is the constant for http UPDATE.
	updateMethod = "UPDATE"
)

// StaticHandler is a http handler that serves files in the static directory.
func StaticHandler() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
}

// AppHandleFunc is a type for functions that accept a context as well as
// the request and response writer.
// It can be used as a http handle func via MailAppHandler.
// An error returned by this function should be only internal server errors.
// All other errors should be handled inside the function and return a
// corresponding http response.
// If an internal server error occurs there should be no writes to the response.
// ServeHTTP will handle internal server errors.
type AppHandleFunc func(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error

// MailAppHandler is the handler class for AppHandleFuncs.
// It can be used as http handler via ServeHTTP.
type MailAppHandler struct {
	// MailAppContext is the context that stores things like global settings,
	// the database connection etc.
	*MailAppContext
	// f is the function that does something with the given context.
	f AppHandleFunc
}

// NewMailAppHandler returns a new MailAppHandler.
func NewMailAppHandler(context *MailAppContext, f AppHandleFunc) *MailAppHandler {
	return &MailAppHandler{MailAppContext: context, f: f}
}

// ServeHTTP implements the http handler interface.
// If will execute the handle function and check for an error. If an error
// is returned (this means an internal error occurred) it will reply with a
// 500 Internal Server Error.
func (handler *MailAppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := handler.f(handler.MailAppContext, w, r); err != nil {
		handler.MailAppContext.Logger.Error(err)
		http.Error(w, "Internal Server Error", 500)
	}
}

// LoginRequired takes a handler function and returns a new handler function
// that first checks if the login is correct.
// It will return a 302 redirect to the login page if the user is not logged in.
// It will also check if the session has a remember-me set, and if it is not set
// update the session MaxAge to 0.
// An error while updating the session will not be returned as an error, but
// will be logged.
func LoginRequired(f AppHandleFunc) AppHandleFunc {
	return func(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
		// first check if the user is logged in
		_, session, err := appcontext.SessionController.ValidateSession(r, appcontext.Store)
		if err != nil {
			switch err {
			case goauth.ErrNotAuthSession, goauth.ErrKeyNotFound, goauth.ErrInvalidKey:
				// consider lookup as failed, redirect to the login page!
				http.Redirect(w, r, "/login", 302)
				return nil
			default:
				// something really went wrong, report the error
				return err
			}
		}
		// check the remember-me field from the session
		rememberContainer, hasRemember := session.Values["remember-me"]
		if !hasRemember {
			appcontext.Logger.Info("Found session without remember-me set")
		} else {
			rememberMe, ok := rememberContainer.(bool)
			if !ok {
				appcontext.Logger.Info("Got remember-me that is not a bool")
			} else {
				if !rememberMe {
					session.Options.MaxAge = 0
				}
			}
		}
		if saveErr := session.Save(r, w); saveErr != nil {
			appcontext.Logger.WithError(saveErr).Error("Saving session failed")
		}
		return f(appcontext, w, r)
	}
}

// Logout will set the MaxAge of the session to -1 and thus destroy the session.
// It will also delete the session from the database.
func Logout(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	// try to get the key
	session, sessionErr := appcontext.SessionController.GetSession(r, appcontext.Store)
	if sessionErr != nil {
		appcontext.Logger.WithField("remote", r.RemoteAddr).WithError(sessionErr).Warn("Log out with invalid session")
	} else {
		// set session max age to 0
		session.Options.MaxAge = -1
		if saveErr := session.Save(r, w); saveErr != nil {
			appcontext.Logger.WithError(saveErr).Error("Failed to save session")
		}
		// try to get the key
		key, keyErr := appcontext.SessionController.GetKey(session)
		if keyErr != nil {
			appcontext.Logger.WithField("remote", r.RemoteAddr).WithError(keyErr).Warn("Log out with invalid session")
		} else {
			// finally we have the key and now we can remove the session
			if delErr := appcontext.SessionController.DeleteKey(key); delErr != nil {
				appcontext.Logger.WithField("remote", r.RemoteAddr).WithError(delErr).Error("Can't delete auth session key, this may be problematic!")
			}
		}
	}
	http.Redirect(w, r, "/login/", 302)
	return nil
}

// BootstrapLoginTemplate is the template for the login page.
func BootstrapLoginTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/login.html"))
}

// RootBootstrapTemplate is the template for the main page (/).
func RootBootstrapTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/home.html"))
}

// BootstrapDomainsTemplate is the template for the domains page.
func BootstrapDomainsTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/domains.html"))
}

// BootstrapUsersTemplate is the template for the users page.
func BootstrapUsersTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/users.html"))
}

// BootstrapAliasesTemplate is the template for the alias page.
func BootstrapAliasesTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/aliases.html"))
}

// BootstrapAdminsTemplate is the template for the admins page.
func BootstrapAdminsTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/admins.html"))
}

// BootstrapLicenseTemplate is the template for the license template.
func BootstrapLicenseTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/license.html"))
}

// BootstrapChangePWTemplate is the template for the change email password site.
func BootstrapChangePWTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/mailpw.html"))
}

// RenderLoginTemplate renders the template stored in
// appContext.Templates["login"].
// It adds the csrf.TemplateTag to the context of the template.
func RenderLoginTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	values := map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r)}
	return appContext.Templates["login"].ExecuteTemplate(w, "layout", values)
}

// RenderDomainsTemplate renders the template appContext.Templates["domains"].
func RenderDomainsTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appContext.Templates["domains"].ExecuteTemplate(w, "layout", nil)
}

// RenderUsersTemplate renders the template appContext.Templates["users"].
func RenderUsersTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appContext.Templates["users"].ExecuteTemplate(w, "layout", nil)
}

// RenderAliasesTemplate renders the template appContext.Templates["aliases"].
func RenderAliasesTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appContext.Templates["aliases"].ExecuteTemplate(w, "layout", nil)
}

// RenderLicenseTemplate renders the template appContext.Templates["license"].
func RenderLicenseTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appContext.Templates["license"].ExecuteTemplate(w, "layout", nil)
}

// RenderAdminsTemplate renders the template appContext.Templates["admins"].
func RenderAdminsTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appContext.Templates["admins"].ExecuteTemplate(w, "layout", nil)
}

// RenderRootTemplate renders the template appContext.Templates["root"].
func RenderRootTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appContext.Templates["root"].ExecuteTemplate(w, "layout", nil)
}

// RenderChangePWTemplate renders the template appContext.Templates["change-pw"].
// It adds the csrf.TemplateTag to the context of the template.
func RenderChangePWTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	values := map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r)}
	return appContext.Templates["change-pw"].ExecuteTemplate(w, "layout", values)
}

// CheckLogin checks the login data contained in the body of the request.
// The body must be a JSON object of the following form:
// {"username": <username>, "password": <password>, "remember-me": bool}
// It returns a 400 error if the syntax is wrong.
// If the login is correct it will create an auth session for the user, set
// the MaxAge field of the session etc.
// If the login succeeds it will return a 302 redirect to /.
// If the login fails it will return a 400.
func CheckLogin(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appcontext.Logger.Info("Invalid request syntax for login.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var loginData struct {
		Username, Password string
		RememberMe         bool `json:"remember-me"`
	}
	jsonErr := json.Unmarshal(body, &loginData)
	if jsonErr != nil {
		appcontext.Logger.Info("Invalid request syntax for login.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	// Validate the user
	userId, checkErr := appcontext.UserHandler.Validate(loginData.Username, []byte(loginData.Password))
	if checkErr == goauth.ErrUserNotFound {
		appcontext.Logger.WithField("username", loginData.Username).WithField("remote", r.RemoteAddr).Warn("Login attempt with unkown username")
		http.Error(w, "Login failed", 400)
		return nil
	}
	if checkErr != nil {
		// something really failed...
		// return this as an error!
		return checkErr
	}
	if userId == goauth.NoUserID {
		// login failed
		appcontext.Logger.WithField("username", loginData.Username).WithField("remote", r.RemoteAddr).Warn("Failed log in attempt")
		http.Error(w, "Login failed", 400)
		return nil
	}
	// everything ok!
	// create an auth session
	_, _, session, sessionErr := appcontext.SessionController.CreateAuthSession(r, appcontext.Store, userId, appcontext.DefaultSessionLifespan)
	if sessionErr != nil {
		// something went wrong, report it!
		return sessionErr
	}
	// save the session, set the max age to 0 if remember me is set to false
	// also set a session value to set the MaxAge to 0 all the time
	session.Values["remember-me"] = loginData.RememberMe
	if !loginData.RememberMe {
		session.Options.MaxAge = 0
	}
	saveErr := session.Save(r, w)
	if saveErr != nil {
		appcontext.Logger.Error("Saving session failed", saveErr)
	}
	http.Redirect(w, r, "/", 302)
	return nil
}

// LoginPageHandler is a handler that returns the login template for GET
// and uses CheckLogin for POST.
// All other methods will return a 400.
func LoginPageHandler(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for \"/login\": %s", r.Method), 400)
		return nil
	case getMethod:
		return RenderLoginTemplate(appcontext, w, r)
	case postMethod:
		return CheckLogin(appcontext, w, r)
	}
}

// ChangeSinglePasswordHandler returns on get the template for changing
// a single password (RenderChangePWTemplate) and on post calls
// ChangeSinglePw to update te password.
func ChangeSinglePasswordHandler(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for \"/password/\": %s", r.Method), 400)
		return nil
	case getMethod:
		return RenderChangePWTemplate(appContext, w, r)
	case postMethod:
		return ChangeSinglePw(appContext, w, r)
	}
}

// ChangeSinglePw reads a JSON encoded map from the body, it must have the form:
// {'mail': <Mail>, 'old_password': <Old>, 'new_password': <New>}
// It compares the old password with the one in the database, if this is not
// correct or the email doesn't exist it responds with a 400.
// If the password is correct the password gets updated.
func ChangeSinglePw(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.Info("Invalid request syntax for login.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var changeData struct {
		Mail        string
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	jsonErr := json.Unmarshal(body, &changeData)
	if jsonErr != nil {
		appContext.Logger.Info("Invalid request syntax for change-pw.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	// verify that the new password and email
	if emailErr := emailValid(changeData.Mail); emailErr != nil {
		appContext.Logger.WithError(emailErr).WithField("mail", changeData.Mail).Warn("Attempt to change password for an invalid email")
		http.Error(w, emailErr.Error(), 400)
		return nil
	}
	if pwErr := passwordValid(changeData.NewPassword); pwErr != nil {
		appContext.Logger.WithError(pwErr).WithField("mail", changeData.Mail).Warn("Attempt to change password to an invalid one.")
		http.Error(w, pwErr.Error(), 400)
		return nil
	}
	// everything seems fine, now get the entry from the database and validate the
	// old password
	id, storedPW, getErr := getUserPassword(appContext, changeData.Mail)
	if getErr != nil {
		appContext.Logger.WithError(getErr).WithField("mail", changeData.Mail).Warn("Error receiving user to change password.")
		http.Error(w, "Provided user and password don't match", 400)
		return nil
	}
	enc, salt, _, parseErr := getPWParts(storedPW)
	if parseErr != nil {
		return parseErr
	}
	equal, encErr := comparePasswords(changeData.OldPassword, salt, enc)
	if encErr != nil {
		return encErr
	}
	// check if they're equal, if yes allow the change
	if !equal {
		// report an error to the user
		appContext.Logger.WithError(getErr).WithFields(logrus.Fields{
			"mail":   changeData.Mail,
			"remote": r.RemoteAddr,
		}).Warn("Invalid attempt to change user password.")
		http.Error(w, "Provided user and password don't match", 400)
		return nil
	} else {
		// everything ok, update the password
		return ChangeUserPassword(appContext, id, changeData.NewPassword)
	}
}

// RootPageHandler is a handler that returns the root page.
func RootPageHandler(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for \"/\": %s", r.Method), 400)
		return nil
	case getMethod:
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return nil
		}
		return RenderRootTemplate(appcontext, w, r)
	}
}
