package cloudaws

var (
	supportedRegions = map[string]Region{}
	regionSortOrder  []string
)

// Region contains details about an AWS region
type Region struct {
	// Name is the name of the region (e.g. us-west-2)
	Name string
	// AMI is the AMI to use in this region
	AMI  string
}

func init() {
	// curl -s https://cloud-images.ubuntu.com/locator/ec2/releasesTable | grep '20.04' | grep 'amd64' | grep 'hvm:ebs-ssd' | awk -F'"' '{print $2, $15}'  | awk -F"launchAmi=" '{print $1,$2}' | awk '{print $1,$3}' | awk -F'\' '{print $1}' | awk '{printf "Region{\"%s\", \"%s\"},\n",$1,$2 }'
	addRegions(
		Region{"af-south-1", "ami-0f072aafc9dfcb24f"},
		Region{"ap-east-1", "ami-04864d873127e4b0a"},
		Region{"ap-northeast-1", "ami-0e039c7d64008bd84"},
		Region{"ap-northeast-2", "ami-067abcae434ee508b"},
		Region{"ap-northeast-3", "ami-08dfee60cf1895207"},
		Region{"ap-south-1", "ami-073c8c0760395aab8"},
		Region{"ap-southeast-1", "ami-09a6a7e49bd29554b"},
		Region{"ap-southeast-2", "ami-0d767dd04ac152743"},
		Region{"ca-central-1", "ami-0df58bd52157c6e83"},
		Region{"eu-central-1", "ami-0932440befd74cdba"},
		Region{"eu-north-1", "ami-09b44b5f46219ee86"},
		Region{"eu-south-1", "ami-0e0812e2467b24796"},
		Region{"eu-west-1", "ami-022e8cc8f0d3c52fd"},
		Region{"eu-west-2", "ami-005383956f2e5fb96"},
		Region{"eu-west-3", "ami-00f6fe7d6cbb56a78"},
		Region{"me-south-1", "ami-07bf297712e054a41"},
		Region{"sa-east-1", "ami-0e765cee959bcbfce"},
		Region{"us-east-1", "ami-03d315ad33b9d49c4"},
		Region{"us-east-2", "ami-0996d3051b72b5b2c"},
		Region{"us-west-1", "ami-0ebef2838fb2605b7"},
		Region{"us-west-2", "ami-0928f4202481dfdf6"},
		Region{"cn-north-1", "ami-0592ccadb56e65f8d"},
		Region{"cn-northwest-1", "ami-007d0f254ea0f8588"},
		Region{"us-gov-west-1", "ami-a7edd7c6"},
		Region{"us-gov-east-1", "ami-c39973b2"},
	)
}

// GetSupportedRegions returns a list of all supported regions
func GetSupportedRegions() []string {
	return regionSortOrder
}

// IsSupportedRegion returns whether a specified region is supported
func IsSupportedRegion(region string) bool {
	if _, ok := supportedRegions[region]; !ok {
		return false
	}
	return true
}

// GetAMIs returns a region to AMI mapping for all supported regions
func GetAMIs() map[string]string {
	amis := map[string]string{}
	for _, region := range regionSortOrder {
		amis[region] = supportedRegions[region].AMI
	}
	return amis
}

func addRegions(regions ...Region) {
	for _, region := range regions {
		supportedRegions[region.Name] = region
		regionSortOrder = append(regionSortOrder, region.Name)
	}
}
