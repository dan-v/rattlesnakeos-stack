package stack

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/dan-v/rattlesnakeos-stack/templates"
	log "github.com/sirupsen/logrus"
)

const terraformVersion = "0.11.8"

var darwinBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_amd64.zip", terraformVersion, terraformVersion)
var linuxBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_linux_amd64.zip", terraformVersion, terraformVersion)
var windowsBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_windows_amd64.zip", terraformVersion, terraformVersion)

const (
	lambdaFunctionFilename = "lambda_spot_function.py"
	lambdaZipFilename      = "lambda_spot.zip"
	buildScriptFilename    = "build.sh"
)

type terraformClient struct {
	configDir string
	tempDir   *TempDir
	stdout    io.Writer
	stderr    io.Writer
}

func newTerraformClient(config *AWSStack, stdout, stderr io.Writer) (*terraformClient, error) {
	tempDir, err := NewTempDir("rattlesnakeos-stack")
	if err != nil {
		return nil, err
	}

	if err := setupBinary(tempDir); err != nil {
		return nil, err
	}

	// write out shell script
	config.BuildScriptFileLocation = tempDir.Path(buildScriptFilename)
	if runtime.GOOS == "windows" {
		config.BuildScriptFileLocation = strings.Replace(config.BuildScriptFileLocation, "\\", "/", -1)
	}
	err = ioutil.WriteFile(config.BuildScriptFileLocation, config.renderedBuildScript, 0644)
	if err != nil {
		return nil, err
	}

	// write out lambda function and zip it up
	err = ioutil.WriteFile(tempDir.Path(lambdaFunctionFilename), config.renderedLambdaFunction, 0644)
	if err != nil {
		return nil, err
	}
	// handle potential issue with non default umask as lambda function must have at least 444 permissions to run
	if err = os.Chmod(tempDir.Path(lambdaFunctionFilename), 0644); err != nil {
		return nil, err
	}
	files := []string{tempDir.Path(lambdaFunctionFilename)}
	output := tempDir.Path(lambdaZipFilename)
	err = zipFiles(output, files)
	if err != nil {
		return nil, err
	}
	config.LambdaZipFileLocation = tempDir.Path(lambdaZipFilename)
	if runtime.GOOS == "windows" {
		config.LambdaZipFileLocation = strings.Replace(config.LambdaZipFileLocation, "\\", "/", -1)
	}

	// render terraform template
	renderedTerraform, err := renderTemplate(templates.TerraformTemplate, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to render terraform template: %v", err)
	}
	configDir := tempDir.Path("config")
	if err := os.Mkdir(configDir, 0777); err != nil {
		return nil, err
	}
	configPath := tempDir.Path("config/main.tf")
	if err := ioutil.WriteFile(configPath, renderedTerraform, 0777); err != nil {
		return nil, err
	}

	// create client and run init
	client := &terraformClient{
		tempDir:   tempDir,
		configDir: configDir,
		stdout:    stdout,
		stderr:    stderr,
	}
	devNull := bytes.NewBuffer(nil)
	if err := client.terraform([]string{"init"}, devNull); err != nil {
		io.Copy(stdout, devNull)
		return nil, err
	}
	return client, nil
}

func (client *terraformClient) Apply() error {
	client.terraform([]string{
		"plan",
		"-input=false",
		"-out=tfplan",
	}, client.stdout)
	return client.terraform([]string{
		"apply",
		"tfplan",
	}, client.stdout)
}

func (client *terraformClient) Destroy() error {
	return client.terraform([]string{
		"destroy",
		"-force",
	}, client.stdout)
}

func (client *terraformClient) terraform(args []string, stdout io.Writer) error {
	terraformBinary := "terraform"
	if runtime.GOOS == "windows" {
		terraformBinary = "terraform.exe"
	}
	cmd := exec.Command(client.tempDir.Path(terraformBinary), args...)
	cmd.Dir = client.configDir
	cmd.Stdout = stdout
	cmd.Stderr = client.stderr
	return cmd.Run()
}

func (client *terraformClient) Cleanup() error {
	return os.RemoveAll(client.tempDir.path)
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

func setupBinary(tempDir *TempDir) error {
	fileHandler, err := os.Create(tempDir.Path("terraform.zip"))
	if err != nil {
		return err
	}
	defer fileHandler.Close()

	url, err := getTerraformURL()
	if err != nil {
		return err
	}

	log.Infoln("Downloading Terraform binary from URL:", url)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(fileHandler, resp.Body); err != nil {
		return err
	}
	if err := fileHandler.Sync(); err != nil {
		return err
	}

	err = unzip(tempDir.Path("terraform.zip"), tempDir.path)
	if err != nil {
		return err
	}

	terraformBinary := "terraform"
	if runtime.GOOS == "windows" {
		terraformBinary = "terraform.exe"
	}
	if err := os.Chmod(tempDir.Path(terraformBinary), 0700); err != nil {
		return err
	}

	return nil
}
