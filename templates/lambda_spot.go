package templates

const LambdaSpotFunctionTemplate = `
#!/usr/bin/env python3
import boto3
import base64
from urllib.request import urlopen
from urllib.request import HTTPError
from datetime import datetime, timedelta

SRC_PATH = 's3://<% .Name %>-script/build.sh'

FLEET_ROLE = 'arn:aws:iam::{0}:role/<% .Name %>-spot-fleet-role'
IAM_PROFILE = 'arn:aws:iam::{0}:instance-profile/<% .Name %>-ec2'
DEVICE = '<% .Device %>'
AMI_ID = '<% .AMI %>'
SSH_KEY_NAME = '<% .SSHKey %>'
SPOT_PRICE = '<% .SpotPrice %>'

def lambda_handler(event, context):
    client = boto3.client('ec2')

    # get all subnets (for some reason spot request is blowing up with an unhelpful error message without this)
    subnets = ",".join([sn['SubnetId'] for sn in client.describe_subnets()['Subnets']])

    # get account id to fill in fleet role and ec2 profile
    account_id = boto3.client('sts').get_caller_identity().get('Account')

    print("spinning up {0} release".format(DEVICE))

    userdata = base64.b64encode("""
#cloud-config
output : {{ all : '| tee -a /var/log/cloud-init-output.log' }}

repo_update: true
repo_upgrade: all
packages:
- awscli

runcmd:
- [ bash, -c, "sudo -u ubuntu aws s3 cp {0} /home/ubuntu/build.sh" ]
- [ bash, -c, "sudo -u ubuntu bash /home/ubuntu/build.sh {1} -A" ]
    """.format(SRC_PATH, DEVICE).encode('ascii')).decode('ascii')

    now_utc = datetime.utcnow().replace(microsecond=0)
    valid_until = now_utc + timedelta(hours=12)
    response = client.request_spot_fleet(
        SpotFleetRequestConfig={
            'IamFleetRole': FLEET_ROLE.format(account_id),
            'AllocationStrategy': 'lowestPrice',
            'TargetCapacity': 1,
            'SpotPrice': SPOT_PRICE,
            'ValidFrom': now_utc,
            'ValidUntil': valid_until,
            'TerminateInstancesWithExpiration': True,
            'LaunchSpecifications': [
                {
                    'ImageId': AMI_ID,
                    'SubnetId': subnets,
                    'InstanceType': 'c4.4xlarge',
                    <% if .SSHKey %>'KeyName': SSH_KEY_NAME,<% end %>
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
        },
    )
    print(response)

if __name__ == '__main__':
   lambda_handler("", "")
`
