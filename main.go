package main

import (
	_ "embed"
	"github.com/dan-v/rattlesnakeos-stack/cmd"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"github.com/dan-v/rattlesnakeos-stack/internal/templates"
	"log"
)

var (
	//go:embed AOSP_VERSION
	aospVersion string
	//go:embed VERSION
	stackVersion string
	//go:embed templates/build.sh
	buildScript string
	//go:embed templates/generated_vars_and_funcs.sh
	buildScriptVars string
	//go:embed templates/lambda.py
	lambdaTemplate string
	//go:embed templates/terraform.tf
	terraformTemplate string
)

var allDevices = []*devices.Device{
	&devices.Device{
		Name:     "blueline",
		Friendly: "Pixel 3",
		Family:   "crosshatch",
		AVBMode:  devices.AVBModeChained,
		ExtraOTA: devices.ExtraOTARetrofitDynamicPartitions,
	},
	&devices.Device{
		Name:     "crosshatch",
		Friendly: "Pixel 3 XL",
		Family:   "crosshatch",
		AVBMode:  devices.AVBModeChained,
		ExtraOTA: devices.ExtraOTARetrofitDynamicPartitions,
	},
	&devices.Device{
		Name:     "sargo",
		Friendly: "Pixel 3a",
		Family:   "bonito",
		AVBMode:  devices.AVBModeChained,
		ExtraOTA: devices.ExtraOTARetrofitDynamicPartitions,
	},
	&devices.Device{
		Name:     "bonito",
		Friendly: "Pixel 3a XL",
		Family:   "bonito",
		AVBMode:  devices.AVBModeChained,
		ExtraOTA: devices.ExtraOTARetrofitDynamicPartitions,
	},
	&devices.Device{
		Name:     "flame",
		Friendly: "Pixel 4",
		Family:   "coral",
		AVBMode:  devices.AVBModeChainedV2,
	},
	&devices.Device{
		Name:     "coral",
		Friendly: "Pixel 4 XL",
		Family:   "coral",
		AVBMode:  devices.AVBModeChainedV2,
	},
	&devices.Device{
		Name:     "sunfish",
		Friendly: "Pixel 4a",
		Family:   "sunfish",
		AVBMode:  devices.AVBModeChainedV2,
	},
	&devices.Device{
		Name:     "bramble",
		Friendly: "Pixel 4a 5G",
		Family:   "bramble",
		AVBMode:  devices.AVBModeChainedV2,
	},
	&devices.Device{
		Name:     "redfin",
		Friendly: "Pixel 5",
		Family:   "redfin",
		AVBMode:  devices.AVBModeChainedV2,
	},
	&devices.Device{
		Name:     "barbet",
		Friendly: "Pixel 5a",
		Family:   "barbet",
		AVBMode:  devices.AVBModeChainedV2,
	},
}

func main() {
	supportedDevices, err := devices.NewSupportedDevices(allDevices...)
	if err != nil {
		log.Fatal(err)
	}

	cmd.Execute(supportedDevices, aospVersion, stackVersion, &templates.TemplateFiles{
		BuildScript:       buildScript,
		BuildScriptVars:   buildScriptVars,
		LambdaTemplate:    lambdaTemplate,
		TerraformTemplate: terraformTemplate,
	})
}
