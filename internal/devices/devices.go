package devices

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// AVBModeChained is chained AVB mode in build script
	AVBModeChained                    = "vbmeta_chained"
	// AVBModeChainedV2 is chained AVB v2 mode in build script
	AVBModeChainedV2                  = "vbmeta_chained_v2"
	// ExtraOTARetrofitDynamicPartitions is additional OTA option to retrofit dynamics partitions
	ExtraOTARetrofitDynamicPartitions = "(--retrofit_dynamic_partitions)"
)

var (
	// ErrMissingName is returned if device is missing name
	ErrMissingName     = errors.New("supported device is missing required name")
	// ErrMissingFriendly is returned if friendly name for device is missing
	ErrMissingFriendly = errors.New("supported device is missing required friendly name")
	// ErrMissingFamily is returned if family name for device is missing
	ErrMissingFamily   = errors.New("supported device is missing required family name")
	// ErrMissingAVBMode is returned if avb mode is missing for device
	ErrMissingAVBMode  = errors.New("supported device is missing required avb mode")
)

// Device contains details and metadata about a device
type Device struct {
	Name     string
	Friendly string
	Family   string
	AVBMode  string
	ExtraOTA string
}

// SupportedDevices contains all the supported devices, their details, and sort order
type SupportedDevices struct {
	supportedDevices map[string]*Device
	deviceSortOrder  []string
}

// NewSupportedDevices takes in all devices, validates them, and returns initialized SupportedDevices
// which contains helper functions to get details about the supported devices
func NewSupportedDevices(devices ...*Device) (*SupportedDevices, error) {
	var deviceSortOrder []string
	supportedDevices := map[string]*Device{}
	for _, device := range devices {
		if device.Name == "" {
			return nil, ErrMissingName
		}
		if device.Friendly == "" {
			return nil, fmt.Errorf("'%v': %w", device.Name, ErrMissingFriendly)
		}
		if device.Family == "" {
			return nil, fmt.Errorf("'%v': %w", device.Name, ErrMissingFamily)
		}
		if device.AVBMode == "" {
			return nil, fmt.Errorf("'%v': %w", device.Name, ErrMissingAVBMode)
		}
		deviceSortOrder = append(deviceSortOrder, device.Name)
		supportedDevices[device.Name] = device
	}

	return &SupportedDevices{
		supportedDevices: supportedDevices,
		deviceSortOrder:  deviceSortOrder,
	}, nil
}

// IsSupportedDevice takes device name (e.g. redfin) and returns boolean support value
func (s *SupportedDevices) IsSupportedDevice(device string) bool {
	if _, ok := s.supportedDevices[device]; !ok {
		return false
	}
	return true
}

// GetDeviceDetails takes device name (e.g. redfin) and returns full Device details
func (s *SupportedDevices) GetDeviceDetails(device string) *Device {
	if _, ok := s.supportedDevices[device]; !ok {
		return nil
	}
	return s.supportedDevices[device]
}

// GetDeviceFriendlyNames returns list of all supported device friendly names (e.g. Pixel 4a)
func (s *SupportedDevices) GetDeviceFriendlyNames() []string {
	var output []string
	for _, device := range s.deviceSortOrder {
		output = append(output, s.supportedDevices[device].Friendly)
	}
	return output
}

// GetDeviceCodeNames returns list of all supported devices code names (e.g. redfin)
func (s *SupportedDevices) GetDeviceCodeNames() []string {
	return s.deviceSortOrder
}

// GetSupportedDevicesOutput returns a nicely formatted comma separated list of codename (friendly name)
func (s *SupportedDevices) GetSupportedDevicesOutput() string {
	var supportDevicesOutput []string
	supportedDevicesFriendly := s.GetDeviceFriendlyNames()
	for i, d := range s.deviceSortOrder {
		supportDevicesOutput = append(supportDevicesOutput, fmt.Sprintf("%v (%v)", d, supportedDevicesFriendly[i]))
	}
	return strings.Join(supportDevicesOutput, ", ")
}
