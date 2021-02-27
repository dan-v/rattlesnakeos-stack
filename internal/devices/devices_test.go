package devices

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSupportedDevices_IsSupportedDevice(t *testing.T) {
	tests := map[string]struct {
		devices     []*Device
		input       string
		expected    bool
		expectedErr error
	}{
		"devices that is supported returns true": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
			},
			input:       "name",
			expected:    true,
			expectedErr: nil,
		},
		"devices that is not supported returns false": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
			},
			input:       "unsupported",
			expected:    false,
			expectedErr: nil,
		},
		"if no supported devices returns false": {
			devices:     []*Device{},
			input:       "empty",
			expected:    false,
			expectedErr: nil,
		},
		"missing name returns error": {
			devices: []*Device{
				&Device{
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
			},
			input:       "name",
			expected:    false,
			expectedErr: ErrMissingName,
		},
		"missing friendly name returns error": {
			devices: []*Device{
				&Device{
					Name:    "name",
					Family:  "family",
					AVBMode: AVBModeChainedV2,
				},
			},
			input:       "name",
			expected:    false,
			expectedErr: ErrMissingFriendly,
		},
		"missing family returns error": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					AVBMode:  AVBModeChainedV2,
				},
			},
			input:       "name",
			expected:    false,
			expectedErr: ErrMissingFamily,
		},
		"missing avb mode returns error": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
				},
			},
			input:       "name",
			expected:    false,
			expectedErr: ErrMissingAVBMode,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			supportedDevices, err := NewSupportedDevices(tc.devices...)
			assert.ErrorIs(t, err, tc.expectedErr)
			if err == nil {
				assert.Equal(t, tc.expected, supportedDevices.IsSupportedDevice(tc.input))
			}
		})
	}
}

func TestSupportedDevices_GetDeviceDetails(t *testing.T) {
	tests := map[string]struct {
		devices  []*Device
		input    string
		expected *Device
	}{
		"valid device returns with details": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
			},
			input: "name",
			expected: &Device{
				Name:     "name",
				Friendly: "friendly",
				Family:   "family",
				AVBMode:  AVBModeChainedV2,
			},
		},
		"non existing device returns nil": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
			},
			input:    "non existing",
			expected: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			supportedDevices, err := NewSupportedDevices(tc.devices...)
			assert.Nil(t, err)

			device := supportedDevices.GetDeviceDetails(tc.input)
			assert.Equal(t, tc.expected, device)
		})
	}
}

func TestSupportedDevices_GetDeviceFriendlyNames(t *testing.T) {
	tests := map[string]struct {
		devices  []*Device
		expected []string
	}{
		"returns friendly names": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
				&Device{
					Name:     "name 2",
					Friendly: "friendly 2",
					Family:   "family 2",
					AVBMode:  AVBModeChainedV2,
				},
			},
			expected: []string{"friendly", "friendly 2"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			supportedDevices, err := NewSupportedDevices(tc.devices...)
			assert.Nil(t, err)

			device := supportedDevices.GetDeviceFriendlyNames()
			assert.Equal(t, tc.expected, device)
		})
	}
}

func TestSupportedDevices_GetDeviceCodeNames(t *testing.T) {
	tests := map[string]struct {
		devices  []*Device
		expected []string
	}{
		"returns friendly names": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
				&Device{
					Name:     "name 2",
					Friendly: "friendly 2",
					Family:   "family 2",
					AVBMode:  AVBModeChainedV2,
				},
			},
			expected: []string{"name", "name 2"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			supportedDevices, err := NewSupportedDevices(tc.devices...)
			assert.Nil(t, err)

			device := supportedDevices.GetDeviceCodeNames()
			assert.Equal(t, tc.expected, device)
		})
	}
}

func TestSupportedDevices_GetSupportedDevicesOutput(t *testing.T) {
	tests := map[string]struct {
		devices  []*Device
		expected string
	}{
		"returns expected output": {
			devices: []*Device{
				&Device{
					Name:     "name",
					Friendly: "friendly",
					Family:   "family",
					AVBMode:  AVBModeChainedV2,
				},
				&Device{
					Name:     "name 2",
					Friendly: "friendly 2",
					Family:   "family 2",
					AVBMode:  AVBModeChainedV2,
				},
			},
			expected: "name (friendly), name 2 (friendly 2)",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			supportedDevices, err := NewSupportedDevices(tc.devices...)
			assert.Nil(t, err)

			output := supportedDevices.GetSupportedDevicesOutput()
			assert.Equal(t, tc.expected, output)
		})
	}
}
