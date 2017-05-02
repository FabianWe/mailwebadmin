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
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/FabianWe/mailwebadmin"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
)

func main() {
	configDirPtr := flag.String("config", "./config", "Directory to store the configuration files.")
	actionPtr := flag.String("action", "", "Set to \"add\" if you want to add a useror \"list\" to list all users.")
	flag.Parse()
	configDir, configDirParseErr := filepath.Abs(*configDirPtr)
	if configDirParseErr != nil {
		log.WithError(configDirParseErr).Fatal("Can't parse config dir path: ", configDir)
	}
	appContext, configErr := mailwebadmin.ParseConfig(configDir, false)
	if configErr != nil {
		log.WithError(configErr).Fatal("Can't parse config file(s)")
	}
	reader := bufio.NewReader(os.Stdin)
	// determine the action
	switch strings.ToLower(*actionPtr) {
	default:
		appContext.Logger.WithField("action", *actionPtr).Fatal("Invalid action, must be either \"add\" or \"list\"")
		os.Exit(1)
	case "list":
		users, listErr := appContext.UserHandler.ListUsers()
		if listErr != nil {
			appContext.Logger.WithError(listErr).Fatal("Can't receive users list.")
		}
		fmt.Printf("There are %d admin users:\n", len(users))
		for _, username := range users {
			fmt.Printf("  - %s\n", username)
		}
	case "add":
		fmt.Print("Username: ")
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)
		fmt.Print("Password: ")
		bytePW, pwErr := terminal.ReadPassword(int(syscall.Stdin))
		if pwErr != nil {
			appContext.Logger.WithError(pwErr).Fatal("Can't read from stdin")
		}
		if _, insertErr := appContext.UserHandler.Insert(username, "", "", "", bytePW); insertErr != nil {
			appContext.Logger.WithError(insertErr).Fatal("Error while new admin.")
		}
		appContext.Logger.WithField("username", username).Info("Successfully added new admin user")
	}
}
