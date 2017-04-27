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

// This file contains functions for deleting the mail directory and backing it
// up before deletion in a zip file.

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// getSourcePath returns the pattern formatted given the domain and user.
// This means it returns the mail directory for that domain and user.
// It replaces the %d and %n placeholders.
func getSourcePath(pattern, domain, user string) string {
	s := strings.Replace(pattern, "%d", domain, -1)
	return strings.Replace(s, "%n", user, -1)
}

// getDestPath returns the zip file path for backing up domains / user accounts.
// The zip file is either called <domain>.zip when backing up a whole domain
// or <domain>-<user>.zip for user accounts.
func getDestPath(backupDir, domain, user string) string {
	var zipName string
	if user == "" {
		zipName = domain + ".zip"
	} else {
		zipName = fmt.Sprintf("%s-%s.zip", domain, user)
	}
	return filepath.Join(backupDir, zipName)
}

// deleteDomainDir deletes the directory for the given domain.
func deleteDomainDir(pattern, domain string) error {
	if containsErr := containsInvalidParts(domain); containsErr != nil {
		return containsErr
	}
	path := getSourcePath(pattern, domain, "")
	return os.RemoveAll(path)
}

// deleteUserDir deletes the directory for the given user and domain.
func deleteUserDir(pattern, domain, user string) error {
	if containsErr := containsInvalidParts(domain); containsErr != nil {
		return containsErr
	}
	if containsErr := containsInvalidParts(user); containsErr != nil {
		return containsErr
	}
	path := getSourcePath(pattern, domain, user)
	return os.RemoveAll(path)
}

// zipDomainDir zips the domain directory.
func zipDomainDir(backupDir, pattern, domain string) error {
	if containsErr := containsInvalidParts(domain); containsErr != nil {
		return containsErr
	}
	sourcePath := getSourcePath(pattern, domain, "")
	destPath := getDestPath(backupDir, domain, "")
	return zipToFile(sourcePath, destPath)
}

// zipUserDir zips the user directory.
func zipUserDir(backupDir, pattern, domain, user string) error {
	if containsErr := containsInvalidParts(domain); containsErr != nil {
		return containsErr
	}
	if containsErr := containsInvalidParts(user); containsErr != nil {
		return containsErr
	}
	sourcePath := getSourcePath(pattern, domain, user)
	destPath := getDestPath(backupDir, domain, user)
	return zipToFile(sourcePath, destPath)
}

// writeZip recursively adds all files under sourcePath to a zip archive.
// The zip will be written to the writer object.
func writeZip(sourcePath string, w io.Writer) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil
	}
	archive := zip.NewWriter(w)
	defer archive.Close()

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(sourcePath)
	}
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, sourcePath))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
	closeErr := archive.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// zipToFile writes all files under source to the destination file.
// It uses writeZip with a file writer.
// If source does not exist (dovecot never wrote some mails there)
// the file gets not created.
func zipToFile(source, destination string) error {
	// first check if source exists
	if _, err := os.Stat(source); os.IsNotExist(err) {
		// in this case return nil, no error simply no mails there yet
		return nil
	}
	file, err := os.Create(destination)
	defer file.Close()
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	zipErr := writeZip(source, writer)
	if zipErr != nil {
		return zipErr
	}
	if err = writer.Flush(); err != nil {
		return err
	}
	return nil
}
