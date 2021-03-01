package templates

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/cloudaws"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	// DefaultReleasesURLTemplate is the URL to use for gathering latest versions of components for builds. This is
	// template string and should be provided a branch name (e.g. 11.0)
	DefaultReleasesURLTemplate = "https://raw.githubusercontent.com/RattlesnakeOS/releases/%v/latest.json"
	// DefaultCoreConfigRepo is the default core config repo to use
	DefaultCoreConfigRepo = "https://github.com/rattlesnakeos/core-config-repo"
	// DefaultRattlesnakeOSStackReleaseURL is the default rattlesnakeos-stack api releases github page
	DefaultRattlesnakeOSStackReleaseURL = "https://api.github.com/repos/dan-v/rattlesnakeos-stack/releases/latest"
)

const (
	defaultBuildScriptFilename       = "build.sh"
	defaultLambdaFunctionFilename    = "lambda_spot_function.py"
	defaultLambdaZipFilename         = "lambda_spot.zip"
	defaultTFMainFilename            = "main.tf"
	defaultGeneratedVarReplaceString = "#### <generated_vars_and_funcs.sh> ####"
)

var (
	// ErrTemplateExecute is returned if there is an error executing template
	ErrTemplateExecute = errors.New("error executing template")
)

// TemplateFiles are all of the files from the root templates directory
type TemplateFiles struct {
	// BuildScript is just the raw build shell script (no templating in this file)
	BuildScript string
	// BuildScriptVars is a template file with variables and functions that gets inserted into build script after render
	BuildScriptVars string
	// LambdaTemplate is a template file of the python Lambda function
	LambdaTemplate string
	// TerraformTemplate is a template file of the Terraform code
	TerraformTemplate string
}

// Config contains all of the template config values
type Config struct {
	// Version is the version of stack
	Version string
	// Name is the name of the stack
	Name string
	// Region is the region to deploy stack
	Region string
	// Device is the device to build for
	Device string
	// Device details is full device details
	DeviceDetails *devices.Device
	// Email is the email address to subscribe to notifications for stack
	Email string
	// InstanceType is the instance type to use for builds
	InstanceType string
	// InstanceRegions is the comma separated list of regions to use for builds
	InstanceRegions string
	// SkipPrice is the spot price at which the build should not start
	SkipPrice string
	// MaxPrice is the maximum spot price to set
	MaxPrice string
	// SSHKey is the name of the SSH key to use for launched spot instances
	SSHKey string
	// Schedule is the cron schedule for builds, can be left empty to disable
	Schedule string
	// ChromiumBuildDisabled can be used to turn of building Chromium
	ChromiumBuildDisabled bool
	// ChromiumVersion can be used to lock Chromium to a specific version
	ChromiumVersion string
	// CoreConfigRepo is the git repo to use for the core configuration of the OS
	CoreConfigRepo string
	// CoreConfigRepoBranch specifies which branch to use for the core configuration repo
	CoreConfigRepoBranch string
	// CustomConfigRepo is the git repo to use for customization on top of core
	CustomConfigRepo string
	// CustomConfigRepoBranch is the branch to use for the custom configuration repo
	CustomConfigRepoBranch string
	// ReleasesURL is the URL to use for gathering latest versions of components for builds
	ReleasesURL string
	// Cloud specifies which cloud to build on (only aws supported right now)
	Cloud string
}

// Templates provides the ability to render templates and write them to disk
type Templates struct {
	config                 *Config
	templateFiles          *TemplateFiles
	buildScriptFilePath    string
	lambdaFunctionFilePath string
	lambdaZipFilePath      string
	tfMainFilePath         string
}

// New returns an initialized Templates
func New(config *Config, templateFiles *TemplateFiles, outputDir string) (*Templates, error) {
	return &Templates{
		config:                 config,
		templateFiles:          templateFiles,
		buildScriptFilePath:    filepath.Join(outputDir, defaultBuildScriptFilename),
		lambdaFunctionFilePath: filepath.Join(outputDir, defaultLambdaFunctionFilename),
		lambdaZipFilePath:      filepath.Join(outputDir, defaultLambdaZipFilename),
		tfMainFilePath:         filepath.Join(outputDir, defaultTFMainFilename),
	}, nil
}

