package devices

const (
	blueline         = "blueline"
	crosshatch       = "crosshatch"
	sargo            = "sargo"
	bonito           = "bonito"
	flame            = "flame"
	coral            = "coral"
	sunfish          = "sunfish"
	redfin           = "redfin"
	avbModeChained   = "vbmeta_chained"
	avbModeChainedV2 = "vbmeta_chained_v2"
)

var (
	supportedDevices = map[string]Device{}
	deviceSortOrder  []string
)

type Device struct {
	Name     string
	Friendly string
	Family   string
	Common   string
	AVBMode  string
	ExtraOTA string
}

func init() {
	addDevices(
		Device{
			Name:     blueline,
			Friendly: "Pixel 3",
			Family:   "crosshatch",
			Common:   "crosshatch",
			AVBMode:  avbModeChained,
			ExtraOTA: "(--retrofit_dynamic_partitions)",
		},
		Device{
			Name:     crosshatch,
			Friendly: "Pixel 3 XL",
			Family:   "crosshatch",
			Common:   "crosshatch",
			AVBMode:  avbModeChained,
			ExtraOTA: "(--retrofit_dynamic_partitions)",
		},
		Device{
			Name:     sargo,
			Friendly: "Pixel 3a",
			Family:   "bonito",
			Common:   "bonito",
			AVBMode:  avbModeChained,
			ExtraOTA: "(--retrofit_dynamic_partitions)",
		},
		Device{
			Name:     bonito,
			Friendly: "Pixel 3a XL",
			Family:   "bonito",
			Common:   "bonito",
			AVBMode:  avbModeChained,
			ExtraOTA: "(--retrofit_dynamic_partitions)",
		},
		Device{
			Name:     flame,
			Friendly: "Pixel 4",
			Family:   "coral",
			Common:   "coral",
			AVBMode:  avbModeChainedV2,
		},
		Device{
			Name:     coral,
			Friendly: "Pixel 4 XL",
			Family:   "coral",
			Common:   "coral",
			AVBMode:  avbModeChainedV2,
		},
		Device{
			Name:     sunfish,
			Friendly: "Pixel 4a",
			Family:   "sunfish",
			Common:   "sunfish",
			AVBMode:  avbModeChainedV2,
		},
		Device{
			Name:     redfin,
			Friendly: "Pixel 5",
			Family:   "redfin",
			Common:   "redfin",
			AVBMode:  avbModeChainedV2,
		},
	)
}

func addDevices(devices ...Device) {
	for _, device := range devices {
		supportedDevices[device.Name] = device
		deviceSortOrder = append(deviceSortOrder, device.Name)
	}
}

// IsSupportedDevice takes device name (e.g. redfin) and returns boolean support value
func IsSupportedDevice(device string) bool {
	if _, ok := supportedDevices[device]; !ok {
		return false
	}
	return true
}

// GetDeviceDetails takes device name (e.g. redfin) and returns full Device details
func GetDeviceDetails(device string) Device {
	return supportedDevices[device]
}

// GetDeviceFriendlyNames returns list of all supported device friendly names (e.g. Pixel 4a)
func GetDeviceFriendlyNames() []string {
	var output []string
	for _, device := range deviceSortOrder {
		output = append(output, supportedDevices[device].Friendly)
	}
	return output
}

// GetDeviceCodeNames returns list of all supported devices code names (e.g. redfin)
func GetDeviceCodeNames() []string {
	return deviceSortOrder
}
