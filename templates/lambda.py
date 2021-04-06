#!/usr/bin/env python3
import boto3
import base64
import json
import time
from urllib.request import urlopen
from datetime import datetime, timedelta
from pkg_resources import packaging

STACK_VERSION = '<% .Config.Version %>'
NAME = '<% .Config.Name %>'
LATEST_JSON_URL = "<% .Config.ReleasesURL %>"
STACK_VERSION_LATEST_URL = "<% .RattlesnakeOSStackReleasesURL %>"
BUILD_SCRIPT_S3_LOCATION = 's3://<% .Config.Name %>-script/build.sh'
RELEASE_BUCKET = '<% .Config.Name %>-release'
FLEET_ROLE = 'arn:aws:iam::{0}:role/aws-service-role/spotfleet.amazonaws.com/AWSServiceRoleForEC2SpotFleet'
IAM_PROFILE = 'arn:aws:iam::{0}:instance-profile/<% .Config.Name %>-ec2'
SNS_ARN = 'arn:aws:sns:<% .Config.Region %>:{}:<% .Config.Name %>'
INSTANCE_TYPE = '<% .Config.InstanceType %>'
DEVICE = '<% .Config.Device %>'
SSH_KEY_NAME = '<% .Config.SSHKey %>'
MAX_PRICE = '<% .Config.MaxPrice %>'
SKIP_PRICE = '<% .Config.SkipPrice %>'
STACK_REGION = '<% .Config.Region %>'
INSTANCE_REGIONS = '<% .Config.InstanceRegions %>'
REGION_AMIS = json.loads('<% .RegionAMIs %>')
CHROMIUM_BUILD_DISABLED = '<% .Config.ChromiumBuildDisabled %>'
CHROMIUM_PINNED_VERSION = '<% .Config.ChromiumVersion %>'


