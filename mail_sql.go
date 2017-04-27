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

// This file contains SQL commands.

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	crypt "github.com/amoghe/go-crypt"
	"github.com/gorilla/securecookie"
	log "github.com/sirupsen/logrus"
)

// GenDovecotSHA512 generates the SHA512 hash of the given password.
// TODO: Also support SHA256, should be very easy.
func GenDovecotSHA512(password string) (string, error) {
	saltBytes := securecookie.GenerateRandomKey(12)
	if saltBytes == nil {
		return "", errors.New("Can't generate random bytes, probably an error with your random generator, do not continue!")
	}
	salt := base64.StdEncoding.EncodeToString(saltBytes)
	sha512, err := crypt.Crypt(password, fmt.Sprintf("$6$%s$", salt))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("{SHA512-CRYPT}%s", sha512), nil
}

// ParseMailParts splits an email address and returns the part before the
// @, the domain and an error if this is not possible.
func ParseMailParts(email string) (string, string, error) {
	components := strings.Split(email, "@")
	if len(components) != 2 {
		return "", "", fmt.Errorf("Invalid Email address \"%s\"", email)
	}
	if components[1] == "" {
		return "", "", fmt.Errorf("Invalid Email address \"%s\": Empty domain part", email)
	}
	return components[0], components[1], nil
}

// AddVirtualDomain adds the domain to the database.
func AddVirtualDomain(appContext *MailAppContext, domain string) (int64, error) {
	query := "INSERT INTO virtual_domains (name) VALUES (?);"
	res, err := appContext.DB.Exec(query, domain)
	if err != nil {
		return -1, err
	}
	id, _ := res.LastInsertId()
	appContext.Logger.WithFields(log.Fields{
		"domain-name": domain,
		"domain-id":   id,
	}).Info("Added new virtual domain")
	return id, nil
}

// DeleteVirtualDomain deletes the domain.
// If the domain was not found no error is returned, but the information gets
// logged.
func DeleteVirtualDomain(appContext *MailAppContext, domainID int64) error {
	query := "DELETE FROM virtual_domains WHERE id = ?;"
	res, err := appContext.DB.Exec(query, domainID)
	deleteNum, _ := res.RowsAffected()
	if err != nil {
		return err
	}
	if deleteNum != 1 {
		appContext.Logger.WithField("domain-id", domainID).Warn("Domain for delete not found")
	} else {
		appContext.Logger.WithField("domain-id", domainID).Info("Deleted domain")
	}
	return nil
}

// getDomainID returns the id in the virtual_domains table for the given domain
// name. It returns the id and nil if the entry was found and MaxInt64 and
// an error != nil if the domain was not found / an error occurred.
func getDomainID(appContext *MailAppContext, domain string) (int64, error) {
	query := "SELECT id FROM virtual_domains WHERE name = ?;"
	row := appContext.DB.QueryRow(query, domain)
	var id int64
	err := row.Scan(&id)
	if err != nil {
		return math.MaxInt64, err
	}
	return id, nil
}

// getDomainName is the counterpart of getDomainID.
func getDomainName(appContext *MailAppContext, domainID int64) (string, error) {
	query := "SELECT name FROM virtual_domains WHERE id = ?"
	row := appContext.DB.QueryRow(query, domainID)
	var domainName string
	err := row.Scan(&domainName)
	if err != nil {
		return "", err
	}
	return domainName, nil
}

// getUserName returns the username for a given user id.
// It returns the username, the domain and an error != nil if an error occurred.
func getUserName(appContext *MailAppContext, userID int64) (string, string, error) {
	query := "SELECT email FROM virtual_users WHERE id = ?"
	row := appContext.DB.QueryRow(query, userID)
	var email string
	err := row.Scan(&email)
	if err != nil {
		return "", "", err
	}
	// split in parts
	return ParseMailParts(email)
}

// AddMailUser adds a new mail user.
// On success it returns the insert id and nil, on failure -1 and an
// error != nil.
func AddMailUser(appContext *MailAppContext, email, plaintextPW string) (int64, error) {
	// first validate the email address, this pretty much makes the next test
	// useless, but ok...
	if validMail := emailValid(email); validMail != nil {
		return -1, validMail
	}
	// get the mail domain
	_, domain, parseErr := ParseMailParts(email)
	if parseErr != nil {
		return -1, parseErr
	}
	// encrypt the password
	pwHash, pwErr := GenDovecotSHA512(plaintextPW)
	if pwErr != nil {
		appContext.Logger.WithError(pwErr).Error("Error while encrypting password")
		return -1, pwErr
	}
	// get the domain id
	domainID, domainErr := getDomainID(appContext, domain)
	if domainErr != nil {
		return -1, domainErr
	}
	// now insert the user
	query := "INSERT INTO virtual_users (domain_id, email, password) VALUES(?, ?, ?);"
	res, insertErr := appContext.DB.Exec(query, domainID, email, pwHash)
	if insertErr != nil {
		appContext.Logger.WithError(insertErr).WithField("email", email).Error("Error inserting email into database")
		return -1, insertErr
	}
	appContext.Logger.WithField("email", email).Info("Added new email")
	id, _ := res.LastInsertId()
	return id, nil
}

