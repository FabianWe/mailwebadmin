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
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// In this file there are some methods that check if certain inputs
// are valid, i.e. passwords are long enough but not too long etc.

// passwordValid checks if the password is valid, i.e. has a correct length.
func passwordValid(password string) error {
	len := utf8.RuneCountInString(password)
	if len < 6 {
		return errors.New("Password must be at least of length 6")
	}
	if len > 30 {
		return errors.New("Password length must be at most 30")
	}
	return nil
}

// containsInvalidParts is used to check if a string contains an invalid
// substring. Those invalid substrings are .., / and \.
// People could do something evil with those when we construct paths.
// So usernames and domains are checked with this methods.
func containsInvalidParts(s string) error {
	if strings.Contains(s, "..") || strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return errors.New("string contains one of the following invalid substrings: \"..\", \"/\", \"\\\"")
	}
	return nil
}

// domainNameValid checks if the domain is valid.
// Note: This is a very simplified version, it does not check any regex or
// something like that.
// It only checks the length as given in the sql specification and
// if the domains contains .. or / or \ (both are invalid and people
// could do something evil when forming paths).
// For this we use containsInvalidParts.
func domainNameValid(name string) error {
	if containErr := containsInvalidParts(name); containErr != nil {
		return containErr
	}
	if utf8.RuneCountInString(name) > 50 {
		return errors.New("Domain name must be at most 50.")
	}
	return nil
}

// mailRegexp is a very simplified version that checks if an email is valid.
var mailRegexp = regexp.MustCompile(`^([a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$)`)

// ErrInvalidEmail is the error returned if an string is not a valid email
// address.
var ErrInvalidEmail = errors.New("Invalid Email address")

// emailValid uses mailRegexp to check if a string is a valid email address.
// Furthermore we check the length of the mail and containsInvalidParts.
func emailValid(mail string) error {
	if partsErr := containsInvalidParts(mail); partsErr != nil {
		return partsErr
	}
	match := mailRegexp.FindStringSubmatch(mail)
	if match == nil {
		return ErrInvalidEmail
	}
	if utf8.RuneCountInString(mail) > 100 {
		return errors.New("Email length must be at most 100.")
	}
	return nil
}

// adminNameValid checks if user is a valid admin name (checks only the length
// of the string).
func adminNameValid(user string) error {
	if utf8.RuneCountInString(user) > 150 {
		return errors.New("Username length must be at most 150.")
	}
	return nil
}
