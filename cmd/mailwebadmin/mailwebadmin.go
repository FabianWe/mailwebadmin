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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/FabianWe/mailwebadmin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/context"
	"github.com/gorilla/csrf"
)

func main() {
	configDirPtr := flag.String("config", "./config", "Directory to store the configuration files.")
	apiOnlyPtr := flag.Bool("api-only", false, "If set only the API services are started, not the user interface")
	flag.Parse()
	configDir, configDirParseErr := filepath.Abs(*configDirPtr)
	if configDirParseErr != nil {
		log.WithError(configDirParseErr).Fatal("Can't parse config dir path: ", configDir)
	}

	appContext, configErr := mailwebadmin.ParseConfig(configDir, true)
	if configErr != nil {
		log.WithError(configErr).Fatal("Can't parse config file(s)")
	}

	// if api only is set to false start the user interface
	if !*apiOnlyPtr {
		// get the templates
		appContext.Templates["login"] = mailwebadmin.BootstrapLoginTemplate()
		appContext.Templates["root"] = mailwebadmin.RootBootstrapTemplate()
		appContext.Templates["domains"] = mailwebadmin.BootstrapDomainsTemplate()
		appContext.Templates["users"] = mailwebadmin.BootstrapUsersTemplate()
		appContext.Templates["aliases"] = mailwebadmin.BootstrapAliasesTemplate()
		appContext.Templates["license"] = mailwebadmin.BootstrapLicenseTemplate()
		appContext.Templates["admins"] = mailwebadmin.BootstrapAdminsTemplate()
		appContext.Templates["change-pw"] = mailwebadmin.BootstrapChangePWTemplate()

		// start the interface
		http.Handle("/static/", mailwebadmin.StaticHandler())
		http.Handle("/favicon.ico", http.FileServer(http.Dir("static")))
		http.Handle("/login/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginPageHandler))
		http.Handle("/logout/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.Logout)))
		http.Handle("/license/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.RenderLicenseTemplate))
		http.Handle("/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RootPageHandler)))
		http.Handle("/domains/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RenderDomainsTemplate)))
		http.Handle("/users", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RenderUsersTemplate)))
		// TODO can we fix this?
		http.Handle("/users/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RenderUsersTemplate)))
		http.Handle("/aliases/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RenderAliasesTemplate)))
		http.Handle("/admins/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RenderAdminsTemplate)))
		http.Handle("/password/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.ChangeSinglePasswordHandler))
	}

	http.Handle("/api/domains/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.ListDomainsJSON)))
	http.Handle("/api/users", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.ListUsersJSON)))
	// really annoying, but I see no other way around this...
	// we want both /users and /users/
	http.Handle("/api/users/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.ListUsersJSON)))
	http.Handle("/api/aliases/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.ListAliasesJSON)))
	http.Handle("/api/admins/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.ListAdminsJSON)))
	appContext.Logger.WithField("port", appContext.Port).Info("Ready. Waiting for requests.")
	appContext.Logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", appContext.Port),
		csrf.Protect(appContext.Keys[len(appContext.Keys)-1], csrf.Secure(false))(context.ClearHandler(http.DefaultServeMux))))
	// appContext.Logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", appContext.Port),
	// 	csrf.Protect(appContext.Keys[len(appContext.Keys)-1])(context.ClearHandler(http.DefaultServeMux))))
}