// ChangeUserPassword changes the password for the user with the given id,
// it returns an error != nil if something went wrong.
func ChangeUserPassword(appContext *MailAppContext, emailID int64, plaintextPW string) error {
	// encrypt the password
	pwHash, pwErr := GenDovecotSHA512(plaintextPW)
	if pwErr != nil {
		return pwErr
	}
	// update the entry
	query := "UPDATE virtual_users SET password = ? WHERE id = ?;"
	res, updateErr := appContext.DB.Exec(query, pwHash, emailID)
	if updateErr != nil {
		return updateErr
	}
	numUpdate, _ := res.RowsAffected()
	if numUpdate != 1 {
		appContext.Logger.WithField("email-id", emailID).Warn("Update of email failed: email not found in virtual_users")
		return fmt.Errorf("Update password failed: email id \"%d\" not found in virtual_users", emailID)
	} else {
		appContext.Logger.WithField("email-id", emailID).Info("Changed email password")
	}
	return nil
}

// DelMailUser removes the user with the given id.
func DelMailUser(appContext *MailAppContext, emailID int64) error {
	query := "DELETE FROM virtual_users WHERE id = ?"
	res, err := appContext.DB.Exec(query, emailID)
	if err != nil {
		return err
	}
	deleteNum, _ := res.RowsAffected()
	if deleteNum != 1 {
		appContext.Logger.WithField("email-id", emailID).Warn("Email for delete not found")
	} else {
		appContext.Logger.WithField("email-id", emailID).Info("Deleted Email")
	}
	return nil
}

// AddAlias adds a new alias, it returns the id of the alias in the table
// and any error that occcurred.
func AddAlias(appContext *MailAppContext, source, destination string) (int64, error) {
	// the source could be an catch all alias, so we don't check if it's a valid
	// mail address but we check if it starts with @
	_, domain, sourceParseErr := ParseMailParts(source)
	if sourceParseErr != nil {
		return -1, sourceParseErr
	}

	if validMail := emailValid(destination); validMail != nil {
		return -1, validMail
	}

	_, _, destParseErr := ParseMailParts(destination)
	if destParseErr != nil {
		return -1, destParseErr
	}

	// lookup source domain
	domainID, domainErr := getDomainID(appContext, domain)
	if domainErr != nil {
		return -1, domainErr
	}

	// finally add it...
	query := "INSERT INTO virtual_aliases (domain_id, source, destination) VALUES(?, ?, ?);"
	res, insertErr := appContext.DB.Exec(query, domainID, source, destination)
	if insertErr != nil {
		appContext.Logger.WithError(insertErr).WithFields(log.Fields{
			"source": source,
			"dest":   destination,
		}).Warn("Adding alias failed")
		return -1, insertErr
	}
	appContext.Logger.WithFields(log.Fields{
		"source": source,
		"dest":   destination,
	}).Info("Added new alias")
	id, _ := res.LastInsertId()
	return id, nil
}

// DelAlias deletes the alias with the given id.
func DelAlias(appContext *MailAppContext, aliasID int64) error {
	query := "DELETE FROM virtual_aliases WHERE id = ?;"
	res, err := appContext.DB.Exec(query, aliasID)
	if err != nil {
		return err
	}
	deleteNum, _ := res.RowsAffected()
	if deleteNum != 1 {
		appContext.Logger.WithField("alias-id", aliasID).Warn("alias not found in virtual_aliases")
	} else {
		appContext.Logger.WithField("alias-id", aliasID).Info("Deleted alias")
	}
	return err
}

// ListVirtualDomains returns a map containing all virtual domains in the form
// id --> name.
func ListVirtualDomains(appContext *MailAppContext) (map[int64]string, error) {
	query := "SELECT id, name FROM virtual_domains;"
	rows, err := appContext.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[int64]string)
	for rows.Next() {
		var id int64
		var domain string
		scanErr := rows.Scan(&id, &domain)
		if scanErr != nil {
			return nil, scanErr
		}
		res[id] = domain
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return res, nil
}

// VirtualUser stores information about a virtual user, the mail address
// and the virtual domain id.
type VirtualUser struct {
	// DomainID is the id for the domain stored in the database.
	DomainID int64
	// Mail is the user Email.
	Mail string
}

