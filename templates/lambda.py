#!/usr/bin/env python3
import boto3
import base64
import json
from urllib.request import urlopen
from datetime import datetime, timedelta

NAME = '<% .Config.Name %>'
SRC_PATH = 's3://<% .Config.Name %>-script/build.sh'
RELEASE_BUCKET = '<% .Config.Name %>-release'
FLEET_ROLE = 'arn:aws:iam::{0}:role/aws-service-role/spotfleet.amazonaws.com/AWSServiceRoleForEC2SpotFleet'
IAM_PROFILE = 'arn:aws:iam::{0}:instance-profile/<% .Config.Name %>-ec2'
SNS_ARN = 'arn:aws:sns:<% .Config.Region %>:{}:<% .Config.Name %>'
INSTANCE_TYPE = '<% .Config.InstanceType %>'
DEVICE = '<% .Config.Device %>'
SSH_KEY_NAME = '<% .Config.SSHKey %>'
MAX_PRICE = '<% .Config.MaxPrice %>'
SKIP_PRICE = '<% .Config.SkipPrice %>'
REGION = '<% .Config.Region %>'
REGIONS = '<% .Config.InstanceRegions %>'
REGION_AMIS = json.loads('<% .RegionAMIs %>')
AMI_OVERRIDE = '<% .Config.AMI %>'
ENCRYPTED_KEYS = '<% .Config.EncryptedKeys %>'
CHROMIUM_PINNED_VERSION = '<% .Config.ChromiumVersion %>'
LATEST_JSON = "https://raw.githubusercontent.com/RattlesnakeOS/latest/11.0/latest.json"
STACK_URL_LATEST = "https://api.github.com/repos/dan-v/rattlesnakeos-stack/releases/latest"
STACK_VERSION = '<% .Config.Version %>'


def lambda_handler(event, context):
    # get latest
    latest_stack_json = json.loads(urlopen(STACK_URL_LATEST).read().decode())
    latest_stack_version = latest_stack_json.get('name')
    print("latest_stack_version", latest_stack_version)
    latest_json = json.loads(urlopen(LATEST_JSON).read().decode())
    latest_chromium_version = latest_json.get('chromium')
    print("latest_chromium_version", latest_chromium_version)
    latest_fdroid_client_version = latest_json.get('fdroid').get('client')
    print("latest_fdroid_client_version", latest_fdroid_client_version)
    latest_fdroid_priv_version = latest_json.get('fdroid').get('privilegedextention')
    print("latest_fdroid_priv_version", latest_fdroid_priv_version)
    latest_aosp_build_id = latest_json.get('devices').get(DEVICE).get('build_id')
    print("latest_aosp_build_id", latest_aosp_build_id)
    latest_aosp_tag = latest_json.get('devices').get(DEVICE).get('aosp_tag')
    print("latest_aosp_tag", latest_aosp_tag)

    # build time overrides
    force_build = event.get('ForceBuild', False)
    print("force_build", force_build)
    aosp_build_id = event.get('AOSPBuildID', latest_aosp_build_id)
    print("aosp_build_id", aosp_build_id)
    aosp_tag = event.get('AOSPTag', latest_aosp_tag)
    print("aosp_tag", aosp_tag)
    chromium_version = event.get('ChromiumVersion', CHROMIUM_PINNED_VERSION if CHROMIUM_PINNED_VERSION != "" else latest_chromium_version)
    print("chromium_version", chromium_version)
    fdroid_client_version = event.get('FDroidClientVersion', latest_fdroid_client_version)
    print("fdroid_client_version", fdroid_client_version)
    fdroid_priv_version = event.get('FDroidPrivVersion', latest_fdroid_priv_version)
    print("fdroid_priv_version", fdroid_priv_version)

    # check if build is required
    needs_build, build_reasons = is_build_required(latest_stack_version, aosp_build_id, chromium_version, fdroid_client_version, fdroid_priv_version)
    if not needs_build and not force_build:
        message = "RattlesnakeOS build is already up to date."
        send_sns_message("RattlesnakeOS Build Not Required", message)
        return message
    if not needs_build and force_build:
        build_reasons.append("Build not required - but force build flag was specified.")
    print("needs_build", needs_build)
    print("build_reasons", build_reasons)

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
    ami = AMI_OVERRIDE if AMI_OVERRIDE else REGION_AMIS[cheapest_region]

    # create ec2 client for cheapest region
    client = boto3.client('ec2', region_name=cheapest_region)

    # get a subnet in cheapest az to request spot instance in
    subnets = client.describe_subnets(Filters=[{'Name': 'availabilityZone', 'Values': [cheapest_az]}])['Subnets'][0][
        'SubnetId']

    # userdata to deploy with spot instance
    copy_build_command = f"sudo -u ubuntu aws s3 --region {REGION} cp {SRC_PATH} /home/ubuntu/build.sh"
    build_args_command = f"echo \\\"aosp_build_id={aosp_build_id} aosp_tag={aosp_tag} chromium_version={chromium_version} fdroid_client_version={fdroid_client_version} fdroid_priv_version={fdroid_priv_version}\\\" > /home/ubuntu/build_args"
    build_start_command = f"sudo -u ubuntu bash /home/ubuntu/build.sh \\\"{aosp_build_id}\\\" \\\"{aosp_tag}\\\" \\\"{chromium_version}\\\" \\\"{fdroid_client_version}\\\" \\\"{fdroid_priv_version}\\\""
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
            message = f"Encrypted keys is enabled, so properly configured SSH keys are mandatory. Unable to find an EC2 Key Pair named '{SSH_KEY_NAME}' in region {cheapest_region}."
            send_sns_message("RattlesnakeOS Spot Instance CONFIGURATION ERROR", message)
            return message
        else:
            print(f"not including SSH key in spot request as no key in region {cheapest_region} with name {SSH_KEY_NAME} found: {e}")

    print("spot_fleet_request_config: {}".format(spot_fleet_request_config))

    try:
        print(f"requesting spot instance in AZ {cheapest_az} with current price of {cheapest_price}")
        response = client.request_spot_fleet(SpotFleetRequestConfig=spot_fleet_request_config)
        print(f"spot request response: {response}")
    except Exception as e:
        message = f"There was a problem requesting a spot instance {INSTANCE_TYPE}: {e}"
        send_sns_message("RattlesnakeOS Spot Instance FAILED", message)
        raise

    subject = "RattlesnakeOS Spot Instance SUCCESS"
    message = f"Successfully requested a spot instance.\n\n Stack Name: {NAME}\n Device: {DEVICE}\n Instance Type: {INSTANCE_TYPE}\n Cheapest Region: {cheapest_region}\n Cheapest Hourly Price: ${cheapest_price}\n Reason: {build_reasons}"
    send_sns_message(subject, message)
    return message.replace('\n', ' ')


