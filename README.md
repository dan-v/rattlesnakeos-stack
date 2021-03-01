Build your own customized Android OS for Google Pixel phones using [AWS](https://aws.amazon.com/) cloud infrastructure. The default OS that this tools builds without any customizations is called `RattlesnakeOS`.  If there is something you don't like about the default OS, you can add customizations on top of it or start with a completely blank slate and build your own OS.

## Features
* Support for Google Pixel phones
* Based on latest [AOSP](https://source.android.com/) 11.0
* Software and firmware security updates delivered through built in OTA updater
* Maintains [verified boot](https://source.android.com/security/verifiedboot/) with a locked bootloader just like official Android but with your own personal signing keys
* Support for building latest stable Chromium [browser](https://www.chromium.org) and [webview](https://www.chromium.org/developers/how-tos/build-instructions-android-webview)
* Support for custom OS builds

## Overview
The default OS built by this tool, `RattlesnakeOS`, is just stock AOSP and has all the baseline privacy and security features from there. Unlike other alternative Android OSes, it aims to keep security on par with stock Android by keeping critical security features like verified boot enabled and ensuring monthly OTA security updates not only update the OS but also the device specific drivers and firmware.

Rather than providing random binaries of an Android OS to install on your phone, I've gone the route of creating a cross platform tool, `rattlesnakeos-stack`, that provisions a "stack", which is just all the AWS cloud infrastructure needed to continuously build your own personal Android OS, with your own signing keys, and your own OTA updates. It uses [AWS Lambda](https://aws.amazon.com/lambda/features/) to provision [EC2 spot instances](https://aws.amazon.com/ec2/spot/) that build the OS and upload artifacts to [S3](https://aws.amazon.com/s3/). Resulting OS builds are configured to receive over the air updates from this environment. It only costs a few dollars a month to run (see FAQ for detailed cost breakdown).

![](/images/overview.png?raw=true)

## Table of Contents
   * [Prerequisites](#prerequisites)
   * [Installation](#installation)
   * [Configuration](#configuration)
   * [Deployment](#deployment)
      * [Default Examples](#default-examples)
      * [Advanced Examples](#advanced-examples)
   * [First Time Setup After Deployment](#first-time-setup-after-deployment)
   * [Customizations](#customizations)
   * [FAQ](#faq)
     * [General](#general)
     * [Costs](#costs)
     * [Builds](#builds)
     * [Security](#security)
   * [Uninstalling](#uninstalling)


## Prerequisites
* An AWS account. You'll need to [create an AWS account](https://portal.aws.amazon.com/billing/signup) if you don't have one. <b>If this is a new AWS account, make sure you launch at least one paid instance before running through these steps.</b>  To do this you can navigate to the [EC2 console](https://console.aws.amazon.com/ec2/), click `Launch instance`, select any OS, pick a `c5.4xlarge`, and click `Review and launch`. <b>After it launches successfully you can terminate the instance through the console</b>. If you can't launch an instance of this size with your new account, you may need to request a limit increase through the web console for that instance type.
* In the AWS web console, you'll need to setup AWS credentials with `AdministratorAccess` access. If you're not sure how to do that, you can follow [this step by step guide](https://serverless-stack.com/chapters/create-an-iam-user.html). You'll need the generated AWS Access Key and Secret Key for the next step.
* On your local computer, install the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/installing.html) for your platform and [configure](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html) it to use the credentials from previous step. Verify that the CLI credentials are configured properly by running a command like 'aws s3 ls' and make sure no errors are returned.
* On your local computer, using the CLI, generate an SSH key and upload the public key to all regions in AWS as shown below. By default we'll name the keypair `rattlesnakeos` in AWS. The public and private SSH key will be dumped to the current directory, make sure to save these to a safe place.
```
keypair_name="rattlesnakeos"
ssh-keygen -t rsa -b 4096 -f ${keypair_name}
for region in $(aws ec2 describe-regions --query "Regions[*].RegionName" --output text); do
  echo "Importing keypair ${keypair_name} to region ${region}..."
  aws ec2 import-key-pair --key-name "${keypair_name}" --public-key-material "file://${keypair_name}.pub" --region $region
done
```

## Installation
The `rattlesnakeos-stack` tool needs to be installed on your local computer. The easiest way is to download a pre-built binary from the [Github Releases](https://github.com/dan-v/rattlesnakeos-stack/releases) page. The other option is to [build from source](#build-from-source).

## Configuration
The rattlesnakeos-stack `config` subcommand should be run first to initialize a config file which will be stored in `$HOME/.rattlesnakeos.toml`. By default, an autogenerated stack name will be generated for `<rattlesnakeos-stackname>`; if you want to customize this name beware that the name must be globally unique in AWS or deployment will fail.

```none
./rattlesnakeos-stack config

Device is the device codename (e.g. sunfish).
device: sunfish

Stack name is used as an identifier for all the AWS components that get deployed. THIS NAME MUST BE UNIQUE OR DEPLOYMENT WILL FAIL.
Stack name: <rattlesnakeos-stackname>

Stack region is the AWS region where you would like to deploy your stack. Valid options: us-west-2, us-east-1, us-east-2, us-west-1, eu-west-1, eu-west-2, eu-west-3, ap-northeast-3, ap-northeast-2, ap-northeast-1, sa-east-1, ap-southeast-1, ca-central-1, ap-southeast-2, ap-south-1, eu-central-1, cn-north-1, cn-northwest-1
Stack region: us-west-2

Email address you would like to send build notifications to.
Email: user@domain.com

SSH keypair name is the name of your EC2 keypair that imported into AWS.
SSH Keypair Name: rattlesnakeos

INFO[0005] rattlesnakeos-stack config file has been written to /Users/username/.rattlesnakeos.toml
```

## Deployment
The rattlesnakeos-stack `deploy` subcommand handles deploying (and updating) your stack. After stack deployment, you will need to manually start a build. By default, it is configured to automatically build once a month on the 10th of the month so that monthly security updates can be picked up and built without the need for manual builds. <b>Anytime you make a config change, you will first need to deploy those changes using this command before starting a new build</b>.

#### Default Examples
Deploy stack using default generated config file:
```none 
./rattlesnakeos-stack deploy

INFO[0000] Using config file: /Users/user/.rattlesnakeos.toml
INFO[0000] Current settings:
chromium-version: ""
device: taimen
email: user@domain.com
hosts-file: ""
instance-regions: us-west-2,us-west-1,us-east-2
instance-type: c5.4xlarge
max-price: "1.00"
name: <rattlesnakeos-stackname>
region: us-west-2
schedule: rate(14 days)
skip-price: "0.68"
ssh-key: rattlesnakeos

Do you want to continue? [y/N]
```

You can override values in the config file with CLI flags:
```none 
./rattlesnakeos-stack deploy --region "us-west-2"
...
```

You can also persist values you override to the config file if desired:
```none 
./rattlesnakeos-stack deploy --region "us-west-2" --save-config
...
```

Or you can specify a different config file to use
```none 
./rattlesnakeos-stack deploy --config-file foo.toml
...
```

To see full list of options you can run `rattlesnakeos-stack deploy -h`. These flags can also be set as config values in the config file.

#### Advanced Examples
Here is an example of a more advanced config file that: disables chromium build (warning: if you do this - you should provide your own up to date webview), disables scheduled monthly builds, specifies a custom configuration repo (more on that in customization section), and uses a much larger c5.24xlarge instance type.
```toml 
chromium-build-disabled = true
chromium-version = ""
cloud = "aws"
core-config-repo = "https://github.com/rattlesnakeos/core"
custom-config-repo = "https://github.com/myrepo/custom"
device = "sunfish"
email = "dan@vittegleo.com"
instance-regions = "us-west-2,us-west-1,us-east-2"
instance-type = "c5.24xlarge"
latest-url = "https://raw.githubusercontent.com/RattlesnakeOS/latest/11.0/latest.json"
max-price = "5.00"
name = "sunfish-cyoydyw3j2"
region = "us-east-2"
schedule = ""
skip-price = "5.00"
ssh-key = "rattlesnakeos"
```

## First Time Setup After Deployment
* Click on the email confirmation link sent to your email in order to start getting build notifications.
* You'll need to manually start your first build using `rattlesnakeos-stack` tool. Future builds will happen automatically based on the schedule defined in your configuration.

    ```sh 
    ./rattlesnakeos-stack build start
    ```

* You should get email notifications that your build has started. If you didn't get an email notification with the details of where it launched, you can use the CLI to list active builds. 

    ```sh 
    ./rattlesnakeos-stack build list
    ```

* The <b>initial build will likely take 7+ hours to complete</b>. Looking at the EC2 instance metrics like CPU, etc is NOT a good way to determine if the build is progressing. See the FAQ for details on how to monitor live build progress.
* After the build finishes, a factory image should be uploaded to the S3 release bucket that you can download. Be sure to replace the command below with your stack name and your device name (e.g. sunfish).
   
    ```sh 
    aws s3 cp s3://<rattlesnakeos-stackname>-release/<device>-factory-latest.zip .
    ```

* Use this factory image and [follow the instructions on flashing your device carefully](FLASHING.md).
* After successfully flashing your device, you will now be running RattlesnakeOS and all future updates will happen through the built-in OTA updater.
* <b>I HIGHLY suggest backing up your generated signing keys and storing them somewhere safe</b>. To back up your signing keys:

    ```sh 
    aws s3 sync s3://<rattlesnakeos-stackname>-keys/ .
    ```

## Customizations
It is possible to customize OS builds to your liking by specifying a custom config repo with the config option `custom-config-repo = "https://github.com/yourrepo/name"`. This git repo needs to adhere to a specific format that will be covered below.

<b>IMPORTANT: using any Git repo here that is not in your control is a security risk, as you are giving control of your build process to the owner of the repo. They could steal your signing keys, inject malicious code, etc.</b>

### Custom Config Repo format
The custom config git repo needs to be laid out in a specific format to work with the build process. An example repo can be found here: https://github.com/RattlesnakeOS/example-custom-config-repo. The directory structure looks like this:
```
hooks/
local_manifests/
vendor/
```
* `hooks` - this directory can contain shell scripts that can hook into the build process at various steps along the way. There are `pre` and `post` entry points. The shell scripts need to be named `<build_function_function_to_hook>_<pre|post>.sh` (e.g. aosp_build_pre.sh). Right now these hooks scripts are sourced in a subshell, so all environment variables from the core build script are available to these hooks (e.g. AOSP_BUILD_DIR, etc), but it's best to limit environment dependencies, as backwards compatibility is not guaranteed as the core build script changes.
* `local_manifests` - this is a directory for local AOSP manifests to be placed. These manifests will be synced to the AOSP build tree.
* `vendor` - is a place to override vendor configuration. You can make use of the support for AOSP overlays to easily modify configuration settings. Under the `vendor` directory, there needs to be a mk file at `config/main.mk`.

## FAQ
### General
#### Should I use rattlesnakeos-stack?
Use this at your own risk.
#### Where can I get help, ask questions, keep up to date on development?
* For general questions and keeping up to date, use subreddit [/r/RattlesnakeOS](https://www.reddit.com/r/RattlesnakeOS/)
* If you run into any issues with rattlesnakeos-stack, please [file an issue or feature request on Github](https://github.com/dan-v/rattlesnakeos-stack/issues) and provide all the requested information in the issue template.
#### How do I update rattlesnakeos-stack?
Just download the new version of rattlesnakeos-stack and run deploy again (e.g. ./rattlesnakeos-stack deploy)
#### How do OTA updates work?
If you go to `Settings -> System -> Advanced (to expand) -> System update settings`, you'll see the updater app settings. The updater app will check S3 to see if there are updates and if it finds one will download and apply it your device.
#### What network carriers are supported?
I only have access to a single device and carrier to test this on, so I can't make any promises about it working with your specific carrier. Confirmed working: T-Mobile, Rogers, Cricket, Ting. Likely not to work: Sprint (has requirements about specific carrier app being on phone to work), Project Fi.
#### Why is this project so closely tied to AWS?
Building AOSP and Chromium requires a fairly powerful server, which is not something everyone readily has access to. Using a cloud provider allows you to spin up compute resources capable of building these projects and only paying for the time you use them. It could really be any cloud provider, but just happened to choose AWS. There are pros and cons to building AOSP in the cloud. On the positive side, cloud providers allow you to easily write automation that can spin up and down resources as needed which allows rattlesnakeos-stack to automate the entire process of building an Android OS and distributing OTA updates. On the downsides, for those that are very security conscious, they may be wary of building an OS on shared cloud resources. You can check out the [security section of the FAQ](#security) for more details on this.
#### Will you support other devices?
It's not likely that other devices will be supported beyond the Google Pixel line. Here are some reasons:
* Support for verified boot with a locked bootloader is a requirement for this project. The Pixel line of phones are fairly unique in that they support verified boot with custom signing keys while locking the bootloader with an alternative OS. I am sure there are/will be devices that support this, but I don't know of any others.
* Being able to get monthly AOSP security updates for a device is a requirement for this project. Google provides proper AOSP releases every month for Pixel devices which makes it very simple to build and stay up to date with monthly security updates - most vendors don't provide this.
* Being able to get monthly firmware and driver updates is a requirement for this project. Google provides updated firmware and drivers for Pixel devices every month (although incomplete - the vendor specific code ends up being extracted from monthly updated factory images) - regardless most vendors don't provide this.
* Even if there is another device that meets these requirements, the build process would likely differ enough that supporting it would be too much overhead. The current build differences between each Pixel device is relatively minor.
#### Is this a fork of CopperheadOS?
No. RattlesnakeOS was created initially as an alternative to [CopperheadOS](https://en.wikipedia.org/wiki/CopperheadOS), a security hardened Android OS created by [Daniel Micay](https://twitter.com/DanielMicay), after it stopped being properly maintained back in June 2018. To be clear, this project is not attempting to add or recreate any of the security hardening features that were present in CopperheadOS. If you are interested in the continuation of the CopperheadOS project you can check out [GrapheneOS](https://grapheneos.org/).

### Costs
#### How much does this cost to run?
The costs are going to be variable by AWS region and by day and time you are running your builds, as spot instances have a variable price depending on market demand. Below is an example scenario that should give you a rough estimate of costs:
   * The majority of the cost will come from builds on EC2. It currently launches spot instances of type c5.4xlarge which average maybe $.30 an hour in us-west-2 (will vary by region) but can get up over $1 an hour depending on the day and time. You can modify the default `max-price` config value to set the max price you are willing to pay and if market price exceeds that then your instance will be terminated. Builds can take anywhere from 3-7 hours depending on if Chromium needs to be built. So let's say you're doing a build every month at $0.50 an hour, and it is taking on average 4 hours - you'd pay ~$2 in EC2 costs per month. 
   * The other very minimal cost would be S3. Storage costs are almost non-existent as a stack will only store about 2GB worth of files (factory image, ota file) and at $0.023 per GB you're looking at $0.07 per month in S3 storage costs. The other S3 cost would be for data transfer out for OTA updates - let's say you are just downloading an update per month (~500MB file) at $0.09 per GB you're looking at $0.05 per month in S3 network costs.

### Builds
#### How do I change build frequency?
By default, it is configured to automatically build once a month on the 10th of the month so that monthly updates can be picked up and built without the need for manual builds. There is a config option to specify how frequently builds are kicked off automatically. For example you could set `schedule = "rate(14 days)"` in the config file to build every 14 days. Also note, the default behavior is to only run a build if there have been version updates in stack, AOSP, or Chromium versions.
#### How do I manually start a build?
You can manually kick off a build with the CLI. Note that this shouldn't normally be necessary as builds are set to happen automatically on a cron schedule.
```sh 
./rattlesnakeos-stack build start
```
#### Where do I find logs for a build?
On build failure/success, the instance should terminate and upload its logs to S3 bucket called `<rattlesnakeos-stackname>-logs` and it's in a file called `<device>/<timestamp>`.
#### How can I see live build status?
There are a few steps required to be able to do this:
   * In the [default security group](https://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/VPC_SecurityGroups.html#DefaultSecurityGroup), you'll need to [open up SSH access](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/authorizing-access-to-an-instance.html).
   * You should be able to SSH into the instance (can get IP address from EC2 console or email notification): `ssh -i yourprivatekey ubuntu@yourinstancepublicip`
   * Tail the logfile to view progress `tail -f /var/log/cloud-init-output.log`
#### Why did my EC2 instance randomly terminate?
If there wasn't an error notification, this is likely because the [Spot Instance](https://aws.amazon.com/ec2/spot/) max price was not high enough or EC2 is low on capacity and needs to reclaim instances. You can see historical spot instance pricing in the [EC2 console](https://console.aws.amazon.com/ec2sp/v1/spot/home). Click `Pricing History`, select c5.4xlarge for `Instance Type` and pick a date range.

### Security
#### How secure is this?
Your ability to secure your signing keys determines how secure RattlesnakeOS is. RattlesnakeOS generates and stores signing keys in AWS, which means the security of your AWS account becomes critical to ensuring the security of your device. If you aren't able to properly secure your local workstation, and your AWS account, then these additional security protections like verified boot become less useful.

Cloud based builds are never going to be as secure as a locally built AOSP signed with highly secured keys generated from an HSM or air gapped computer, so if this is the level of security you require then there really is no other way. Would I recommend cloud builds like this for a large OEM or a company like CopperheadOS where the signing key being generated is protecting thousands of users? No, this becomes a high profile target as getting a hold of these keys essentially gives an attacker access to thousands of devices. On the other hand, for a single user generating their own key protecting a single device, there is less concern in my mind unless your threat profile includes very targeted attacks. 
#### What are some security best practices for AWS accounts?
Some minimum steps worth considering are having an account solely for building RattlesnakeOS with a strong password, enabling two-factor authentication, enabling auditing with CloudTrail, and locking down access to your AWS API credentials.

## Uninstalling
### Remove AWS resources
If you decide this isn't for you and you want to remove all the provisioned AWS resources, there's a command for that. 

```sh
./rattlesnakeos-stack remove --name <rattlesnakeos-stackname> --region us-west-2
```

<b>IMPORTANT NOTE</b>: this will not terminate any running EC2 instances that may have launched, and these will need to be terminated manually.

### Revert back to stock Android
You'll need to clear the configured AVB public key after unlocking the bootloader and before locking it again with the stock factory images.

```sh
fastboot erase avb_custom_key
```

## Donations
* [Github](https://github.com/sponsors/dan-v)
* [Liberapay](https://liberapay.com/rattlesnakeos/)
* [Bitcoin](https://www.blockchain.com/btc/address/17GHmnK3fyw9TBngvaM8Veh37UR65rmvZS)

## Powered by
* [android-prepare-vendor](https://github.com/anestisb/android-prepare-vendor)
* [CalyxOS](https://github.com/CalyxOS)
* [GrapheneOS](https://github.com/GrapheneOS)
* [Terraform](https://www.terraform.io/)
* Huimin Zhang - author of the original underlying build script that was written for CopperheadOS.

## Build from Source
 * To compile from source code you'll need to install Go 1.16+ (https://golang.org/) for your platform
  ```sh
  git clone github.com/dan-v/rattlesnakeos-stack
  make tools
  make
  ```
