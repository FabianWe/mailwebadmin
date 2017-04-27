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

// This file contains functions for parsing the configuration file and initializing
// the database.

import (
	"bufio"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/FabianWe/goauth"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
)

// MailAppContext stores all global options for all handlers.
type MailAppContext struct {
	// DB is the database to work on.
	DB *sql.DB
	// ConfigDir is the directory containing the configuration files.
	ConfigDir string
	// Store is the session store to be used. It gets initialized after reading
	// the key file.
	Store sessions.Store
	// Logger is used to log messages.
	Logger *logrus.Logger
	// UserHandler is used to administer admin users.
	UserHandler goauth.UserHandler
	// SessionController is used to control the admin users sessions.
	SessionController *goauth.SessionController
	// Keys stores the keys used for the auth sessions, see
	// http://www.gorillatoolkit.org/pkg/sessions for more details.
	// We assume that we always have pairs of keys: auth-key and encryption-key.
	Keys [][]byte
	// Templates stores all templates for rendering the pages.
	// See the main file for all templates used.
	Templates map[string]*template.Template
	// DefaultSessionLifespan is the lifespan of a session for an admin user.
	DefaultSessionLifespan time.Duration
	// Port is the port to run on, defaults to 80.
	Port int
	// MailDir is the pattern that returns the path of a user / domain.
	// It must contain the placeholders %d that gets replaced by the domain
	// and %n that gets replaced by the user name.
	// It defaults to "/var/vmail/%d/%n".
	// If no username is given (i.e. we want the directory for a whole domain)
	// %n gets replaced by the empty string, so this must return a valid path.
	MailDir string
	// Delete is set to true if when deleting a domain / user from the database
	// the corresponding directories should be deleted as well.
	Delete bool
	// Backup is the directory where the backup files are stored.
	// If set to the empty string no backups will be created.
	// Otherwise backups (as zip files) are created inside this directory.
	// It defaults to the empty string.
	Backup string
}

// ReadOrCreateKeys either reads the key file or, if it doesn't exist, creates
// a key pair. contenxt.Keys are set to the keys read / created.
// If a key file (inside ConfigDir/keys) exists it must be a file with
// a key in each line.
// There must be pairs stored in the file: A list of
// auth-key
// encryption-key
// ...
// The auth-keys must be 64 byte long, the encryption keys 32 bytes long.
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

// ReadKeyPairs reads the key pairs from the key files.
// It returns an error if there are not % 2 keys in the file or something
// during reading goes wrong.
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

// WriteKeyPairs writes the key pairs to the file specified in path.
// All keys get base64 encoded.
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

// GenKeyPair generates a new auth-key, encryption-key pair.
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

// tomlConfig is used to parse the configuration file.
// See wiki for configuration options.
type tomlConfig struct {
	Port         int
	MailDir      string `toml:"maildir"`
	Delete       bool
	Backup       string
	DB           dbInfo       `toml:"mysql"`
	TimeSettings timeSettings `toml:"timers"`
}

// dbInfo is used in the server config in the [mysql] section.
type dbInfo struct {
	User, Password, DBName, Host string
	Port                         int
}

// duration is a time that simply stores a time.Duration and can be
// unmarshalled.
type duration struct {
	time.Duration
}

// UnmarshalText Unmarshal the given text and transform to a duration object.
func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// timeSettings is used in the server config in the [timers] section.
type timeSettings struct {
	sessionLifespan duration `toml:"session-lifespan"`
	invalidKeyTimer duration `toml:"invalid-keys"`
}

// ParseConfig parses the configuration file (called mailconf in the config dir).
// It sets some values to a default value, connects to and initializes the
// database.
// It calls ReadOrCreateKeys.
func ParseConfig(configDir string) (*MailAppContext, error) {
	confPath := path.Join(configDir, "mailconf")
	var conf tomlConfig
	if _, err := toml.DecodeFile(confPath, &conf); err != nil {
		return nil, err
	}
	if conf.Port == 0 {
		conf.Port = 80
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
	if conf.MailDir == "" {
		conf.MailDir = "/var/vmail/%d/%n"
	}

	if !strings.Contains(conf.MailDir, "%d") || !strings.Contains(conf.MailDir, "%n") {
		return nil, errors.New("Invalid maildir in conf: Must contain %d and %n")
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
		sessionLifespan = time.Duration(168 * time.Hour)
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
	res.Port = conf.Port
	res.MailDir = conf.MailDir
	res.Delete = conf.Delete
	res.Backup = conf.Backup

	res.ReadOrCreateKeys()

	if err := userHandler.Init(); err != nil {
		res.Logger.Fatal("Unable to connecto to database:", err)
	}
	if err := sessionController.Init(); err != nil {
		res.Logger.Fatal("Unable to connect to database:", err)
	}
	logrusFormatter := logrus.TextFormatter{}
	logrusFormatter.FullTimestamp = true

	res.Logger.Level = logrus.InfoLevel
	res.Logger.Formatter = &logrusFormatter

	// start a goroutine to clear the sessions table
	sessionController.DeleteEntriesDaemon(invalidKeyTimer, nil, true)
	res.Logger.WithField("sleep-time", invalidKeyTimer).Info("Starting daemon to delete invalid keys")

	return res, nil
}
