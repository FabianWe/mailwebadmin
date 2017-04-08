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
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Muh(source, destination string) error {
	return zipToFile(source, destination)
}

func getSourcePath(pattern, domain, user string) string {
	s := strings.Replace(pattern, "%d", domain, -1)
	return strings.Replace(s, "%n", user, -1)
}

func getDestPath(backupDir, domain, user string) string {
	var zipName string
	if user == "" {
		zipName = domain + ".zip"
	} else {
		zipName = fmt.Sprintf("%s-%s.zip", domain, user)
	}
	return filepath.Join(backupDir, zipName)
}

func deleteDomainDir(pattern, domain string) error {
	path := getSourcePath(pattern, domain, "")
	return os.RemoveAll(path)
}

func deleteUserDir(pattern, domain, user string) error {
	path := getSourcePath(pattern, domain, user)
	return os.RemoveAll(path)
}

func zipDomainDir(backupDir, pattern, domain string) error {
	sourcePath := getSourcePath(pattern, domain, "")
	destPath := getDestPath(backupDir, domain, "")
	return zipToFile(sourcePath, destPath)
}

func zipUserDir(backupDir, pattern, domain, user string) error {
	sourcePath := getSourcePath(pattern, domain, user)
	destPath := getDestPath(backupDir, domain, user)
	return zipToFile(sourcePath, destPath)
}

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

func zipToFile(source, destination string) error {
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
