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
	"database/sql"
	"log"
	"net/http"

	"github.com/FabianWe/mailwebadmin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/context"
	"github.com/gorilla/csrf"
)

func main() {
	db, openErr := sql.Open("mysql", "root:@/mailserver")
	if openErr != nil {
		log.Fatal(openErr)
	}
	appContext := mailwebadmin.NewMailAppContext(db, "config", nil)
	appContext.ReadOrCreateKeys()
	http.Handle("/static/", mailwebadmin.StaticHandler())
	http.Handle("/favicon.ico", http.FileServer(http.Dir("static")))
	// get the templates
	appContext.Templates["login"] = mailwebadmin.BootstrapLoginTemplate()
	appContext.Templates["root"] = mailwebadmin.RootBootstrapTemplate()
	http.Handle("/login/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginPageHandler))
	// http.HandleFunc("/login/", mailwebadmin.PostDistinguisher(execHandleFunc(loginTemplate, "layout", ""),
	// 	http.HandlerFunc(mailwebadmin.LoginPost)))
	http.Handle("/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(mailwebadmin.RootPageHandler)))
	f := func(appcontext *mailwebadmin.MailAppContext, w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("JUHU"))
		return nil
	}
	http.Handle("/muh/", mailwebadmin.NewMailAppHandler(appContext, mailwebadmin.LoginRequired(f)))
	// appContext.Logger.Fatal(http.ListenAndServe(":8080", context.ClearHandler(http.DefaultServeMux)))
	appContext.Logger.Fatal(http.ListenAndServe(":8080",
		csrf.Protect(appContext.Keys[len(appContext.Keys)-1], csrf.Secure(false))(context.ClearHandler(http.DefaultServeMux))))
}
