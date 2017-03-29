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
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"

	"github.com/FabianWe/goauth"
	"github.com/gorilla/csrf"
)

// Simplified, but should be ok
var mailRegexp = regexp.MustCompile(`^([a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$)`)

var ErrInvalidEmail = errors.New("Invalid Email address")

const (
	getMethod    = "GET"
	postMethod   = "POST"
	deleteMethod = "DELETE"
)

func IsValidMail(email string) error {
	res := mailRegexp.FindStringSubmatch(email)
	if res == nil {
		return ErrInvalidEmail
	} else {
		return nil
	}
}

func StaticHandler() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
}

type AppHandleFunc func(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error

type MailAppHandler struct {
	*MailAppContext
	f AppHandleFunc
}

func NewMailAppHandler(context *MailAppContext, f AppHandleFunc) *MailAppHandler {
	return &MailAppHandler{MailAppContext: context, f: f}
}

func (handler *MailAppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := handler.f(handler.MailAppContext, w, r); err != nil {
		handler.MailAppContext.Logger.Error(err)
		http.Error(w, "Internal Server Error", 500)
	}
}

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
			appcontext.Logger.Error("Saving session failed", saveErr)
		}
		return f(appcontext, w, r)
	}
}

func BootstrapLoginTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/login.html"))
}

func RootBootstrapTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html"))
}

func BootstrapDomainsTemplate() *template.Template {
	return template.Must(template.ParseFiles("templates/default/base.html", "templates/default/domains.html"))
}

func RenderLoginTemplate(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	values := map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r)}
	return appcontext.Templates["login"].ExecuteTemplate(w, "layout", values)
}

func RenderDomainsTemplate(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appContext.Templates["domains"].ExecuteTemplate(w, "layout", nil)
}

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
		appcontext.Logger.WithField("username", loginData.Username).Info("Login attempt with unkown username")
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
		appcontext.Logger.WithField("username", loginData.Username).Warn("Failed log in attempt")
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
	// save the session, set the max age to -1 if remember me is set to false
	// also set a session value to set the MaxAge to -1 all the time
	session.Values["remember-me"] = loginData.RememberMe
	if !loginData.RememberMe {
		session.Options.MaxAge = 0
	}
	saveErr := session.Save(r, w)
	if saveErr != nil {
		appcontext.Logger.Error("Saving session failed", saveErr)
	}
	http.Redirect(w, r, "/welcome", 302)
	return nil
}

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

func RenderRootTemplate(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return appcontext.Templates["root"].ExecuteTemplate(w, "layout", nil)
}

func RootPageHandler(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for \"/\": %s", r.Method), 400)
		return nil
	case getMethod:
		return RenderRootTemplate(appcontext, w, r)
	}
}

var listDomainsRegex = regexp.MustCompile(`^/listdomains/(\d+/?)?$`)

func ParseListDomainURL(url string) (string, error) {
	res := listDomainsRegex.FindStringSubmatch(url)
	if res == nil {
		return "", errors.New("No match")
	} else {
		return res[1], nil
	}
}

func addDomain(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appcontext.Logger.Info("Invalid request syntax for add domain.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var domainData struct {
		DomainName string `json:"domain-name"`
	}
	jsonErr := json.Unmarshal(body, &domainData)
	if jsonErr != nil {
		appcontext.Logger.Info("Invalid request syntax for add domain.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	// try to add the domain, we write the result new id back to the writer
	domainID, err := AddVirtualDomain(appcontext, domainData.DomainName)
	if err != nil {
		return err
	}
	res := make(map[string]interface{})
	res["domain-id"] = domainID
	// encode to json
	jsonEnc, jsonEncErr := json.Marshal(res)
	if jsonEncErr != nil {
		// just log the error, but the insertion took place, so we return nil
		appcontext.Logger.WithField("map", res).WithError(jsonEncErr).Warn("Can't enocode map to JSON")
		return nil
	}
	// everything ok
	w.Write(jsonEnc)
	return nil
}

func deleteDomain(domainID int64, appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return nil
}

func ListDomainsJSON(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	part, parseErr := ParseListDomainURL(r.URL.String())
	if parseErr != nil {
		http.NotFound(w, r)
		return nil
	}
	switch r.Method {
	case getMethod:
		if part != "" {
			http.Error(w, "Invalid request. Must be GET /listdomains/", 400)
			return nil
		}
		res, err := ListVirtualDomains(appcontext)
		if err != nil {
			return err
		}
		// set csrf header
		w.Header().Set("X-CSRF-Token", csrf.Token(r))
		// create json encoding
		jsonEnc, jsonErr := json.Marshal(res)
		if jsonErr != nil {
			return jsonErr
		}
		w.Write(jsonEnc)
		return nil
	case postMethod:
		if part != "" {
			http.Error(w, "Invalid request to /listdomains/.", 400)
			return nil
		}
		return addDomain(appcontext, w, r)
	case deleteMethod:
		if part == "" {
			http.Error(w, "Invalid request to /listdomains/: No id given.", 400)
			return nil
		}
		domainID, parseErr := strconv.ParseInt(part, 10, 64)
		if parseErr != nil {
			http.Error(w, "Invalid request to /listdomains/: Id is not an integer.", 400)
			return nil
		}
		return deleteDomain(domainID, appcontext, w, r)
	default:
		http.Error(w, fmt.Sprintf("Invalid method for \"/\": %s", r.Method), 400)
		return nil
	}
}
