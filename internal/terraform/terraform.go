package terraform

import (
	"archive/zip"
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

const (
	terraformVersion = "0.11.14"
)

var (
	darwinBinaryURL  = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_amd64.zip", terraformVersion, terraformVersion)
	linuxBinaryURL   = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_linux_amd64.zip", terraformVersion, terraformVersion)
	windowsBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_windows_amd64.zip", terraformVersion, terraformVersion)
)

type Client struct {
	rootDir             string
	terraformBinaryFile string
}

func New(rootDir string) (*Client, error) {
	terraformBinary, err := setupBinary(rootDir)
	if err != nil {
		return nil, err
	}

	client := &Client{
		rootDir:             rootDir,
		terraformBinaryFile: terraformBinary,
	}
	return client, nil
}

func (c *Client) Apply() ([]byte, error) {
	output, err := c.init()
	if err != nil {
		return output, err
	}

	cmd := c.setup(append([]string{"apply", "-auto-approve", c.rootDir}))
	return c.run(cmd)
}

func (c *Client) Destroy() ([]byte, error) {
	cmd := c.setup(append([]string{"destroy", "-auto-approve", c.rootDir}))
	return c.run(cmd)
}

func (c *Client) init() ([]byte, error) {
	cmd := c.setup(append([]string{"init", c.rootDir}))
	return c.run(cmd)
}

func (c *Client) setup(args []string) *exec.Cmd {
	cmd := exec.Command(c.terraformBinaryFile, args...)
	cmd.Dir = c.rootDir
	return cmd
}

func (c *Client) run(cmd *exec.Cmd) ([]byte, error) {
	cmdOutput, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	b := &bytes.Buffer{}
	scanner := bufio.NewScanner(cmdOutput)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
		b.WriteString(scanner.Text() + "\n")
	}

	cmd.Wait()
	return b.Bytes(), nil
}

func getTerraformURL() (string, error) {
	os := runtime.GOOS
	if os == "darwin" {
		return darwinBinaryURL, nil
	} else if os == "linux" {
		return linuxBinaryURL, nil
	} else if os == "windows" {
		return windowsBinaryURL, nil
	}
	return "", fmt.Errorf("unknown os: `%s`", os)
}

func setupBinary(outputDir string) (string, error) {
	terraformZipFilename := fmt.Sprintf("terraform-%v.zip", terraformVersion)
	terraformZipFullPathFilename := filepath.Join(outputDir, terraformZipFilename)

	downloadRequired := true
	if _, err := os.Stat(terraformZipFullPathFilename); err == nil {
		downloadRequired = false
		log.Infof("Skipping download of terraform zip as it already exists %v", terraformZipFullPathFilename)
	}

	if downloadRequired {
		fileHandler, err := os.Create(terraformZipFullPathFilename)
		if err != nil {
			return "", err
		}
		defer fileHandler.Close()

		url, err := getTerraformURL()
		if err != nil {
			return "", err
		}

		log.Infoln("Downloading Terraform binary from URL:", url)
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if _, err := io.Copy(fileHandler, resp.Body); err != nil {
			return "", err
		}
		if err := fileHandler.Sync(); err != nil {
			return "", err
		}
	}

	err := unzip(terraformZipFullPathFilename, outputDir)
	if err != nil {
		return "", err
	}

	terraformBinary := "terraform"
	if runtime.GOOS == "windows" {
		terraformBinary = "terraform.exe"
	}
	terraformBinaryFullPath := filepath.Join(outputDir, terraformBinary)
	if err := os.Chmod(terraformBinaryFullPath, 0700); err != nil {
		return "", err
	}

	return terraformBinaryFullPath, nil
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
