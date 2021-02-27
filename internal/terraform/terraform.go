package terraform

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// Version is the Terraform version that is downloaded and used
	// TODO: this version of Terraform is getting quite old, but don't have a great plan for seamless major version upgrade.
	Version = "0.11.14"
	// DefaultTerraformDestroyTimeout is the default timeout for running Terraform destroy
	DefaultTerraformDestroyTimeout = time.Minute * 2
)

var (
	darwinBinaryURL  = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_amd64.zip", Version, Version)
	linuxBinaryURL   = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_linux_amd64.zip", Version, Version)
	windowsBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_windows_amd64.zip", Version, Version)
)

// Client provides a basic Terraform client
type Client struct {
	rootDir             string
	terraformBinaryFile string
}

// New downloads the Terraform binary for current platform and returns an initialized Client
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

// Apply runs terraform init and apply
func (c *Client) Apply(ctx context.Context) ([]byte, error) {
	output, err := c.init(ctx)
	if err != nil {
		return output, err
	}

	cmd := c.setup(ctx, []string{"apply", "-auto-approve", c.rootDir})
	return c.run(cmd)
}

// Destroy runs terraform destroy
func (c *Client) Destroy(ctx context.Context) ([]byte, error) {
	cmd := c.setup(ctx, []string{"destroy", "-auto-approve", c.rootDir})
	return c.run(cmd)
}

func (c *Client) init(ctx context.Context) ([]byte, error) {
	cmd := c.setup(ctx, []string{"init", c.rootDir})
	return c.run(cmd)
}

func (c *Client) setup(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.terraformBinaryFile, args...)
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

	err = cmd.Wait()
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func getTerraformURL() (string, error) {
	osName := runtime.GOOS
	if osName == "darwin" {
		return darwinBinaryURL, nil
	} else if osName == "linux" {
		return linuxBinaryURL, nil
	} else if osName == "windows" {
		return windowsBinaryURL, nil
	}
	return "", fmt.Errorf("unknown os: `%s`", osName)
}

func setupBinary(outputDir string) (string, error) {
	terraformZipFilename := fmt.Sprintf("terraform-%v.zip", Version)
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
		defer func(){
			_ = fileHandler.Close()
		}()

		url, err := getTerraformURL()
		if err != nil {
			return "", err
		}

		log.Infoln("Downloading Terraform binary from URL:", url)
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer func(){
			_ = resp.Body.Close()
		}()

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
	defer func() {
		_ = r.Close()
	}()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}

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
				return err
			}
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
			_ = f.Close()
		}
		_ = rc.Close()
	}
	return nil
}