// RenderAll renders all templates and writes them to output directory
func (t *Templates) RenderAll() error {
	renderedBuildScript, err := t.renderBuildScript()
	if err != nil {
		return err
	}
	err = t.writeBuildScript(renderedBuildScript)
	if err != nil {
		return err
	}

	renderedLambdaFunction, err := t.renderLambdaFunction()
	if err != nil {
		return err
	}
	err = t.writeLambdaFunction(renderedLambdaFunction)
	if err != nil {
		return err
	}

	renderedTerraform, err := t.renderTerraform()
	if err != nil {
		return err
	}
	err = t.writeTerraform(renderedTerraform)
	if err != nil {
		return err
	}

	return nil
}

func (t *Templates) renderBuildScript() ([]byte, error) {
	renderedBuildScriptTemplate, err := renderTemplate(t.templateFiles.BuildScriptVars, t.config)
	if err != nil {
		return nil, err
	}

	// insert the generated vars and funcs into raw build script
	updatedBuildScript := strings.Replace(t.templateFiles.BuildScript, defaultGeneratedVarReplaceString, string(renderedBuildScriptTemplate), 1)

	return []byte(updatedBuildScript), nil
}

func (t *Templates) renderLambdaFunction() ([]byte, error) {
	regionAMIs, err := json.Marshal(cloudaws.GetAMIs())
	if err != nil {
		return nil, err
	}

	return renderTemplate(t.templateFiles.LambdaTemplate, struct {
		Config                        *Config
		RegionAMIs                    string
		RattlesnakeOSStackReleasesURL string
	}{
		t.config,
		string(regionAMIs),
		DefaultRattlesnakeOSStackReleaseURL,
	})
}

func (t *Templates) renderTerraform() ([]byte, error) {
	return renderTemplate(t.templateFiles.TerraformTemplate, struct {
		Config                  Config
		LambdaZipFileLocation   string
		BuildScriptFileLocation string
	}{
		*t.config,
		t.lambdaZipFilePath,
		t.buildScriptFilePath,
	})
}

func (t *Templates) writeBuildScript(renderedBuildScript []byte) error {
	return ioutil.WriteFile(t.buildScriptFilePath, renderedBuildScript, 0644)
}

func (t *Templates) writeLambdaFunction(renderedLambdaFunction []byte) error {
	if err := ioutil.WriteFile(t.lambdaFunctionFilePath, renderedLambdaFunction, 0644); err != nil {
		return err
	}

	if err := os.Chmod(t.lambdaFunctionFilePath, 0644); err != nil {
		return err
	}

	if err := zipFiles(t.lambdaZipFilePath, []string{t.lambdaFunctionFilePath}); err != nil {
		return err
	}
	return nil
}

func (t *Templates) writeTerraform(renderedTerraform []byte) error {
	if err := ioutil.WriteFile(t.tfMainFilePath, renderedTerraform, 0777); err != nil {
		return err
	}
	return nil
}

func renderTemplate(templateStr string, params interface{}) ([]byte, error) {
	temp, err := template.New("templates").Delims("<%", "%>").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	buffer := new(bytes.Buffer)
	if err = temp.Execute(buffer, params); err != nil {
		return nil, fmt.Errorf("%v: %w", err, ErrTemplateExecute)
	}

	outputBytes, err := ioutil.ReadAll(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated templates: %w", err)
	}

	return outputBytes, nil
}

func zipFiles(filename string, files []string) error {
	newFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer func() {
		_ = newFile.Close()
	}()

	zipWriter := zip.NewWriter(newFile)
	defer func() {
		_ = zipWriter.Close()
	}()

	for _, file := range files {
		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer func() {
			_ = zipfile.Close()
		}()

		info, err := zipfile.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

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
