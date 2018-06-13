package stack

import (
	"github.com/dan-v/rattlesnakeos-stack/templates"
	log "github.com/sirupsen/logrus"
)

const (
	LambdaSpotFunctionFilename = "lambda_spot_function.py"
	LambdaSpotZipFilename      = "lambda_spot.zip"
	ShellScriptFilename        = "build.sh"
)

type TerraformConfig struct {
	Name                    string
	Region                  string
	Device                  string
	TempDir                 *TempDir
	ShellScriptFile         string
	ShellScriptBytes        []byte
	LambdaSpotZipFile       string
	LambdaSpotFunctionBytes []byte
	PreventShutdown         bool
}

func generateTerraformConfig(config StackConfig) (*TerraformConfig, error) {
	renderedLambdaSpotFunction, err := renderTemplate(templates.LambdaSpotFunctionTemplate, config)
	if err != nil {
		log.Fatalln("Failed to render Lambda spot function:", err)
	}

	renderedShellScript, err := renderTemplate(templates.ShellScriptTemplate, config)
	if err != nil {
		log.Fatalln("Failed to render shell script:", err)
	}

	tempDir, err := NewTempDir("rattlesnakeos-stack")
	if err != nil {
		return nil, err
	}

	conf := TerraformConfig{
		Name:                    config.Name,
		Region:                  config.Region,
		Device:                  config.Device,
		TempDir:                 tempDir,
		ShellScriptFile:         tempDir.Path(ShellScriptFilename),
		ShellScriptBytes:        renderedShellScript,
		LambdaSpotZipFile:       tempDir.Path(LambdaSpotZipFilename),
		LambdaSpotFunctionBytes: renderedLambdaSpotFunction,
		PreventShutdown:         config.PreventShutdown,
	}

	return &conf, nil
}
