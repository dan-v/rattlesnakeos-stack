package templates

const LambdaTemplate = `
#!/usr/bin/env python3
import boto3
import base64
import json
from urllib.request import urlopen
from urllib.request import HTTPError
from datetime import datetime, timedelta

# ubuntu 18.04 AMI hvm:ebs-ssd: https://cloud-images.ubuntu.com/locator/ec2/
REGION_AMIS = {
    "us-east-1": "ami-0ac019f4fcb7cb7e6",
    "us-west-1": "ami-063aa838bd7631e0b",
    "ap-northeast-3": "ami-05e896b95030bd37c",
    "ap-northeast-2": "ami-06e7b9c5e0c4dd014",
    "ap-northeast-1": "ami-07ad4b1c3af1ea214",
    "sa-east-1": "ami-03c6239555bb12112",
    "ap-southeast-1": "ami-0c5199d385b432989",
    "ca-central-1": "ami-0427e8367e3770df1",
    "ap-southeast-2": "ami-07a3bd4944eb120a0",
    "us-west-2": "ami-0bbe6b35405ecebdb",
    "us-east-2": "ami-0f65671a86f061fcd",
    "ap-south-1": "ami-0d773a3b7bb2bb1c1",
    "eu-central-1": "ami-0bdf93799014acdc4",
    "eu-west-1": "ami-00035f41c82244dab",
    "eu-west-2": "ami-0b0a60c0a2bd40612",
    "eu-west-3": "ami-08182c55a1c188dee",
    "cn-north-1": "ami-05596fb52c3802012",
    "cn-northwest-1": "ami-03f7db8b059795736",
    "us-gov-west-1": "ami-33dc4252"
}

NAME = '<% .Name %>'
SRC_PATH = 's3://<% .Name %>-script/build.sh'
FLEET_ROLE = 'arn:aws:iam::{0}:role/<% .Name %>-spot-fleet-role'
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
- [ bash, -c, "sudo -u ubuntu bash /home/ubuntu/build.sh {1} {2}" ]
    """.format(SRC_PATH, DEVICE, str(force_build).lower()).encode('ascii')).decode('ascii')

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
                            'VolumeSize': 200,
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
`
