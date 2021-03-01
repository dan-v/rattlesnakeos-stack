package templates

import (
	"fmt"
	"github.com/dan-v/rattlesnakeos-stack/internal/devices"
	"github.com/stretchr/testify/assert"
	"regexp"
	"strings"
	"testing"
)

func TestTemplates_RenderBuildScript(t *testing.T) {
	tests := map[string]struct {
		config          *Config
		buildScript     string
		buildScriptVars string
		expected        []byte
		expectedErr     error
	}{
		"happy path build script render": {
			config: testConfig,
			buildScript: dedent(fmt.Sprintf(`above
				%v
				below`, defaultGeneratedVarReplaceString)),
			buildScriptVars: dedent(`DEVICE="<% .Device %>"
				DEVICE_FRIENDLY="<% .DeviceDetails.Friendly %>"
				DEVICE_FAMILY="<% .DeviceDetails.Family %>"
				DEVICE_AVB_MODE="<% .DeviceDetails.AVBMode %>"
				DEVICE_EXTRA_OTA=<% .DeviceDetails.ExtraOTA %>
				STACK_NAME="<% .Name %>"
				STACK_VERSION="<% .Version %>"
				CHROMIUM_BUILD_DISABLED="<% .ChromiumBuildDisabled %>"
				CORE_CONFIG_REPO="<% .CoreConfigRepo %>"
				CORE_CONFIG_REPO_BRANCH="<% .CoreConfigRepoBranch %>"
				CUSTOM_CONFIG_REPO="<% .CustomConfigRepo %>"
				CUSTOM_CONFIG_REPO_BRANCH="<% .CustomConfigRepoBranch %>"`),
			expected: []byte(dedent(`above
				DEVICE="test device"
				DEVICE_FRIENDLY="friendly"
				DEVICE_FAMILY="family"
				DEVICE_AVB_MODE="avb mode"
				DEVICE_EXTRA_OTA=extra ota
				STACK_NAME="test stack"
				STACK_VERSION="test version"
				CHROMIUM_BUILD_DISABLED="false"
				CORE_CONFIG_REPO="core-config-repo"
				CORE_CONFIG_REPO_BRANCH="core-config-repo-branch"
				CUSTOM_CONFIG_REPO="custom-config-repo"
				CUSTOM_CONFIG_REPO_BRANCH="custom-config-repo-branch"
				below`)),
			expectedErr: nil,
		},
		"bad template variable returns error": {
			config:          testConfig,
			buildScript:     defaultGeneratedVarReplaceString,
			buildScriptVars: dedent(`DEVICE="<% .Bad %>"`),
			expected:        nil,
			expectedErr:     ErrTemplateExecute,
		},
		"buildscript with no defaultGeneratedVarReplaceString does not have buildScriptVars inserted": {
			config:          testConfig,
			buildScript:     "",
			buildScriptVars: `DEVICE="<% .Device %>"`,
			expected:        []byte(""),
			expectedErr:     nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			templateFiles, err := New(tc.config, &TemplateFiles{BuildScript: tc.buildScript, BuildScriptVars: tc.buildScriptVars}, "")
			assert.Nil(t, err)

			output, err := templateFiles.renderBuildScript()
			assert.ErrorIs(t, err, tc.expectedErr)

			assert.Equal(t, string(tc.expected), string(output))
		})
	}
}

func TestTemplates_RenderLambdaFunction(t *testing.T) {
	tests := map[string]struct {
		config         *Config
		lambdaTemplate string
		expected       []byte
		expectedErr    error
	}{
		"happy path build script render": {
			config: testConfig,
			lambdaTemplate: dedent(`DEVICE="<% .Config.Device %>"
				DEVICE_FRIENDLY="<% .Config.DeviceDetails.Friendly %>"
				DEVICE_FAMILY="<% .Config.DeviceDetails.Family %>"
				DEVICE_AVB_MODE="<% .Config.DeviceDetails.AVBMode %>"
				DEVICE_EXTRA_OTA=<% .Config.DeviceDetails.ExtraOTA %>
				STACK_NAME="<% .Config.Name %>"
				STACK_VERSION="<% .Config.Version %>"
				CHROMIUM_BUILD_DISABLED="<% .Config.ChromiumBuildDisabled %>"
				CORE_CONFIG_REPO="<% .Config.CoreConfigRepo %>"
				CORE_CONFIG_REPO_BRANCH="<% .Config.CoreConfigRepoBranch %>"
				CUSTOM_CONFIG_REPO="<% .Config.CustomConfigRepo %>"
				CUSTOM_CONFIG_REPO_BRANCH="<% .Config.CustomConfigRepoBranch %>"`),
			expected: []byte(dedent(`DEVICE="test device"
				DEVICE_FRIENDLY="friendly"
				DEVICE_FAMILY="family"
				DEVICE_AVB_MODE="avb mode"
				DEVICE_EXTRA_OTA=extra ota
				STACK_NAME="test stack"
				STACK_VERSION="test version"
				CHROMIUM_BUILD_DISABLED="false"
				CORE_CONFIG_REPO="core-config-repo"
				CORE_CONFIG_REPO_BRANCH="core-config-repo-branch"
				CUSTOM_CONFIG_REPO="custom-config-repo"
				CUSTOM_CONFIG_REPO_BRANCH="custom-config-repo-branch"`)),
			expectedErr: nil,
		},
		"bad template variable returns error": {
			config:         testConfig,
			lambdaTemplate: dedent(`DEVICE="<% .Bad %>""`),
			expected:       nil,
			expectedErr:    ErrTemplateExecute,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			templateFiles, err := New(tc.config, &TemplateFiles{LambdaTemplate: tc.lambdaTemplate}, "")
			assert.Nil(t, err)

			output, err := templateFiles.renderLambdaFunction()
			assert.ErrorIs(t, err, tc.expectedErr)

			assert.Equal(t, string(tc.expected), string(output))
		})
	}
}

