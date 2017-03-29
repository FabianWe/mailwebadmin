// The MIT License (MIT)
//
// Copyright (c) 2017 Fabian Wenzelmann
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
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
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/FabianWe/goauth"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
)

type MailAppContext struct {
	DB                     *sql.DB
	ConfigDir              string
	Store                  sessions.Store
	Logger                 *logrus.Logger
	UserHandler            goauth.UserHandler
	SessionController      *goauth.SessionController
	Keys                   [][]byte
	Templates              map[string]*template.Template
	DefaultSessionLifespan time.Duration
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

type tomlConfig struct {
	DB           dbInfo       `toml:"mysql"`
	TimeSettings timeSettings `toml:"timers"`
}

type dbInfo struct {
	User, Password, DBName, Host string
	Port                         int
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type timeSettings struct {
	sessionLifespan duration `toml:"session-lifespan"`
	invalidKeyTimer duration `toml:"invalid-keys"`
}

func ParseConfig(configDir string) (*MailAppContext, error) {
	confPath := path.Join(configDir, "mailconf")
	var conf tomlConfig
	if _, err := toml.DecodeFile(confPath, &conf); err != nil {
		return nil, err
	}
	if conf.DB.User == "" {
		conf.DB.User = "root"
	}
	if conf.DB.Port == 0 {
		conf.DB.Port = 3306
	}
	if conf.DB.Host == "" {
		conf.DB.Host = "localhost"
	}
	if conf.DB.DBName == "" {
		conf.DB.DBName = "mailserver"
	}
	var confDBStr string

	if conf.DB.Password == "" {
		confDBStr = fmt.Sprintf("%s@tcp(%s:%d)/%s", conf.DB.User, conf.DB.Host, conf.DB.Port, conf.DB.DBName)
	} else {
		confDBStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", conf.DB.User, conf.DB.Password, conf.DB.Host, conf.DB.Port, conf.DB.DBName)
	}

	var invalidKeyTimer, sessionLifespan time.Duration

	if conf.TimeSettings.invalidKeyTimer.Duration == time.Duration(0) {
		invalidKeyTimer = time.Duration(24 * time.Hour)
	} else {
		invalidKeyTimer = conf.TimeSettings.invalidKeyTimer.Duration
	}

	if conf.TimeSettings.sessionLifespan.Duration == time.Duration(0) {
		sessionLifespan = time.Duration(336 * time.Hour)
	} else {
		sessionLifespan = conf.TimeSettings.sessionLifespan.Duration
	}

	db, openErr := sql.Open("mysql", confDBStr)
	if openErr != nil {
		return nil, openErr
	}

	pwHandler := goauth.NewScryptHandler(nil)
	userHandler := goauth.NewMySQLUserHandler(db, pwHandler)
	sessionController := goauth.NewMySQLSessionController(db, "", "")

	res := &MailAppContext{DB: db, ConfigDir: configDir,
		Store: nil, Logger: logrus.New(), UserHandler: userHandler,
		SessionController: sessionController, Templates: make(map[string]*template.Template)}

	res.DefaultSessionLifespan = sessionLifespan

	res.ReadOrCreateKeys()

	if err := userHandler.Init(); err != nil {
		res.Logger.Fatal("Unable to connecto to database:", err)
	}
	if err := sessionController.Init(); err != nil {
		res.Logger.Fatal("Unable to connect to database:", err)
	}

	res.Logger.Level = logrus.InfoLevel

	// start a goroutine to clear the sessions table
	sessionController.DeleteEntriesDaemon(invalidKeyTimer, nil, true)
	res.Logger.WithField("sleep-time", invalidKeyTimer).Info("Starting daemon to delete invalid keys")

	return res, nil
}
