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
	// parse config dir flag

	configDirPtr := flag.String("config", "./config", "Directory to store the configuration files.")
	flag.Parse()
	configDir, configDirParseErr := filepath.Abs(*configDirPtr)
	if configDirParseErr != nil {
		log.WithError(configDirParseErr).Fatal("Can't parse config dir path: ", configDir)
	}

	appContext, configErr := mailwebadmin.ParseConfig(configDir)
	if configErr != nil {
		log.WithError(configErr).Fatal("Can't parse config file(s)")
	}
	http.Handle("/static/", mailwebadmin.StaticHandler())
	http.Handle("/favicon.ico", http.FileServer(http.Dir("static")))
	// get the templates
	appContext.Templates["login"] = mailwebadmin.BootstrapLoginTemplate()
	appContext.Templates["root"] = mailwebadmin.RootBootstrapTemplate()
	appContext.Templates["domains"] = mailwebadmin.BootstrapDomainsTemplate()
	appContext.Templates["users"] = mailwebadmin.BootstrapUsersTemplate()
	http.Handle("/login/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginPageHandler))
	http.Handle("/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RootPageHandler)))
	http.Handle("/listdomains/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.ListDomainsJSON)))
	http.Handle("/domains/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RenderDomainsTemplate)))
	http.Handle("/listusers", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.ListUsersJSON)))
	http.Handle("/users", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RenderUsersTemplate)))
	appContext.Logger.WithField("port", appContext.Port).Info("Ready. Waiting for requests.")
	appContext.Logger.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", appContext.Port),
		csrf.Protect(appContext.Keys[len(appContext.Keys)-1], csrf.Secure(false))(context.ClearHandler(http.DefaultServeMux))))
}
