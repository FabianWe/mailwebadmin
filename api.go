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

	"github.com/gorilla/csrf"
	"github.com/sirupsen/logrus"
)

// This file defines methods for a REST-like API.

// errNoID is an error returned if request URL does not contain an id.
// This error will be returned by parseIDFromURL if no id (sequence of integers)
// is provided.
var errNoID = errors.New("No id provided")

// parseIDFromURL parses the id part from the url, that is the part in the URL after
// the prefix. For example: /api/domains/ is a valid URL, it can have an id
// for the domain, the URL would look like this: /api/domains/123456789.
// So it will return 123456789.
// If no id is found it will return -1 and errNoID, if the URL does not
// match the provided regex it will return -1 and an error != nil.
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

// listDomainsRegex is the regex for parsing the id from /api/domains.
var listDomainsRegex = regexp.MustCompile(`^/api/domains/((\d+)/?)?$`)

// parseListDomainURL parses the id from /api/domains.
func parseListDomainURL(url string) (int64, error) {
	return parseIDFromURL(listDomainsRegex, url)
}

// listUsersRegex is the regex for parsing the id from /api/users.
var listUsersRegex = regexp.MustCompile(`^/api/users(/(\d+)/?)?$`)

// parseListUsersURL parses the id from /api/users.
func parseListUsersURL(url string) (int64, error) {
	return parseIDFromURL(listUsersRegex, url)
}

// listAliasRegx is the regex for parsing the id from /api/aliases.
var listAliasRegx = regexp.MustCompile(`^/api/aliases/((\d+)/?)?$`)

// parseListAliasesURL parses the id from /api/aliases.
func parseListAliasesURL(url string) (int64, error) {
	return parseIDFromURL(listAliasRegx, url)
}

// adminsAliasRegx is the regex for parsing the username from /api/admins.
var adminsAliasRegx = regexp.MustCompile(`^/api/admins/((\w+)/?)?$`)

// parseAdminListURL parses the username from /api/admins.
func parseAdminListURL(url string) (string, error) {
	res := adminsAliasRegx.FindStringSubmatch(url)
	if res == nil {
		return "", errors.New("No match")
	} else {
		return res[2], nil
	}
}

