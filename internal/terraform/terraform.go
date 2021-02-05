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
	terraformVersion       = "0.11.8"
)

var (
	darwinBinaryURL   = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_amd64.zip", terraformVersion, terraformVersion)
	linuxBinaryURL    = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_linux_amd64.zip", terraformVersion, terraformVersion)
	windowsBinaryURL  = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_windows_amd64.zip", terraformVersion, terraformVersion)
)

type Client struct {
	terraformDir string
	terraformBinary string
}

func New(terraformDir string) (*Client, error) {
	terraformBinary, err := setupBinary(terraformDir)
	if err != nil {
		return nil, err
	}

	absolutePathTerraformBinary, err := filepath.Abs(terraformBinary)
	if err != nil {
		return nil, err
	}
	client := &Client{
		terraformDir:    terraformDir,
		terraformBinary: absolutePathTerraformBinary,
	}
	return client, nil
}

func (c *Client) Apply(args []string) ([]byte, error) {
	cmd := c.setup(append([]string{"apply"}, args...))
	return c.run(cmd)
}

func (c *Client) Init(args []string) ([]byte, error) {
	cmd := c.setup(append([]string{"init"}, args...))
	return c.run(cmd)
}

func (c *Client) Destroy(args []string) ([]byte, error) {
	cmd := c.setup(append([]string{"destroy"}, args...))
	return c.run(cmd)
}

func (c *Client) setup(args []string) *exec.Cmd {
	fmt.Printf("running %v %v\n", c.terraformBinary, args)
	cmd := exec.Command(c.terraformBinary, args...)
	cmd.Dir = c.terraformDir
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
	terraformZipFilename := filepath.Join(outputDir, "terraform.zip")
	fileHandler, err := os.Create(terraformZipFilename)
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

	err = unzip(terraformZipFilename, outputDir)
	if err != nil {
		return "", err
	}

	terraformBinary := "terraform"
	if runtime.GOOS == "windows" {
		terraformBinary = "terraform.exe"
	}
	if err := os.Chmod(filepath.Join(outputDir, terraformBinary), 0700); err != nil {
		return "", err
	}

	return filepath.Join(outputDir, terraformBinary), nil
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