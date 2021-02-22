package main

import (
	_ "embed"
	"github.com/dan-v/rattlesnakeos-stack/cmd"
	"github.com/dan-v/rattlesnakeos-stack/internal/templates"
)

//go:embed AOSP_VERSION
var aospVersion string

//go:embed VERSION
var stackVersion string

//go:embed templates/build.sh
var buildScript string

//go:embed templates/generated_vars_and_funcs.sh
var buildScriptVars string

//go:embed templates/lambda.py
var lambdaTemplate string

//go:embed templates/terraform.tf
var terraformTemplate string

func main() {
	cmd.Execute(aospVersion, stackVersion, &templates.TemplateFiles{
		BuildScript:       buildScript,
		BuildScriptVars:   buildScriptVars,
		LambdaTemplate:    lambdaTemplate,
		TerraformTemplate: terraformTemplate,
	})
}
