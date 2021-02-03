package stack

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func renderTemplate(templateStr string, params interface{}) ([]byte, error) {
	templ, err := template.New("template").Delims("<%", "%>").Parse(templateStr)
	if err != nil {
		return nil, err
	}

	buffer := new(bytes.Buffer)

	if err = templ.Execute(buffer, params); err != nil {
		return nil, err
	}

	outputBytes, err := ioutil.ReadAll(buffer)
	if err != nil {
		return nil, err
	}
	return outputBytes, nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			err := os.MkdirAll(fpath, f.Mode())
			if err != nil {
				return err
			}
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}

			err = os.MkdirAll(fdir, f.Mode())
			if err != nil {
				log.Fatal(err)
				return err
			}
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func zipFiles(filename string, files []string) error {

	newfile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer newfile.Close()

	zipWriter := zip.NewWriter(newfile)
	defer zipWriter.Close()

	// Add files to zip
	for _, file := range files {

		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()

		// Get the file information
		info, err := zipfile.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Change to deflate to gain better compression
		// see http://golang.org/pkg/archive/zip/#pkg-constants
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, zipfile)
		if err != nil {
			return err
		}
	}
	return nil
}

type TempDir struct {
	path string
}

func NewTempDir(name string) (*TempDir, error) {
	path, err := ioutil.TempDir("", name)
	if err != nil {
		return nil, err
	}

	return &TempDir{
		path: path,
	}, nil
}

func (tempDir *TempDir) Save(filename string, contents []byte) (string, error) {
	path := filepath.Join(tempDir.path, filename)
	if err := ioutil.WriteFile(path, contents, 0700); err != nil {
		return "", err
	}

	return path, nil
}

func (tempDir *TempDir) Path(filename string) string {
	return filepath.Join(tempDir.path, filename)
}

func (tempDir *TempDir) Cleanup() error {
	return os.RemoveAll(tempDir.path)
}
