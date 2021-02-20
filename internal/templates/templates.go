package templates

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/cloudaws"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"github.com/dan-v/rattlesnakeos-stack/internal/stack"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	defaultLambdaFunctionFilename    = "lambda_spot_function.py"
	defaultLambdaZipFilename         = "lambda_spot.zip"
	defaultBuildScriptFilename       = "build.sh"
	defaultTFMainFilename            = "main.tf"
	defaultGeneratedVarReplaceString = "#### <generated_vars_and_funcs.sh> ####"
)

type TemplateFiles struct {
	BuildScript       string
	BuildScriptVars   string
	LambdaTemplate    string
	TerraformTemplate string
}

type Config struct {
	Version               string
	Name                  string
	Region                string
	Device                string
	DeviceDetails         devices.Device
	Email                 string
	InstanceType          string
	InstanceRegions       string
	SkipPrice             string
	MaxPrice              string
	SSHKey                string
	Schedule              string
	ChromiumBuildDisabled bool
	ChromiumVersion       string
	CoreConfigRepo        string
	CustomConfigRepo      string
	LatestURL             string
	Cloud                 string
}

type Templates struct {
	config                 *Config
	templateFiles          *TemplateFiles
	buildScriptFilePath    string
	lambdaFunctionFilePath string
	lambdaZipFilePath      string
	tfMainFilePath         string
}

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

func (t *Templates) RenderAll() error {
	renderedBuildScript, err := t.renderBuildScript()
	if err != nil {
		return err
	}
	t.writeBuildScript(renderedBuildScript)

	renderedLambdaFunction, err := t.renderLambdaFunction()
	if err != nil {
		return err
	}
	t.writeLambdaFunction(renderedLambdaFunction)

	renderedTerraform, err := t.renderTerraform()
	if err != nil {
		return err
	}
	t.writeTerraform(renderedTerraform)

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
		stack.RattlesnakeOSStackReleasesURL,
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
		return nil, fmt.Errorf("failed to execute templates: %w", err)
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
	defer newFile.Close()

	zipWriter := zip.NewWriter(newFile)
	defer zipWriter.Close()

	for _, file := range files {
		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()

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
