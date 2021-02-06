package aws

var DefaultInstanceRegions = "us-west-2,us-west-1,us-east-2"

var RegionAMIs = map[string]string{
	"af-south-1": "ami-0f072aafc9dfcb24f",
	"ap-east-1": "ami-04864d873127e4b0a",
	"ap-northeast-1": "ami-0e039c7d64008bd84",
	"ap-northeast-2": "ami-067abcae434ee508b",
	"ap-northeast-3": "ami-08dfee60cf1895207",
	"ap-south-1": "ami-073c8c0760395aab8",
	"ap-southeast-1": "ami-09a6a7e49bd29554b",
	"ap-southeast-2": "ami-0d767dd04ac152743",
	"ca-central-1": "ami-0df58bd52157c6e83",
	"eu-central-1": "ami-0932440befd74cdba",
	"eu-north-1": "ami-09b44b5f46219ee86",
	"eu-south-1": "ami-0e0812e2467b24796",
	"eu-west-1": "ami-022e8cc8f0d3c52fd",
	"eu-west-2": "ami-005383956f2e5fb96",
	"eu-west-3": "ami-00f6fe7d6cbb56a78",
	"me-south-1": "ami-07bf297712e054a41",
	"sa-east-1": "ami-0e765cee959bcbfce",
	"us-east-1": "ami-03d315ad33b9d49c4",
	"us-east-2": "ami-0996d3051b72b5b2c",
	"us-west-1": "ami-0ebef2838fb2605b7",
	"us-west-2": "ami-0928f4202481dfdf6",
	"cn-north-1": "ami-0592ccadb56e65f8d",
	"cn-northwest-1": "ami-007d0f254ea0f8588",
	"us-gov-west-1": "ami-a7edd7c6",
	"us-gov-east-1": "ami-c39973b2",
}

func SupportedRegions() []string {
	var supportedRegions []string
	for r := range RegionAMIs {
		supportedRegions = append(supportedRegions, r)
	}
	return supportedRegions
}

func IsSupportedRegion(region string) bool {
	if _, ok := RegionAMIs[region]; !ok {
		return false
	}
	return true
}
