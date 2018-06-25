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

	"github.com/dan-v/rattlesnakeos-stack/templates"
	log "github.com/sirupsen/logrus"
)

const terraformVersion = "0.11.7"

var darwinBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_darwin_amd64.zip", terraformVersion, terraformVersion)
var linuxBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_linux_amd64.zip", terraformVersion, terraformVersion)
var windowsBinaryURL = fmt.Sprintf("https://releases.hashicorp.com/terraform/%s/terraform_%s_windows_amd64.zip", terraformVersion, terraformVersion)

type TerraformClient struct {
	configDir string
	tempDir   *TempDir
	stdout    io.Writer
	stderr    io.Writer
}

func NewTerraformClient(config *TerraformConfig, stdout, stderr io.Writer) (*TerraformClient, error) {
	if err := setupBinary(config.TempDir); err != nil {
		return nil, err
	}

	log.Info("Rendering Terraform templates in temp dir " + config.TempDir.path)
	terraformFile, err := renderTemplate(templates.TerraformTemplate, config)
	if err != nil {
		return nil, err
	}

	// write out terraform template
	configDir := config.TempDir.Path("config")
	if err := os.Mkdir(configDir, 0777); err != nil {
		return nil, err
	}
	configPath := config.TempDir.Path("config/main.tf")
	if err := ioutil.WriteFile(configPath, terraformFile, 0777); err != nil {
		return nil, err
	}

	// write out shell script
	err = ioutil.WriteFile(config.TempDir.Path(ShellScriptFilename), config.ShellScriptBytes, 0644)
	if err != nil {
		return nil, err
	}

	// write out spot lambda function and zip it up
	err = ioutil.WriteFile(config.TempDir.Path(LambdaSpotFunctionFilename), config.LambdaSpotFunctionBytes, 0644)
	if err != nil {
		return nil, err
	}
	files := []string{config.TempDir.Path(LambdaSpotFunctionFilename)}
	output := config.TempDir.Path(LambdaSpotZipFilename)
	err = zipFiles(output, files)
	if err != nil {
		return nil, err
	}

	// create client and run init
	client := &TerraformClient{
		tempDir:   config.TempDir,
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

func (client *TerraformClient) Apply() error {
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

func (client *TerraformClient) Destroy() error {
	return client.terraform([]string{
		"destroy",
		"-force",
	}, client.stdout)
}

func (client *TerraformClient) terraform(args []string, stdout io.Writer) error {
	cmd := exec.Command(client.tempDir.Path("terraform"), args...)
	cmd.Dir = client.configDir
	cmd.Stdout = stdout
	cmd.Stderr = client.stderr
	return cmd.Run()
}

func (client *TerraformClient) Cleanup() error {
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

	if err := os.Chmod(tempDir.Path("terraform"), 0700); err != nil {
		return err
	}

	return nil
}
