package main

import (
	_ "embed"
	"github.com/dan-v/rattlesnakeos-stack/cmd"
)

//go:embed templates/build.sh
var buildTemplate string

//go:embed templates/lambda.py
var lambdaTemplate string

//go:embed templates/terraform.tf
var terraformTemplate string

func main() {
	cmd.Execute(buildTemplate, lambdaTemplate, terraformTemplate)
}