func TestTemplates_RenderTerraform(t *testing.T) {
	tests := map[string]struct {
		config            *Config
		terraformTemplate string
		expected          []byte
		expectedErr       error
	}{
		"happy path build script render": {
			config: testConfig,
			terraformTemplate: dedent(`DEVICE="<% .Config.Device %>"
				DEVICE_FRIENDLY="<% .Config.DeviceDetails.Friendly %>"
				DEVICE_FAMILY="<% .Config.DeviceDetails.Family %>"
				DEVICE_AVB_MODE="<% .Config.DeviceDetails.AVBMode %>"
				DEVICE_EXTRA_OTA=<% .Config.DeviceDetails.ExtraOTA %>
				STACK_NAME="<% .Config.Name %>"
				STACK_VERSION="<% .Config.Version %>"
				CHROMIUM_BUILD_DISABLED="<% .Config.ChromiumBuildDisabled %>"
				CORE_CONFIG_REPO="<% .Config.CoreConfigRepo %>"
				CORE_CONFIG_REPO_BRANCH="<% .Config.CoreConfigRepoBranch %>"
				CUSTOM_CONFIG_REPO="<% .Config.CustomConfigRepo %>"
				CUSTOM_CONFIG_REPO_BRANCH="<% .Config.CustomConfigRepoBranch %>"`),
			expected: []byte(dedent(`DEVICE="test device"
				DEVICE_FRIENDLY="friendly"
				DEVICE_FAMILY="family"
				DEVICE_AVB_MODE="avb mode"
				DEVICE_EXTRA_OTA=extra ota
				STACK_NAME="test stack"
				STACK_VERSION="test version"
				CHROMIUM_BUILD_DISABLED="false"
				CORE_CONFIG_REPO="core-config-repo"
				CORE_CONFIG_REPO_BRANCH="core-config-repo-branch"
				CUSTOM_CONFIG_REPO="custom-config-repo"
				CUSTOM_CONFIG_REPO_BRANCH="custom-config-repo-branch"`)),
			expectedErr: nil,
		},
		"bad template variable returns error": {
			config:            testConfig,
			terraformTemplate: dedent(`DEVICE="<% .Bad %>""`),
			expected:          nil,
			expectedErr:       ErrTemplateExecute,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			templateFiles, err := New(tc.config, &TemplateFiles{TerraformTemplate: tc.terraformTemplate}, "")
			assert.Nil(t, err)

			output, err := templateFiles.renderTerraform()
			assert.ErrorIs(t, err, tc.expectedErr)

			assert.Equal(t, string(tc.expected), string(output))
		})
	}
}

var testConfig = &Config{
	Version: "test version",
	Name:    "test stack",
	Region:  "test region",
	Device:  "test device",
	DeviceDetails: &devices.Device{
		Name:     "test device",
		Friendly: "friendly",
		Family:   "family",
		AVBMode:  "avb mode",
		ExtraOTA: "extra ota",
	},
	Email:                  "email",
	InstanceType:           "instance type",
	InstanceRegions:        "region1,region2",
	SkipPrice:              "skip price",
	MaxPrice:               "max price",
	SSHKey:                 "ssh key",
	Schedule:               "schedule",
	ChromiumBuildDisabled:  false,
	ChromiumVersion:        "chromium version",
	CoreConfigRepo:         "core-config-repo",
	CoreConfigRepoBranch:   "core-config-repo-branch",
	CustomConfigRepo:       "custom-config-repo",
	CustomConfigRepoBranch: "custom-config-repo-branch",
	ReleasesURL:            "latest url",
	Cloud:                  "cloud",
}

// source https://github.com/lithammer/dedent
func dedent(text string) string {
	var margin string
	var whitespaceOnly = regexp.MustCompile("(?m)^[ \t]+$")
	var leadingWhitespace = regexp.MustCompile("(?m)(^[ \t]*)(?:[^ \t\n])")

	text = whitespaceOnly.ReplaceAllString(text, "")
	indents := leadingWhitespace.FindAllStringSubmatch(text, -1)

	// Look for the longest leading string of spaces and tabs common to all
	// lines.
	for i, indent := range indents {
		if i == 0 {
			margin = indent[1]
		} else if strings.HasPrefix(indent[1], margin) {
			// Current line more deeply indented than previous winner:
			// no change (previous winner is still on top).
			continue
		} else if strings.HasPrefix(margin, indent[1]) {
			// Current line consistent with and no deeper than previous winner:
			// it's the new winner.
			margin = indent[1]
		} else {
			// Current line and previous winner have no common whitespace:
			// there is no margin.
			margin = ""
			break
		}
	}

	if margin != "" {
		text = regexp.MustCompile("(?m)^"+margin).ReplaceAllString(text, "")
	}
	return text
}