func ListVirtualUsers(appContext *MailAppContext, domainID int64) (map[int64]*VirtualUser, error) {
	var query string
	queryArgs := make([]interface{}, 0)
	if domainID < 0 {
		query = "SELECT id, email, domain_id FROM virtual_users;"
	} else {
		query = "SELECT id, email, domain_id FROM virtual_users WHERE domain_id = ?;"
		queryArgs = append(queryArgs, domainID)
	}
	rows, err := appContext.DB.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[int64]*VirtualUser)
	for rows.Next() {
		var mail string
		var id, domainID int64
		scanErr := rows.Scan(&id, &mail, &domainID)
		if scanErr != nil {
			return nil, scanErr
		}
		res[id] = &VirtualUser{Mail: mail, DomainID: domainID}
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Alias stores information about a virtual alias.
type Alias struct {
	// DomainID is the domain id of the source mail.
	DomainID int64
	// Source and Dest are the source and destination email.
	Source, Dest string
}

// ListVirtualAliases lists all virtual aliases given an domainID.
// If domainID is < 0 it returns all entries (for all domains).
// The map contains entries of the form aliasID --> Alias.
func ListVirtualAliases(appContext *MailAppContext, domainID int64) (map[int64]*Alias, error) {
	var query string
	queryArgs := make([]interface{}, 0)
	if domainID < 0 {
		query = "SELECT id, domain_id, source, destination FROM virtual_aliases;"
	} else {
		query = "SELECT id, domain_id, source, destination FROM virtual_aliases WHERE domain_id = ?;"
		queryArgs = append(queryArgs, domainID)
	}
	rows, err := appContext.DB.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[int64]*Alias)
	for rows.Next() {
		var id, resDomainID int64
		var source, dest string
		scanErr := rows.Scan(&id, &resDomainID, &source, &dest)
		if scanErr != nil {
			return nil, scanErr
		}
		res[id] = &Alias{DomainID: resDomainID, Source: source, Dest: dest}
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListUserResult stores information about users. This is: All virtual users
// and users that are only an alias for something else combined.
// The VirtualUser is set to nil if it is only an alias and the virtual user ID.
// Otherwise it contains the valid user information.
// Alias for contains all aliases in the form aliasID -> Alias.
type ListUserResult struct {
	VirtualUser   *VirtualUser
	VirtualUserID int64
	AliasFor      map[int64]*Alias
}

// NewListResultForVirtualUser creates a new ListUserResult for a virtual user.
// AliasFor gets initialized to an empty map.
func NewListResultForVirtualUser(user *VirtualUser, virtualUserID int64) *ListUserResult {
	return &ListUserResult{VirtualUser: user,
		AliasFor: make(map[int64]*Alias), VirtualUserID: virtualUserID}
}

// NewListResultForVirtualAlias creates a new ListUserResult for a virtual alias.
// It sets VirtualUser to nil, VirtualUserID to -1 and creates an empty AliasFor
// map.
func NewListResultForVirtualAlias() *ListUserResult {
	return NewListResultForVirtualUser(nil, -1)
}

// ListAllUsers lists all users for a given domain.
// The result maps the email to the ListUserResult for that mail.
// Again a domainID < 0 means "all domains".
func ListAllUsers(appContext *MailAppContext, domainID int64) (map[string]*ListUserResult, error) {
	// we get the virtual users and all aliases for the domain, each in a different
	// goroutine
	// the first go routine simply adds each results it gets from ListVirtualUsers
	// to the result
	// afterwards we merge into this the information from ListVirtualAliases

	var virtualUsers map[int64]*VirtualUser
	var usersErr error

	var virtualAliases map[int64]*Alias
	var aliasErr error

	res := make(map[string]*ListUserResult)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		virtualUsers, usersErr = ListVirtualUsers(appContext, domainID)
		if usersErr != nil {
			return
		}
		for userID, user := range virtualUsers {
			listResult := NewListResultForVirtualUser(user, userID)
			res[user.Mail] = listResult
		}
	}()

	go func() {
		defer wg.Done()
		virtualAliases, aliasErr = ListVirtualAliases(appContext, domainID)
	}()

	wg.Wait()

	// first check for any errors, then merge the results
	if usersErr != nil {
		return nil, usersErr
	}
	if aliasErr != nil {
		return nil, aliasErr
	}

	// now for each alias: if the entry already exists (from virtual_users)
	// then just add the alias. Otherwise add a new result with
	// VirtualUserID = -1 and VirtualUser = nil
	// important: we're interested in the destination of the alias, not the source!
	for virtualID, virtualAlias := range virtualAliases {
		source := virtualAlias.Source
		// first check that we can parse the source mail correctly, it could
		// be an catch all in which case we don't want to put it here
		name, _, emailErr := ParseMailParts(source)
		if emailErr != nil {
			appContext.Logger.WithFields(log.Fields{
				"source":           source,
				"dest":             virtualAlias.Dest,
				"virtual-alias-id": virtualID,
			}).Warn("Invalid email in virtual_aliases table.")
			return nil, emailErr
		}
		if name == "" {
			continue
		}
		// now everything is ok... first we check if there is not an entry
		// in res yet
		if _, hasEntry := res[source]; !hasEntry {
			res[source] = NewListResultForVirtualAlias()
		}
		// finally we have ensured that there is an entry
		// so now we can add the new alias
		res[source].AliasFor[virtualID] = virtualAlias
	}
	return res, nil
}
