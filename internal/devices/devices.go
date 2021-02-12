package devices

const (
	Blueline         = "blueline"
	Crosshatch       = "crosshatch"
	Sargo            = "sargo"
	Bonito           = "bonito"
	Flame            = "flame"
	Coral            = "coral"
	Sunfish          = "sunfish"
	Redfin           = "redfin"
	AVBModeSimple    = "vbmeta_simple"
	AVBModeChained   = "vbmeta_chained"
	AVBModeChainedV2 = "vbmeta_chained_v2"
)

var deviceSortOrder = []string{
	Blueline, Crosshatch, Sargo, Bonito, Flame, Coral, Sunfish, Redfin,
}

func IsSupportedDevice(device string) bool {
	if _, ok := SupportedDevices[device]; !ok {
		return false
	}
	return true
}

func GetDeviceDetails(device string) Device {
	return SupportedDevices[device]
}

var SupportedDevices = DeviceMap{
	Blueline: {
		Name:     Blueline,
		Friendly: "Pixel 3",
		Family:   "crosshatch",
		Common:   "crosshatch",
		AVBMode:  AVBModeChained,
		ExtraOTA: "(--retrofit_dynamic_partitions)",
	},
	Crosshatch: {
		Name:     Crosshatch,
		Friendly: "Pixel 3 XL",
		Family:   "crosshatch",
		Common:   "crosshatch",
		AVBMode:  AVBModeChained,
		ExtraOTA: "(--retrofit_dynamic_partitions)",
	},
	Sargo: {
		Name:     Sargo,
		Friendly: "Pixel 3a",
		Family:   "bonito",
		Common:   "bonito",
		AVBMode:  AVBModeChained,
		ExtraOTA: "(--retrofit_dynamic_partitions)",
	},
	Bonito: {
		Name:     Bonito,
		Friendly: "Pixel 3a XL",
		Family:   "bonito",
		Common:   "bonito",
		AVBMode:  AVBModeChained,
		ExtraOTA: "(--retrofit_dynamic_partitions)",
	},
	Flame: {
		Name:     Flame,
		Friendly: "Pixel 4",
		Family:   "coral",
		Common:   "coral",
		AVBMode:  AVBModeChainedV2,
	},
	Coral: {
		Name:     Coral,
		Friendly: "Pixel 4 XL",
		Family:   "coral",
		Common:   "coral",
		AVBMode:  AVBModeChainedV2,
	},
	Sunfish: {
		Name:     Sunfish,
		Friendly: "Pixel 4a",
		Family:   "sunfish",
		Common:   "sunfish",
		AVBMode:  AVBModeChainedV2,
	},
	Redfin: {
		Name:     Redfin,
		Friendly: "Pixel 5",
		Family:   "redfin",
		Common:   "redfin",
		AVBMode:  AVBModeChainedV2,
	},
}

type Device struct {
	Name     string
	Friendly string
	Family   string
	Common   string
	AVBMode  string
	ExtraOTA string
}

type DeviceMap map[string]Device

func (d DeviceMap) GetDeviceCodeNames() []string {
	return deviceSortOrder
}

func (d DeviceMap) GetDeviceFriendlyNames() []string {
	var output []string
	for _, device := range deviceSortOrder {
		output = append(output, d[device].Friendly)
	}
	return output
}