// addDomain adds a new domain to the database.
// The body of the request must be a valid JSON dictionary of the form
// {"domain-name": <domain>}
// It checks if the name is valid according to domainNameValid.
// It will write the domain id of the new domain to the response as a JSON dictionary:
// {"domain-id": <id>}.
// On error it will return a 400.
func addDomain(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.WithError(readErr).Info("Invalid request syntax for add domain.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var domainData struct {
		DomainName string `json:"domain-name"`
	}
	jsonErr := json.Unmarshal(body, &domainData)
	if jsonErr != nil {
		appContext.Logger.WithError(jsonErr).Info("Invalid request syntax for add domain.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if domainErr := domainNameValid(domainData.DomainName); domainErr != nil {
		appContext.Logger.WithError(domainErr).WithField("domain-name", domainData.DomainName).Warn("Invalid domain name in add domain")
		http.Error(w, domainErr.Error(), 400)
		return nil
	}
	// try to add the domain, we write the result new id back to the writer
	domainID, err := AddVirtualDomain(appContext, domainData.DomainName)
	if err != nil {
		return err
	}
	res := make(map[string]interface{})
	res["domain-id"] = domainID
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

// deleteDomain deletes the domain with the given id.
// If appContext.Delete is set it will delete the domain directory and if also
// appContext.Backup is != "" it will first create a backup.
// However backup and deleting will run in a different goroutine (we don't wait for
// it to finish). The result will only get logged.
func deleteDomain(domainID int64, appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	// first: check if the delete option is set, if so create backup if required and
	// delete
	if appContext.Delete {
		// lookup domain name before deletion
		name, err := getDomainName(appContext, domainID)
		// start a go routine, we don't want the user to wait
		go func() {
			if err != nil {
				appContext.Logger.WithError(err).WithField("domain-id", domainID).Error("Can't create backup of domain directory, NOT deleting directory. Database lookup failed.")
				return
			}
			// backupr if requested
			if appContext.Backup != "" {
				if backupErr := zipDomainDir(appContext.Backup, appContext.MailDir, name); backupErr != nil {
					appContext.Logger.WithError(backupErr).WithField("domain-name", name).Error("Can't create backup of domain. NOT deleting directory")
					return
				} else {
					appContext.Logger.WithField("domain-name", name).Info("Created backup for domain")
				}
			}
			// delete directory
			if delErr := deleteDomainDir(appContext.MailDir, name); delErr != nil {
				appContext.Logger.WithError(delErr).WithField("domain-name", name).Error("Can't delete domain directory")
				return
			} else {
				appContext.Logger.WithField("domain-name", name).Info("Deleted domain directory.")
			}
		}()
	}
	// try to remove the domain
	return DeleteVirtualDomain(appContext, domainID)
}

// deleteAlias will delete the alias with the given id.
func deleteAlias(aliasID int64, appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	return DelAlias(appContext, aliasID)
}

// ListDomainsJSON is the main handler for domains.
// It either renders the template on GET, creates a new domain on POST or deletes
// a domain on DELETE.
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

// addMail adds a new mail user. It accepts a request in the following JSON dictionary
// format:
// {"mail": <mail>, "password": <password>}.
// It tests if the email is valid according to emailValid and if the password is valid
// according to passwordValid.
// On success it writes the following JSON to the response:
// {"user-id": <id>}.
func addMail(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.WithError(readErr).Info("Invalid request syntax to add a user")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var userData struct {
		Password, Mail string
	}
	jsonErr := json.Unmarshal(body, &userData)
	if jsonErr != nil {
		appContext.Logger.WithError(jsonErr).Info("Invalid request syntax to add a user")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if emailErr := emailValid(userData.Mail); emailErr != nil {
		appContext.Logger.WithError(emailErr).WithField("mail", userData.Mail).Warn("Attempt to add a user with wrong email")
		http.Error(w, emailErr.Error(), 400)
		return nil
	}
	if pwErr := passwordValid(userData.Password); pwErr != nil {
		appContext.Logger.WithError(pwErr).WithField("mail", userData.Mail).Warn("Attempt to add a user with invalid password")
		http.Error(w, pwErr.Error(), 400)
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

// changePassword changes the password for the user with the given id.
// It accepts JSON requests of the form:
// {"password": <password>}.
// It replies with a 400 if something went wrong.
func changePassword(userID int64, appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.WithError(readErr).Info("Invalid request syntax to change password")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var pwData struct {
		Password string
	}
	jsonErr := json.Unmarshal(body, &pwData)
	if jsonErr != nil {
		appContext.Logger.WithError(jsonErr).Info("Invalid request syntax to change password")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if pwErr := passwordValid(pwData.Password); pwErr != nil {
		appContext.Logger.WithError(pwErr).WithField("user-id", userID).Warn("Attempt to change a user password to an invalid password")
		http.Error(w, pwErr.Error(), 400)
		return nil
	}
	return ChangeUserPassword(appContext, userID, pwData.Password)
}

// deleteMail deletes the mail with the given id.
// If appContext.Delete is set it will delete the mail directory and if
// appContext.Backup != "" it will also backup the directory to the given
// location first.
// Again, as in deleteDomain this happens in a different goroutine we don't
// wait for.
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

// ListUsersJSON handles the /api/users domains.
// Works nearly as ListDomainsJSON.
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

// addAlias adds a new alias. The request must be JSON in the form
// {"source": <source-mail>, "dest": <destination-mail>}.
// It works as the other addXXX methods that have more documentation ;).
// It also checks if source and dest have a valid form.
// It writes the JSON dictionary:
// {"alias-id": <id>} if everything went ok.
func addAlias(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.WithError(readErr).Info("Invalid request syntax to add an alias")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var aliasData struct {
		Source, Dest string
	}
	jsonErr := json.Unmarshal(body, &aliasData)
	if jsonErr != nil {
		appContext.Logger.WithError(jsonErr).Info("Invalid request syntax to add an alias")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if sourceMailErr := emailValid(aliasData.Source); sourceMailErr != nil {
		appContext.Logger.WithError(sourceMailErr).WithFields(logrus.Fields{
			"source": aliasData.Source,
			"dest":   aliasData.Dest,
		}).Warn("Tried to add invalid alias")
		http.Error(w, sourceMailErr.Error(), 400)
		return nil
	}
	if destMailErr := emailValid(aliasData.Dest); destMailErr != nil {
		appContext.Logger.WithError(destMailErr).WithFields(logrus.Fields{
			"source": aliasData.Source,
			"dest":   aliasData.Dest,
		}).Warn("Tried to add invalid alias")
		http.Error(w, destMailErr.Error(), 400)
		return nil
	}
	// add alias
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

// ListAliasesJSON is the main handler for /api/aliases.
// It works nearly as ListDomainsJSON, which has more documentation ;).
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

// addAdmin adds a new admin user.
// See addDomain for more documentation, it does nearly the same thing.
// Username and password are verified first.
// It writes the new id to the response in the JSON format:
// {"admin-id": <id>}.
func addAdmin(appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.WithError(readErr).Info("Invalid request syntax for add admin.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var adminData struct {
		Username, Password string
	}
	jsonErr := json.Unmarshal(body, &adminData)
	if jsonErr != nil {
		appContext.Logger.WithError(jsonErr).Info("Invalid request syntax for add admin.")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if userNameErr := adminNameValid(adminData.Username); userNameErr != nil {
		appContext.Logger.WithError(userNameErr).WithField("admin-name", adminData.Username).Warn("Invalid admin user name")
		http.Error(w, userNameErr.Error(), 400)
		return nil
	}
	if pwErr := passwordValid(adminData.Password); pwErr != nil {
		appContext.Logger.WithError(pwErr).WithField("admin-name", adminData.Username).Warn("Invalid password for new admin user")
		http.Error(w, pwErr.Error(), 400)
		return nil
	}
	// try to add the user
	adminID, insertErr := appContext.UserHandler.Insert(adminData.Username, "", "", "", []byte(adminData.Password))
	if insertErr != nil {
		return insertErr
	}
	res := make(map[string]interface{})
	res["admin-id"] = adminID
	// encode to json
	jsonEnc, jsonEncErr := json.Marshal(res)
	if jsonEncErr != nil {
		// just log the error, but the insertion took place, so we return nil
		appContext.Logger.WithField("map", res).WithError(jsonEncErr).Warn("Can't enocode map to JSON")
		return nil
	}
	appContext.Logger.WithField("admin-name", adminData.Username).Info("Added new admin user")
	// everything ok
	w.Write(jsonEnc)
	return nil
}

// changeAdminPassword changes the password for the given admin user.
// The password is validated first.
// This method also deletes all sessions for the user.
func changeAdminPassword(userName string, appContext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		appContext.Logger.WithError(readErr).Info("Invalid request syntax to change admin password")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	var pwData struct {
		Password string
	}
	jsonErr := json.Unmarshal(body, &pwData)
	if jsonErr != nil {
		appContext.Logger.WithError(jsonErr).Info("Invalid request syntax to change admin password")
		http.Error(w, "Invalid request syntax", 400)
		return nil
	}
	if pwErr := passwordValid(pwData.Password); pwErr != nil {
		appContext.Logger.WithError(pwErr).WithField("admin-name", userName).Warn("Invalid password for admin user")
		http.Error(w, pwErr.Error(), 400)
		return nil
	}
	if updateErr := appContext.UserHandler.UpdatePassword(userName, []byte(pwData.Password)); updateErr != nil {
		return updateErr
	}
	// delete all sessions for the user, user has to login again
	adminID, getIDErr := appContext.UserHandler.GetUserID(userName)
	if getIDErr != nil {
		appContext.Logger.WithField("admin-user", userName).Error("Can't get admin id for user after changing password")
		// don't return an error, password was changed
		return nil
	}
	// now try to delete the sessions
	if _, delSessionsErr := appContext.SessionController.DeleteEntriesForUser(adminID); delSessionsErr != nil {
		appContext.Logger.WithField("admin-user", userName).Error("Can't delete sessions for user after changing password, user may be still logged in!")
		return nil
	}
	return nil
}

// ListAdminsJSON is the main handler for /api/admins.
// An admin is identified by the username, not an ID.
// On delete all sessions for the user will be deleted as well.
func ListAdminsJSON(appcontext *MailAppContext, w http.ResponseWriter, r *http.Request) error {
	userName, parseErr := parseAdminListURL(r.URL.String())
	if parseErr != nil {
		http.NotFound(w, r)
		return nil
	}
	switch r.Method {
	default:
		http.Error(w, fmt.Sprintf("Invalid method for /api/admins/: %s", r.Method), 400)
		return nil
	case getMethod:
		if userName != "" {
			http.Error(w, "Invalid GET request. Must be GET /api/admins/", 400)
			return nil
		}
		res, err := appcontext.UserHandler.ListUsers()
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
		if userName == "" {
			http.Error(w, "Invalid DELETE request to /api/admins/: No id given.", 400)
			return nil
		}
		// first get the id of the user, we need this later to destroy all sessions
		adminID, getIDErr := appcontext.UserHandler.GetUserID(userName)
		if getIDErr != nil {
			return getIDErr
		}
		// delete user, if this fails reply with internal server error
		if delErr := appcontext.UserHandler.DeleteUser(userName); delErr != nil {
			return delErr
		}
		// now delete all sessions for the user
		if _, delAllErr := appcontext.SessionController.DeleteEntriesForUser(adminID); delAllErr != nil {
			appcontext.Logger.WithField("admin-user", userName).Error("Can't delete sessions for user, he may still be logged in even after removal!")
			// deletion took place, so still we return nil
		}
		return nil
	case postMethod:
		if userName != "" {
			http.Error(w, "Invalid POST request to /api/admins/.", 400)
			return nil
		}
		return addAdmin(appcontext, w, r)
	case updateMethod:
		if userName == "" {
			http.Error(w, "Invalid UPDATE request to /api/admins/.", 400)
			return nil
		}
		return changeAdminPassword(userName, appcontext, w, r)
	}
}
