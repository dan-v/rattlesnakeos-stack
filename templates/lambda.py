#!/usr/bin/env python3
import boto3
import base64
import json
from urllib.request import urlopen
from urllib.request import HTTPError
from datetime import datetime, timedelta

# curl -s https://cloud-images.ubuntu.com/locator/ec2/releasesTable | grep '18.04' | grep 'amd64' | grep 'hvm:ebs-ssd' | awk -F'"' '{print $2, $15}'  | awk -F"launchAmi=" '{print $1,$2}' | awk '{print $1,$3}' | awk -F'\' '{print $1}' | awk '{printf "\"%s\": \"%s\",\n",$1,$2 }'
# ubuntu 18.04 AMI hvm:ebs-ssd: https://cloud-images.ubuntu.com/locator/ec2/
REGION_AMIS = {
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

NAME = '<% .Name %>'
SRC_PATH = 's3://<% .Name %>-script/build.sh'
FLEET_ROLE = 'arn:aws:iam::{0}:role/aws-service-role/spotfleet.amazonaws.com/AWSServiceRoleForEC2SpotFleet'
IAM_PROFILE = 'arn:aws:iam::{0}:instance-profile/<% .Name %>-ec2'
SNS_ARN = 'arn:aws:sns:<% .Region %>:{}:<% .Name %>'
INSTANCE_TYPE = '<% .InstanceType %>'
DEVICE = '<% .Device %>'
SSH_KEY_NAME = '<% .SSHKey %>'
MAX_PRICE = '<% .MaxPrice %>'
SKIP_PRICE = '<% .SkipPrice %>'
REGIONS = '<% .InstanceRegions %>'
AMI_OVERRIDE = '<% .AMI %>'
ENCRYPTED_KEYS = '<% .EncryptedKeys %>'

def send_sns_message(subject, message):
    account_id = boto3.client('sts').get_caller_identity().get('Account')
    sns = boto3.client('sns')
    resp = sns.publish(TopicArn=SNS_ARN.format(account_id), Subject=subject, Message=message)
    print("Sent SNS message {} and got response: {}".format(message, resp))

def lambda_handler(event, context):
    # get account id to fill in fleet role and ec2 profile
    account_id = boto3.client('sts').get_caller_identity().get('Account')

    force_build = False
    if "ForceBuild" in event:
        force_build = event['ForceBuild']
    aosp_build = ""
    if "AOSPBuild" in event:
        aosp_build = event['AOSPBuild']
    aosp_branch = ""
    if "AOSPBranch" in event:
        aosp_branch = event['AOSPBranch']

    try:
        cheapest_price = 0
        cheapest_region = ""
        cheapest_az = ""
        for region in REGIONS.split(","):
            ec2_client = boto3.client('ec2', region_name=region)
            spot_price_dict = ec2_client.describe_spot_price_history(
                        StartTime=datetime.now() - timedelta(minutes=1),
                        EndTime=datetime.now(),
                        InstanceTypes=[
                            INSTANCE_TYPE
                        ],
                        ProductDescriptions=[
                            'Linux/UNIX (Amazon VPC)'
                        ],
                    )
            for key, value in spot_price_dict.items():
                if key == u'SpotPriceHistory':
                    for i in value:
                        az = i[u'AvailabilityZone']
                        price = i[u'SpotPrice']
                        if cheapest_price == 0:
                            cheapest_price = price
                            cheapest_region = region
                            cheapest_az = az
                        else:
                            if price < cheapest_price:
                                cheapest_price = price
                                cheapest_region = region
                                cheapest_az = az
                        print("{} {}".format(az, price))
    except Exception as e:
        send_sns_message("RattlesnakeOS Spot Instance FAILED", "There was a problem finding cheapest region for spot instance {}: {}".format(INSTANCE_TYPE, e))
        raise

    if float(cheapest_price) > float(SKIP_PRICE):
        message = "Cheapest spot instance {} price ${} in AZ {} is not lower than --skip-price ${}.".format(INSTANCE_TYPE, cheapest_price,
                      cheapest_az, SKIP_PRICE)
        send_sns_message("RattlesnakeOS Spot Instance SKIPPED", message)
        return message

    # AMI to launch with
    ami = REGION_AMIS[cheapest_region]
    if AMI_OVERRIDE:
        ami = AMI_OVERRIDE

    # create ec2 client for cheapest region
    client = boto3.client('ec2', region_name=cheapest_region)

    # get a subnet in cheapest az to request spot instance in
    subnets = client.describe_subnets(Filters=[{'Name': 'availabilityZone','Values': [cheapest_az]}])['Subnets'][0]['SubnetId']

    # userdata to deploy with spot instance
    userdata = base64.b64encode("""
#cloud-config
output : {{ all : '| tee -a /var/log/cloud-init-output.log' }}

repo_update: true
repo_upgrade: all
packages:
- awscli

runcmd:
- [ bash, -c, "sudo -u ubuntu aws s3 --region <% .Region %> cp {0} /home/ubuntu/build.sh" ]
- [ bash, -c, "sudo -u ubuntu bash /home/ubuntu/build.sh {1} {2} {3} {4}" ]
    """.format(SRC_PATH, DEVICE, str(force_build).lower(), aosp_build, aosp_branch).encode('ascii')).decode('ascii')

    # make spot fleet request config
    now_utc = datetime.utcnow().replace(microsecond=0)
    valid_until = now_utc + timedelta(hours=12)
    spot_fleet_request_config = {
        'IamFleetRole': FLEET_ROLE.format(account_id),
        'AllocationStrategy': 'lowestPrice',
        'TargetCapacity': 1,
        'SpotPrice': MAX_PRICE,
        'ValidFrom': now_utc,
        'ValidUntil': valid_until,
        'TerminateInstancesWithExpiration': True,
        'LaunchSpecifications': [
            {
                'ImageId': ami,
                'SubnetId': subnets,
                'InstanceType': INSTANCE_TYPE,
                'IamInstanceProfile': {
                    'Arn': IAM_PROFILE.format(account_id)
                },
                'BlockDeviceMappings': [
                    {
                        'DeviceName' : '/dev/sda1',
                        'Ebs': {
                            'DeleteOnTermination': True,
                            'VolumeSize': 250,
                            'VolumeType': 'gp2'
                        },
                    },
                ],
                'UserData': userdata
            },
        ],
        'Type': 'request'
    }

    # check if ec2 keypair exists in this region - otherwise don't include keypair in spot request
    try:
        client.describe_key_pairs(KeyNames=[SSH_KEY_NAME])
        spot_fleet_request_config['LaunchSpecifications'][0]['KeyName'] = SSH_KEY_NAME
    except Exception as e:
        if ENCRYPTED_KEYS == "true":
            message = "Encrypted keys is enabled, so properly configured SSH keys are mandatory. Unable to find an EC2 Key Pair named '{}' in region {}.".format(SSH_KEY_NAME, cheapest_region)
            send_sns_message("RattlesnakeOS Spot Instance CONFIGURATION ERROR", message)
            return message
        else:
            print("Not including SSH key in spot request as couldn't find a key in region {} with name {}: {}".format(cheapest_region, SSH_KEY_NAME, e))

    print("spot_fleet_request_config: ", spot_fleet_request_config)

    try:
        print("Requesting spot instance in AZ {} with current price of {}".format(cheapest_az, cheapest_price))
        response = client.request_spot_fleet(SpotFleetRequestConfig=spot_fleet_request_config)
        print("Spot request response: {}".format(response))
    except Exception as e:
        send_sns_message("RattlesnakeOS Spot Instance FAILED", "There was a problem requesting a spot instance {}: {}".format(INSTANCE_TYPE, e))
        raise

    subject = "RattlesnakeOS Spot Instance SUCCESS"
    message = "Successfully requested a spot instance.\n\n Stack Name: {}\n Device: {}\n Force Build: {}\n Instance Type: {}\n Cheapest Region: {}\n Cheapest Hourly Price: ${} ".format(NAME, DEVICE, force_build, INSTANCE_TYPE, cheapest_region, cheapest_price)
    send_sns_message(subject, message)
    return message.replace('\n', ' ')

if __name__ == '__main__':
   lambda_handler("", "")