def lambda_handler(event, context):
    # get latest
    latest_stack_json = json.loads(urlopen(STACK_VERSION_LATEST_URL).read().decode())
    latest_stack_version = latest_stack_json.get('name')
    print("latest_stack_version", latest_stack_version)
    latest_json = json.loads(urlopen(LATEST_JSON_URL).read().decode())
    latest_release = latest_json.get('release')
    print("latest_release", latest_release)
    latest_chromium_version = latest_json.get('chromium')
    print("latest_chromium_version", latest_chromium_version)
    latest_aosp_build_id = latest_json.get('devices').get(DEVICE).get('build_id')
    print("latest_aosp_build_id", latest_aosp_build_id)
    latest_aosp_tag = latest_json.get('devices').get(DEVICE).get('aosp_tag')
    print("latest_aosp_tag", latest_aosp_tag)
    minimum_stack_version = latest_json.get('minimum_stack_version')
    print("minimum_stack_version", minimum_stack_version)

    # only build if minimum stack version requirement is met
    if packaging.version.parse(STACK_VERSION) < packaging.version.parse(minimum_stack_version):
        message = "RattlesnakeOS build was cancelled. Existing stack version {} needs to be updated to latest".format(STACK_VERSION)
        send_sns_message("RattlesnakeOS Stack Needs Update", message)
        return message

    # gather revisions for passing to build script
    revisions = []
    for k, v in latest_json.get('revisions').items():
        revisions.append("{}={}".format(k, v))
    revisions_string = (",".join(revisions))
    print("revisions_string", revisions_string)

    # build time overrides
    force_build = event.get('force-build') or False
    print("force_build", force_build)
    force_chromium_build_string = "true" if event.get('force-chromium-build') else "false"
    print("force_chromium_build_string", force_chromium_build_string)
    if force_chromium_build_string == "true":
        force_build = True
    aosp_build_id = event.get('aosp-build-id') or latest_aosp_build_id
    print("aosp_build_id", aosp_build_id)
    aosp_tag = event.get('aosp-tag') or latest_aosp_tag
    print("aosp_tag", aosp_tag)
    chromium_version = event.get('ChromiumVersion') or CHROMIUM_PINNED_VERSION if CHROMIUM_PINNED_VERSION != "" else latest_chromium_version
    print("chromium_version", chromium_version)

    # check if build is required
    needs_build, build_reason = is_build_required(latest_release)
    if not needs_build and not force_build:
        message = "RattlesnakeOS build is already up to date."
        send_sns_message("RattlesnakeOS Build Not Required", message)
        return message
    if not needs_build and force_build:
        build_reason = "Build not required - but force build flag was specified."
    print("needs_build", needs_build)
    print("build_reason", build_reason)

    # find region and az with cheapest price
    try:
        cheapest_price, cheapest_region, cheapest_az = find_cheapest_region()
        if float(cheapest_price) > float(SKIP_PRICE):
            message = f"Cheapest spot instance {INSTANCE_TYPE} price ${cheapest_price} in AZ {cheapest_az} is not lower than --skip-price ${SKIP_PRICE}."
            send_sns_message("RattlesnakeOS Spot Instance SKIPPED", message)
            return message
    except Exception as e:
        message = f"There was a problem finding cheapest region for spot instance {INSTANCE_TYPE}: {e}"
        send_sns_message("RattlesnakeOS Spot Instance FAILED", message)
        raise

    # AMI to launch with
    ami = REGION_AMIS[cheapest_region]

    # create ec2 client for cheapest region
    client = boto3.client('ec2', region_name=cheapest_region)

    # get a subnet in cheapest az to request spot instance in
    subnets = client.describe_subnets(Filters=[{'Name': 'availabilityZone', 'Values': [cheapest_az]}])['Subnets'][0][
        'SubnetId']

    # userdata to deploy with spot instance
    copy_build_command = f"sudo -u ubuntu aws s3 --region {STACK_REGION} cp {BUILD_SCRIPT_S3_LOCATION} /home/ubuntu/build.sh"
    build_args_command = f"echo \\\"/home/ubuntu/build.sh {latest_release} {aosp_build_id} {aosp_tag} {chromium_version} {force_chromium_build_string} {revisions_string}\\\" > /home/ubuntu/build_cmd"
    build_start_command = f"sudo -u ubuntu bash /home/ubuntu/build.sh \\\"{latest_release}\\\" \\\"{aosp_build_id}\\\" \\\"{aosp_tag}\\\" \\\"{chromium_version}\\\" \\\"{force_chromium_build_string}\\\" \\\"{revisions_string}\\\""
    userdata = base64.b64encode(f"""
#cloud-config
output : {{ all : '| tee -a /var/log/cloud-init-output.log' }}

repo_update: true
repo_upgrade: all
packages:
- awscli

runcmd:
- [ bash, -c, "{copy_build_command}" ]
- [ bash, -c, "{build_args_command}" ]
- [ bash, -c, "{build_start_command}" ]
    """.encode('ascii')).decode('ascii')

    # make spot fleet request config
    account_id = boto3.client('sts').get_caller_identity().get('Account')
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
                        'DeviceName': '/dev/sda1',
                        'Ebs': {
                            'DeleteOnTermination': True,
                            'VolumeSize': 300,
                            'VolumeType': 'gp3'
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
        print(f"not including SSH key in spot request as no key in region {cheapest_region} with name {SSH_KEY_NAME} found: {e}")

    print("spot_fleet_request_config: {}".format(spot_fleet_request_config))

    try:
        print(f"requesting spot instance in AZ {cheapest_az} with current price of {cheapest_price}")
        response = client.request_spot_fleet(SpotFleetRequestConfig=spot_fleet_request_config)
        print(f"spot request response: {response}")
        spot_fleet_request_id = response.get('SpotFleetRequestId')
    except Exception as e:
        message = f"There was a problem requesting a spot instance {INSTANCE_TYPE}: {e}"
        send_sns_message("RattlesnakeOS Spot Instance FAILED", message)
        raise

    try:
        found_instance = False
        retry_interval = 5
        retry_attempts = 30
        for i in range(1, retry_attempts):
            print(f"waiting for spot instance launch for spot fleet request id {spot_fleet_request_id}: {i}/{retry_attempts}")
            response = client.describe_spot_fleet_instances(SpotFleetRequestId=spot_fleet_request_id)
            print(response)
            if len(response.get('ActiveInstances')) > 0:
                found_instance = True
                break
            time.sleep(retry_interval)
        if not found_instance:
            raise Exception("max wait timeout for spot instance launch")
    except Exception as e:
        try:
            print(f"attempting to cancel spot fleet request id {spot_fleet_request_id}")
            client.cancel_spot_fleet_requests(SpotFleetRequestIds=[spot_fleet_request_id], TerminateInstances=True)
        except Exception as ex:
            print(f"failed to cancel spot fleet request: {ex}")
        message = f"There was a problem waiting for active spot instance launch {INSTANCE_TYPE}: {e}"
        send_sns_message("RattlesnakeOS Spot Instance FAILED", message)
        raise

    chromium_message = ""
    if CHROMIUM_BUILD_DISABLED == "false":
        chromium_message = f"Chromium Version: {latest_chromium_version}\n "

    subject = "RattlesnakeOS Spot Instance LAUNCHED"
    message = f"Successfully launched a spot instance.\n\n Stack Name: {NAME}\n Stack Version: {STACK_VERSION}\n Device: {DEVICE}\n Release: {latest_release}\n Tag: {latest_aosp_tag}\n Build ID: {latest_aosp_build_id}\n {chromium_message}Instance Type: {INSTANCE_TYPE}\n Cheapest Region: {cheapest_region}\n Cheapest Hourly Price: ${cheapest_price}\n Build Reason: {build_reason} "
    send_sns_message(subject, message)
    return message.replace('\n', ' ')


def is_build_required(latest_release):
    s3 = boto3.resource('s3')
    needs_update = False
    reason = ""

    existing_release_version = ""
    try:
        existing_release_version = s3.Object(RELEASE_BUCKET, "release").get()['Body'].read().decode().strip("\n")
    except Exception as e:
        print("failed to get existing_release_version: {}".format(e))
        pass
    if latest_release > existing_release_version:
        needs_update = True
        reason = "New release '{}'".format(latest_release)
        print(reason)
        return needs_update, reason

    return needs_update, reason


def find_cheapest_region():
    cheapest_price = 0
    cheapest_region = ""
    cheapest_az = ""
    for region in INSTANCE_REGIONS.split(","):
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
    return cheapest_price, cheapest_region, cheapest_az


def send_sns_message(subject, message):
    account_id = boto3.client('sts').get_caller_identity().get('Account')
    sns = boto3.client('sns')
    resp = sns.publish(TopicArn=SNS_ARN.format(account_id), Subject=subject, Message=message)
    print("Sent SNS message {} and got response: {}".format(message, resp))


if __name__ == '__main__':
    lambda_handler("", "")
