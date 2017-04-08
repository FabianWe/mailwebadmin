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
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"unicode/utf8"

	"github.com/gorilla/csrf"
)

var errNoID = errors.New("No id provided")

func parseIDFromURL(regex *regexp.Regexp, url string) (int64, error) {
	res := regex.FindStringSubmatch(url)
	if res == nil {
		return -1, errors.New("No match")
	} else {
		idPart := res[2]
		// if no id was provided return -1 and errNoID
		if idPart == "" {
			return -1, errNoID
		}
		// try to parse int64
		id, parseErr := strconv.ParseInt(idPart, 10, 64)
		if parseErr != nil {
			return -1, parseErr
		}
		// everything ok
		return id, nil
	}
}

var listDomainsRegex = regexp.MustCompile(`^/api/domains/((\d+)/?)?$`)

func parseListDomainURL(url string) (int64, error) {
	return parseIDFromURL(listDomainsRegex, url)
}

var listUsersRegex = regexp.MustCompile(`^/api/users(/(\d+)/?)?$`)

func parseListUsersURL(url string) (int64, error) {
	return parseIDFromURL(listUsersRegex, url)
}

var listAliasRegx = regexp.MustCompile(`^/api/aliases/((\d+)/?)?$`)

func parseListAliasesURL(url string) (int64, error) {
	return parseIDFromURL(listAliasRegx, url)
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
	if domainLen := utf8.RuneCountInString(domainData.DomainName); domainLen > 50 || domainLen == 0 {
		http.Error(w, "Domain length must be not empty and <= 50 characters", 400)
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
	// first: check if the delete option is set, if so create backup if required and
	// delete
	if appcontext.Delete {
		// lookup domain name before deletion
		name, err := getDomainName(appcontext, domainID)
		// start a go routine, we don't want the user to wait
		go func() {
			if err != nil {
				appcontext.Logger.WithError(err).WithField("domain-id", domainID).Error("Can't create backup of domain directory, NOT deleting directory. Database lookup failed.")
				return
			}
			// backupr if requested
			if appcontext.Backup != "" {
				if backupErr := zipDomainDir(appcontext.Backup, appcontext.MailDir, name); backupErr != nil {
					appcontext.Logger.WithError(backupErr).WithField("domain-name", name).Error("Can't create backup of domain. NOT deleting directory")
					return
				} else {
					appcontext.Logger.WithField("domain-name", name).Info("Created backup for domain")
				}
			}
			// delete directory
			if delErr := deleteDomainDir(appcontext.MailDir, name); delErr != nil {
				appcontext.Logger.WithError(delErr).WithField("domain-name", name).Error("Can't delete domain directory")
				return
			} else {
				appcontext.Logger.WithField("domain-name", name).Info("Deleted domain directory.")
			}
		}()
	}
	// try to remove the domain
	return DeleteVirtualDomain(appcontext, domainID)
}

func deleteAlias(aliasID int64, appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return DelAlias(appContext, aliasID)
}

func ListDomainsJSON(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	domainID, parseErr := parseListDomainURL(r.URL.String())
	if parseErr != nil && parseErr != errNoID {
		http.NotFound(w, r)
		return nil
	}
	switch r.Method {
	case getMethod:
		if domainID >= 0 {
			http.Error(w, "Invalid GET request. Must be GET /api/domains/", 400)
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
		if domainID >= 0 {
			http.Error(w, "Invalid POST request to /api/domains/.", 400)
			return nil
		}
		return addDomain(appcontext, w, r)
	case deleteMethod:
		if domainID < 0 {
			http.Error(w, "Invalid DELETE request to /api/domains/: No id given.", 400)
			return nil
		}
		return deleteDomain(domainID, appcontext, w, r)
	default:
		http.Error(w, fmt.Sprintf("Invalid method for /api/domains/: %s", r.Method), 400)
		return nil
	}
}

func addMail(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.Info("Invalid request syntax to add a user")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var userData struct {
		Password, Mail string
	}
	jsonErr := json.Unmarshal(body, &userData)
	if jsonErr != nil {
		appContext.Logger.Info("Invalid request syntax to add a user")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if userLen := utf8.RuneCountInString(userData.Mail); userLen == 0 || userLen > 100 {
		appContext.Logger.WithField("mail", userData.Mail).Warn("Attempt to add a user with too long / short mail")
		return nil
	}
	if pwLen := utf8.RuneCountInString(userData.Password); pwLen < 6 {
		appContext.Logger.Warn("Attempt to add a user with a too short password")
		http.Error(w, "Password length too short", 400)
		return nil
	}
	// add user
	userID, addErr := AddMailUser(appContext, userData.Mail, userData.Password)
	if addErr != nil {
		return addErr
	}
	res := make(map[string]interface{})
	res["user-id"] = userID
	// encode to json
	jsonEnc, jsonEncErr := json.Marshal(res)
	if jsonEncErr != nil {
		// just log the error, but the insertion took place, so we return nil
		appContext.Logger.WithField("map", res).WithError(jsonEncErr).Warn("Can't enocode map to JSON")
		return nil
	}
	// everything ok
	w.Write(jsonEnc)
	return nil
}

func changePassword(userID int64, appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.Info("Invalid request syntax to change password")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var pwData struct {
		Password string
	}
	jsonErr := json.Unmarshal(body, &pwData)
	if jsonErr != nil {
		appContext.Logger.Info("Invalid request syntax to change password")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if pwLen := utf8.RuneCountInString(pwData.Password); pwLen < 6 {
		appContext.Logger.Warn("Attempt to change password to a password of length < 6")
		http.Error(w, "Password length too short", 400)
		return nil
	}
	return ChangeUserPassword(appContext, userID, pwData.Password)
}

func deleteMail(userID int64, appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	// first: check if the delete option is set, if so create backup if required and
	// delete
	if appContext.Delete {
		// lookup domain name before deletion
		mail, domain, err := getUserName(appContext, userID)
		// start a go routine, we don't want the user to wait
		go func() {
			if err != nil {
				appContext.Logger.WithError(err).WithField("user-id", userID).Error("Can't create backup of user directory, NOT deleting directory. Database lookup failed")
				return
			}
			// backupr if requested
			if appContext.Backup != "" {
				if backupErr := zipUserDir(appContext.Backup, appContext.MailDir, domain, mail); backupErr != nil {
					appContext.Logger.WithError(backupErr).WithField("user-id", userID).Error("Can't create backup of user id. NOT deleting directory")
					return
				} else {
					appContext.Logger.WithField("user-id", userID).Info("Created backup for user.")
				}
			}
			// delete directory
			if delErr := deleteUserDir(appContext.MailDir, domain, mail); delErr != nil {
				appContext.Logger.WithError(delErr).WithField("user-id", userID).Error("Can't delete user directory")
				return
			} else {
				appContext.Logger.WithField("user-id", userID).Info("Deleted user directory")
			}
		}()
	}
	// try to remove the domain
	return DelMailUser(appContext, userID)
}

func ListUsersJSON(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	userID, parseErr := parseListUsersURL(r.URL.Path)
	if parseErr != nil && parseErr != errNoID {
		http.NotFound(w, r)
		return nil
	}
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for /api/users/: %s", r.Method), 400)
		return nil
	case getMethod:
		if userID >= 0 {
			http.Error(w, "Invalid GET request. Must be GET /api/users/", 400)
			return nil
		}
		var domainID int64 = -1
		queryArgs := r.URL.Query()
		if domainValues, has := queryArgs["domain"]; has {
			if len(domainValues) != 1 {
				http.Error(w, "Invalid GET request. query params must contain at most one domain=DOMAIN-ID", 400)
				return nil
			}
			// get first element and try to parse it as an int
			if domainID, parseErr = strconv.ParseInt(domainValues[0], 10, 64); parseErr != nil {
				http.Error(w, "Invalid GET request. query params must contain at most one domain=DOMAIN-ID. DOMAIN-ID must be an int.", 400)
				return nil
			}
		}
		users, err := ListAllUsers(appcontext, domainID)
		if err != nil {
			return err
		}
		// set csrf header
		w.Header().Set("X-CSRF-Token", csrf.Token(r))
		// create json encoding
		jsonEnc, jsonErr := json.Marshal(users)
		if jsonErr != nil {
			return jsonErr
		}
		w.Write(jsonEnc)
		return nil
	case updateMethod:
		if userID < 0 {
			http.Error(w, "Invalid UPDATE request to /api/users/: No id given.", 400)
			return nil
		}
		return changePassword(userID, appcontext, w, r)
	case postMethod:
		if userID >= 0 {
			http.Error(w, "Invalid POST request to /api/users/.", 400)
			return nil
		}
		return addMail(appcontext, w, r)
	case deleteMethod:
		if userID < 0 {
			http.Error(w, "Invalid DELETE request to /api/users/: No id given.", 400)
			return nil
		}
		return deleteMail(userID, appcontext, w, r)
	}
}

func addAlias(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.Info("Invalid request syntax to add an alias")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var aliasData struct {
		Source, Dest string
	}
	jsonErr := json.Unmarshal(body, &aliasData)
	if jsonErr != nil {
		appContext.Logger.Info("Invalid request syntax to add an alias")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	// add user
	aliasID, addErr := AddAlias(appContext, aliasData.Source, aliasData.Dest)
	if addErr != nil {
		return addErr
	}
	res := make(map[string]interface{})
	res["alias-id"] = aliasID
	// encode to json
	jsonEnc, jsonEncErr := json.Marshal(res)
	if jsonEncErr != nil {
		// just log the error, but the insertion took place, so we return nil
		appContext.Logger.WithField("map", res).WithError(jsonEncErr).Warn("Can't enocode map to JSON")
		return nil
	}
	// everything ok
	w.Write(jsonEnc)
	return nil
}

func ListAliasesJSON(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	aliasID, parseErr := parseListAliasesURL(r.URL.String())
	if parseErr != nil && parseErr != errNoID {
		http.NotFound(w, r)
		return nil
	}
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for /api/aliases/: %s", r.Method), 400)
		return nil
	case getMethod:
		if aliasID >= 0 {
			http.Error(w, "Invalid GET request. Must be GET /api/aliases/", 400)
			return nil
		}
		res, err := ListVirtualAliases(appcontext, -1)
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
	case deleteMethod:
		if aliasID < 0 {
			http.Error(w, "Invalid DELETE request to /api/aliases/: No id given.", 400)
			return nil
		}
		return deleteAlias(aliasID, appcontext, w, r)
	case postMethod:
		if aliasID >= 0 {
			http.Error(w, "Invalid POST request to /api/aliases/.", 400)
			return nil
		}
		return addAlias(appcontext, w, r)
	}
}
