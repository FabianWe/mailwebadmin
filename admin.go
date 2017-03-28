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
	"bufio"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
	"time"

	"github.com/FabianWe/goauth"
	"github.com/gorilla/csrf"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
)

// Simplified, but should be ok
var MailRegexp = regexp.MustCompile(`^([a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$)`)

var ErrInvalidEmail = errors.New("Invalid Email address")

func IsValidMail(email string) error {
	res := MailRegexp.FindStringSubmatch(email)
	if res == nil {
		return ErrInvalidEmail
	} else {
		return nil
	}
}

func StaticHandler() http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
}

type MailAppContext struct {
	DB                *sql.DB
	ConfigDir         string
	Store             sessions.Store
	Logger            *logrus.Logger
	UserHandler       goauth.UserHandler
	SessionController *goauth.SessionController
	Keys              [][]byte
	Templates         map[string]*template.Template
}

func NewMailAppContext(db *sql.DB, configDir string, store sessions.Store) *MailAppContext {
	pwHandler := goauth.NewScryptHandler(nil)
	userHandler := goauth.NewMySQLUserHandler(db, pwHandler)
	sessionController := goauth.NewMySQLSessionController(db, "", "")
	res := &MailAppContext{DB: db, ConfigDir: configDir,
		Store: store, Logger: logrus.New(), UserHandler: userHandler,
		SessionController: sessionController, Templates: make(map[string]*template.Template)}
	if err := userHandler.Init(); err != nil {
		res.Logger.Fatal("Unable to connecto to database:", err)
	}
	if err := sessionController.Init(); err != nil {
		res.Logger.Fatal("Unable to connect to database:", err)
	}
	res.Logger.Level = logrus.InfoLevel
	return res
}

func (context *MailAppContext) ReadOrCreateKeys() {
	keyFile := path.Join(context.ConfigDir, "keys")
	var res [][]byte
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		context.Logger.Info("Key file doesn't exist, creating new keys.")
		// path does not exist, so get a new random pair
		pairs, genErr := GenKeyPair()
		if genErr != nil {
			context.Logger.Fatal("Can't create random key pair, there seems to be an error with your random engine. Stop now!", genErr)
		}
		// write the pairs
		writeErr := WriteKeyPairs(keyFile, pairs...)
		if writeErr != nil {
			context.Logger.Fatal("Can't write new keys to file:", writeErr)
		}
		res = pairs
	} else {
		// try to read from file
		pairs, readErr := ReadKeyPairs(keyFile)
		if readErr != nil {
			context.Logger.Fatal("Can't read key file:", readErr)
		}
		res = pairs
	}
	context.Keys = res
	context.Store = sessions.NewCookieStore(res...)
}

func ReadKeyPairs(path string) ([][]byte, error) {
	file, err := os.Open(path)
	defer file.Close()
	res := make([][]byte, 0)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		decode, decodeErr := base64.StdEncoding.DecodeString(line)
		if decodeErr != nil {
			return nil, decodeErr
		}
		res = append(res, decode)
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	if len(res)%2 != 0 {
		return nil, fmt.Errorf("Expected a list of keyPairs, i.e. length mod 2 == 0, got length %d", len(res))
	}
	return res, nil
}

func WriteKeyPairs(path string, keyPairs ...[]byte) error {
	if len(keyPairs)%2 != 0 {
		return fmt.Errorf("Expected a list of keyPairs, i.e. length mod 2 == 0, got length %d", len(keyPairs))
	}
	file, err := os.Create(path)
	defer file.Close()
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	// write each line
	for _, val := range keyPairs {
		_, err = writer.WriteString(base64.StdEncoding.EncodeToString(val) + "\n")
		if err != nil {
			return err
		}
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	return nil
}

func GenKeyPair() ([][]byte, error) {
	err := errors.New("Can't create a random key, something wrong with your random engine? Stop now!")
	authKey := securecookie.GenerateRandomKey(64)
	if authKey == nil {
		return nil, err
	}
	encryptionKey := securecookie.GenerateRandomKey(32)
	if encryptionKey == nil {
		return nil, err
	}
	return [][]byte{authKey, encryptionKey}, nil
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

func RenderLoginTemplate(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	values := map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r)}
	return appcontext.Templates["login"].ExecuteTemplate(w, "layout", values)
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
	_, _, session, sessionErr := appcontext.SessionController.CreateAuthSession(r, appcontext.Store, userId, time.Duration(5*time.Minute))
	if sessionErr != nil {
		// something went wrong, report it!
		return sessionErr
	}
	// save the session
	saveErr := session.Save(r, w)
	if saveErr != nil {
		appcontext.Logger.Error("Saving session failed", saveErr)
	}
	http.Redirect(w, r, "/", 302)
	return nil
}

func LoginPageHandler(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for \"/login\": %s", r.Method), 400)
		return nil
	case "GET":
		return RenderLoginTemplate(appcontext, w, r)
	case "POST":
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
	case "GET":
		return RenderRootTemplate(appcontext, w, r)
	}
}