def is_build_required(latest_stack_version, aosp_build_id, chromium_version, fdroid_client_version, fdroid_priv_version):
    s3 = boto3.resource('s3')
    needs_update = False
    build_reasons = []

    # STACK
    existing_stack_version = ""
    try:
        existing_stack_version = s3.Object(RELEASE_BUCKET, "rattlesnakeos-stack/revision").get()[
            'Body'].read().decode().strip("\n")
    except Exception as e:
        print("failed to get existing_stack_version: {}".format(e))
        pass
    if existing_stack_version == "":
        needs_update = True
        build_reasons.append("Initial build")
        return needs_update, build_reasons
    if existing_stack_version != latest_stack_version:
        print("WARNING: existing stack version {} is not the latest {}", existing_stack_version, latest_stack_version)

    # AOSP
    existing_aosp_build_id = ""
    try:
        existing_aosp_build_id = s3.Object(RELEASE_BUCKET, "{}-vendor".format(DEVICE)).get()[
            'Body'].read().decode().strip("\n")
        print("existing_aosp_build_id='{}'".format(existing_aosp_build_id))
    except Exception as e:
        print("failed to get existing_aosp_build_id: {}".format(e))
        pass
    if existing_aosp_build_id != aosp_build_id:
        needs_update = True
        build_reasons.append("AOSP build id {} != {}".format(existing_aosp_build_id, aosp_build_id))

    # CHROMIUM
    existing_chromium_version = ""
    try:
        existing_chromium_version = s3.Object(RELEASE_BUCKET, "chromium/revision").get()['Body'].read().decode().strip(
            "\n")
        print("existing_chromium_version='{}'".format(existing_chromium_version))
    except Exception as e:
        print("failed to get existing_chromium_version: {}".format(e))
        pass
    chromium_included = False
    try:
        chromium_included_text = s3.Object(RELEASE_BUCKET, "chromium/included").get()['Body'].read().decode().strip(
            "\n")
        if chromium_included_text == "yes":
            chromium_included = True
    except:
        print("failed to get chromium_included: {}".format(e))
        pass

    if existing_chromium_version == chromium_version and chromium_included:
        print("chromium build {} is up to date".format(existing_chromium_version))
    else:
        needs_update = True
        if existing_chromium_version == chromium_version:
            print("chromium {} was built but not installed".format(existing_chromium_version))
            build_reasons.append("Chromium version {} built but not installed".format(existing_chromium_version))
        else:
            print("chromium needs to be updated to {}".format(chromium_version))
            build_reasons.append("Chromium version {} != {}".format(existing_chromium_version, chromium_version))

    # FDROID
    existing_fdroid_client_version = ""
    try:
        existing_fdroid_client_version = s3.Object(RELEASE_BUCKET, "fdroid/revision").get()[
            'Body'].read().decode().strip("\n")
        print("existing_fdroid_client_version='{}'".format(existing_chromium_version))
    except:
        print("failed to get existing_fdroid_client_version: {}".format(e))
        pass
    if existing_fdroid_client_version == fdroid_client_version:
        print("fdroid client {} is up to date".format(existing_fdroid_client_version))
    else:
        print("fdroid needs to be updated to {}".format(fdroid_client_version))
        needs_update = True
        build_reasons.append("F-Droid version {} != {}".format(existing_fdroid_client_version, fdroid_client_version))

    # FDROID PRIV
    existing_fdroid_priv_version = ""
    try:
        existing_fdroid_priv_version = s3.Object(RELEASE_BUCKET, "fdroid-priv/revision").get()[
            'Body'].read().decode().strip("\n")
        print("existing_fdroid_priv_version='{}'".format(existing_fdroid_priv_version))
    except Exception as e:
        print("failed to get existing_fdroid_priv_version: {}".format(e))
        pass
    if existing_fdroid_priv_version == fdroid_priv_version:
        print("fdroid priv {} is up to date".format(fdroid_priv_version))
    else:
        print("fdroid priv needs to be updated to {}".format(fdroid_priv_version))
        needs_update = True
        build_reasons.append(
            "F-Droid priv ext version {} != {}".format(existing_fdroid_priv_version, fdroid_priv_version))

    return needs_update, build_reasons

def find_cheapest_region():
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
    return cheapest_price, cheapest_region, cheapest_az

def send_sns_message(subject, message):
    account_id = boto3.client('sts').get_caller_identity().get('Account')
    sns = boto3.client('sns')
    resp = sns.publish(TopicArn=SNS_ARN.format(account_id), Subject=subject, Message=message)
    print("Sent SNS message {} and got response: {}".format(message, resp))


if __name__ == '__main__':
    lambda_handler("", "")
