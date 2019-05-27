RattlesnakeOS is a privacy and security focused Android OS for Google Pixel phones.

## Features
* Based on latest [AOSP](https://source.android.com/) 9.0 (Android P)
* Support for Google <b>Pixel, Pixel XL, Pixel 2, Pixel 2 XL, Pixel 3, Pixel 3 XL, Pixel 3a, Pixel 3a XL</b>
* Monthly software and firmware security fixes delivered through built in OTA updater
* [Verified boot](https://source.android.com/security/verifiedboot/) with a locked bootloader just like official Android but with your own personal signing keys
* Optional support for [remote attestation](#how-does-the-remote-attestation-feature-work) using [Auditor](https://github.com/GrapheneOS/Auditor) and [AttestationServer](https://github.com/GrapheneOS/AttestationServer)
* Latest stable Chromium [browser](https://www.chromium.org) and [webview](https://www.chromium.org/developers/how-tos/build-instructions-android-webview)
* Latest stable [F-Droid](https://f-droid.org/) app store and [privileged extension](https://gitlab.com/fdroid/privileged-extension)
* Free of Google’s apps and services
* Advanced build customization options

## Background
RattlesnakeOS is truly just stock AOSP and has all of the baseline privacy and security features from there. Unlike other alternative Android OSes, it aims to keep security on par with stock Android by keeping critical security features like verified boot enabled, ensuring monthly OTA security updates not only update the OS but also the device specific drivers and firmware, and by not adding additional features or software that will needlessly increase attack surface. By not deviating from stock AOSP, updating to new major Android releases doesn't require any major porting effort and this means devices running RattlesnakeOS continue to receive proper security updates without delay.

## What is rattlesnakeos-stack?
Rather than providing random binaries of RattlesnakeOS to install on your phone, I've gone the route of creating a cross platform tool, `rattlesnakeos-stack`, that provisions a "stack", which is just all of the [AWS](https://aws.amazon.com/) cloud infrastructure needed to continuously build your own personal RattlesnakeOS, with your own signing keys, and your own OTA updates. It uses [AWS Lambda](https://aws.amazon.com/lambda/features/) to provision [EC2 spot instances](https://aws.amazon.com/ec2/spot/) that build RattlesnakeOS and upload artifacts to [S3](https://aws.amazon.com/s3/). Resulting OS builds are configured to receive over the air updates from this environment. It only costs a few dollars a month to run (see FAQ for detailed cost breakdown).

![](/images/overview.png?raw=true)

## Table of Contents
   * [Prerequisites](#prerequisites)
   * [Installation](#installation)
   * [Configuration](#configuration)
   * [Deployment](#deployment)
      * [Default Examples](#default-examples)
      * [Advanced Examples](#advanced-examples)
      * [All Options](#all-options)
   * [First Time Setup After Deployment](#first-time-setup-after-deployment)
   * [FAQ](#faq)
     * [General](#general)
     * [Costs](#costs)
     * [Builds](#builds)
     * [Remote Attestation](#remote-attestation)
     * [Customizations](#customizations)
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
for region in $(aws ec2 describe-regions --output text | awk '{print $3}'); do
  echo "Importing keypair ${keypair_name} to region ${region}..."
  aws ec2 import-key-pair --key-name "${keypair_name}" --public-key-material "file://${keypair_name}.pub" --region $region;
done
```

## Installation
The `rattlesnakeos-stack` tool needs to be installed on your local computer. The easiest way is to download a pre-built binary from the [Github Releases](https://github.com/dan-v/rattlesnakeos-stack/releases) page. The other option is to [build from source](#build-from-source).

## Configuration
The rattlesnakeos-stack `config` subcommand should be run first to initialize a config file which will be stored in `$HOME/.rattlesnakeos.toml`. By default, an autogenerated stack name will be generated for `<rattlesnakeos-stackname>`; if you want to customize this name beware that the name must be globally unique in AWS or deployment will fail.

```none
./rattlesnakeos-stack config

Device is the device codename (e.g. sailfish). Supported devices: sailfish (Pixel), marlin (Pixel XL), walleye (Pixel 2), taimen (Pixel 2 XL), blueline (Pixel 3), crosshatch (Pixel 3 XL)
Device: taimen

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
The rattlesnakeos-stack `deploy` subcommand handles deploying (and updating) your stack. After stack deployment, you will need to manually start a build. By default it is configured to automatically build once a month on the 10th of the month so that monthly security updates can be picked up and built without the need for manual builds. <b>Anytime you make a config change, you will first need to deploy those changes using this command before starting a new build</b>.

#### Default Examples
Deploy stack using default generated config file:
```none 
./rattlesnakeos-stack deploy

INFO[0000] Using config file: /Users/user/.rattlesnakeos.toml
INFO[0000] Current settings:
chromium-version: ""
device: taimen
email: user@domain.com
encrypted-keys: false
ignore-version-checks: false
hosts-file: ""
instance-regions: us-west-2,us-west-1,us-east-1,us-east-2
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

#### Advanced Examples
Here is an example of a more advanced config file that: enables deployment of a personal attestation server, locks to a specific version of Chromium, specifies a hosts file to install, uses a larger EC2 instance type, builds every 2 days, and pulls in custom patches from the [community patches repo](https://github.com/RattlesnakeOS/community_patches). You can read more about [advanced customization options in FAQ](#customizations).
```toml 
attestation-server = true
chromium-version = "70.0.3538.80"
device = "crosshatch"
email = "user@domain.com"
encrypted-keys = "false"
ignore-version-checks = false
hosts-file = "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"
instance-regions = "us-west-2,us-west-1,us-east-1,us-east-2"
instance-type = "c5.18xlarge"
max-price = "1.00"
name = "<rattlesnakeos-stackname>"
region = "us-west-2"
schedule = "rate(2 days)"
skip-price = "1.00"
ssh-key = "rattlesnakeos"

[[custom-patches]]
  patches = [
        "00001-global-internet-permission-toggle.patch",
        "00002-global-sensors-permission-toggle.patch",
  ]
  repo = "https://github.com/RattlesnakeOS/community_patches"
```

#### All Options
To see full list of options you can pass rattlesnakeos-stack you can use the help flag (-h). These flags can also be set as config values in the config file.

```none
...

Usage:
  rattlesnakeos-stack deploy [flags]

Flags:
      --attestation-instance-type string   instance type to use for attestation server. (default "t3.nano")
      --attestation-max-price string       max ec2 spot instance bid for attestation server. if this value is too low, you may not launch an instance. (default ".005")
      --attestation-server                 deploys and configures a personal attestation server
      --chromium-version string            specify the version of Chromium you want (e.g. 69.0.3497.100) to pin to. if not specified, the latest stable version of Chromium is used.
  -d, --device string                      device you want to build for (e.g. marlin): to list supported devices use '-d list'
  -e, --email string                       email address you want to use for build notifications
      --encrypted-keys                     an advanced option that allows signing keys to be stored with symmetric gpg encryption and decrypted into memory during the build process. this option requires manual intervention during builds where you will be sent a notification and need to provide the key required for decryption over SSH to continue the build process. important: if you have an existing stack - please see the FAQ for how to migrate your keys
  -h, --help                               help for deploy
      --hosts-file string                  an advanced option that allows you to specify a replacement /etc/hosts file to enable global dns adblocking (e.g. https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts). note: be careful with this, as you 1) won't get any sort of notification on blocking 2) if you need to unblock something you'll have to rebuild the OS
      --ignore-version-checks              ignore the versions checks for stack, AOSP, Chromium, and F-Droid and always do a build.
      --instance-regions string            possible regions to launch spot instance. the region with cheapest spot instance price will be used. (default "us-west-2,us-west-1,us-east-1,us-east-2")
      --instance-type string               EC2 instance type (e.g. c4.4xlarge) to use for the build. (default "c5.4xlarge")
      --max-price string                   max ec2 spot instance bid. if this value is too low, you may not obtain an instance or it may terminate during a build. (default "1.00")
  -n, --name string                        name for stack. note: this must be a valid/unique S3 bucket name.
      --prevent-shutdown                   for debugging purposes only - will prevent ec2 instance from shutting down after build.
  -r, --region string                      aws region for stack deployment (e.g. us-west-2)
      --save-config                        allows you to save all passed CLI flags to config file
      --schedule string                    cron expression that defines when to kick off builds. by default this is set to build on the 10th of every month. note: if you give an invalid expression it will fail to deploy the stack. see this for cron format details: https://docs.aws.amazon.com/AmazonCloudWatch/latest/events/ScheduledEvents.html#CronExpressions (default "cron(0 0 10 * ? *)")
      --skip-price string                  skip requesting ec2 spot instance if price is above this value to begin with. (default "0.68")
      --ssh-key string                     aws ssh key to add to ec2 spot instances. this is optional but is useful for debugging build issues on the instance.

Global Flags:
      --config-file string   config file (default location to look for config is $HOME/.rattlesnakeos.toml)
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

* The <b>initial build will likely take 5+ hours to complete</b>. Looking at the EC2 instance metrics like CPU, etc is NOT a good way to determine if the build is progressing. See the FAQ for details on how to monitor live build progress.
* After the build finishes, a factory image should be uploaded to the S3 release bucket that you can download. Be sure to replace the command below with your stack name and your device name (e.g. taimen).
   
    ```sh 
    aws s3 cp s3://<rattlesnakeos-stackname>-release/<device>-factory-latest.tar.xz .
    ```

* Use this factory image and [follow the instructions on flashing your device carefully](FLASHING.md).
* After successfully flashing your device, you will now be running RattlesnakeOS and all future updates will happen through the built in OTA updater.
* <b>I HIGHLY suggest backing up your generated signing keys and storing them somewhere safe</b>. To backup your signing keys:

    ```sh 
    aws s3 sync s3://<rattlesnakeos-stackname>-keys/ .
    # or if you are using encrypted keys
    aws s3 sync s3://<rattlesnakeos-stackname>-keys-encrypted/ .
    ```

## FAQ
### General
#### Should I use rattlesnakeos-stack?
Use this at your own risk.
#### Where can I get help, ask questions, keep up to date on development?
* For general questions and keeping up to date, use subreddit [/r/RattlesnakeOS](https://www.reddit.com/r/RattlesnakeOS/)
* If you run into any issues with rattlesnakeos-stack, please [file an issue or feature request on Github](https://github.com/dan-v/rattlesnakeos-stack/issues) and provide all of the requested information in the issue template.
#### How do I update rattlesnakeos-stack?
Just download the new version of rattlesnakeos-stack and run deploy again (e.g. ./rattlesnakeos-stack deploy)
#### How do OTA updates work?
If you go to `Settings->System update settings` you'll see the updater app settings. The updater app will check S3 to see if there are updates and if it finds one will download and apply it your device. There is no progress indicator unfortunately - you'll just got a notification when it's done and it will ask you to reboot. If you want to force a check for OTA updates, you can toggle the `Require battery above warning level` setting and it will check for a new build in your S3 bucket.
#### What network carriers are supported?
I only have access to a single device and carrier to test this on, so I can't make any promises about it working with your specific carrier. Confirmed working: T-Mobile, Rogers, Cricket, Ting. Likely not to work: Sprint (has requirements about specific carrier app being on phone to work), Project Fi.
#### Why is this project so closely tied to AWS?
Building AOSP and Chromium requires a fairly powerful server, which is not something everyone readily has access to. Using a cloud provider allows you to spin up compute resources capable of building these projects and only paying for the time you use them. It could really be any cloud provider, but just happened to choose AWS. There are pros and cons to building AOSP in the cloud. On the positive side, cloud providers allow you to easily write automation that can spin up and down resources as needed which allows rattlesnakeos-stack to automate the entire process of building an Android OS and distributing OTA updates. On the downsides, for those that are very security conscious, they may be wary of building an OS on shared cloud resources. You can checkout the [security section of the FAQ](#security) for more details on this.
#### Will you support other devices?
It's not likely that other devices will be supported beyond the Pixel line. Here are some of the reasons:
* Support for verified boot with a locked bootloader is a requirement for this project. The Pixel line of phones are fairly unique in that they support verified boot with custom signing keys while locking the bootloader with an alternative OS. I am sure there are/will be devices that support this but I don't know of any others.
* Being able to get monthly AOSP security updates for a device is a requirement for this project. Google provides proper AOSP releases every month for Pixel devices which makes it very simple to build and stay up to date with monthly security updates - most vendors don't provide this.
* Being able to get monthly firmware and driver updates is a requirement for this project. Google provides updated firmware and drivers for Pixel devices every month (although incomplete - the vendor specific code ends up being extracted from monthly updated factory images) - regardless most vendors don't provide this.
* Even if there is another device that meets these requirements, the build process would likely differ enough that supporting it would be too much overhead. The current build differences between each Pixel device is relatively minor.
#### Is this a fork of CopperheadOS?
No. RattlesnakeOS was created initially as an alternative to [CopperheadOS](https://en.wikipedia.org/wiki/CopperheadOS), a security hardened Android OS created by [Daniel Micay](https://twitter.com/DanielMicay), after it stopped being properly maintained back in June 2018. To be clear, this project is not attempting to add or recreate any of the security hardening features that were present in CopperheadOS. If you are interested in the continuation of the CopperheadOS project you can check out [GrapheneOS](https://grapheneos.org/).
#### Why is my webview not working?
* If you are using work profiles, it is a known issue and it has a [workaround](https://support.google.com/work/android/answer/7506908?hl=en).

### Costs
#### How much does this cost to run?
The costs are going to be variable by AWS region and by day and time you are running your builds as spot instances have a variable price depending on market demand. Below is an example scenario that should give you a rough estimate of costs:
   * The majority of the cost will come from builds on EC2. It currently launches spot instances of type c5.4xlarge which average maybe $.30 an hour in us-west-2 (will vary by region) but can get up over $1 an hour depending on the day and time. You can modify the default `max-price` config value to set the max price you are willing to pay and if market price exceeds that then your instance will be terminated. Builds can take anywhere from 2-6 hours depending on if Chromium needs to be built. So let's say you're doing a build every month at $0.50 an hour and it is taking on average 4 hours - you'd pay ~$2 in EC2 costs per month. 
   * The other very minimal cost would be S3. Storage costs are almost non existent as a stack will only store about 3GB worth of files (factory image, ota file, target file) and at $0.023 per GB you're looking at $0.07 per month in S3 storage costs. The other S3 cost would be for data transfer out for OTA updates - let's say you are just downloading an update per month (~500MB file) at $0.09 per GB you're looking at $0.05 per month in S3 network costs.
   * If you are running the optional attestation server, the costs will be around $3-$5/month. This cost comes from an EC2 spot instance and EBS volumes it uses. You can modify the default `attestation-max-price` to a lower value if you want to try to reduce costs further, but if you go to low you may never launch an instance.

### Builds
#### How do I change build frequency?
By default it is configured to automatically build once a month on the 10th of the month so that monthly security updates can be picked up and built without the need for manual builds. There is a config option to specify how frequently builds are kicked off automatically. For example you could set `schedule = "rate(14 days)"` in the config file to build every 14 days. Also note, the default behavior is to only run a build if there have been version updates in stack, AOSP, Chromium, or F-Droid versions.
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
#### How can I debug build issues?
There is a flag you can pass `rattlesnakeos-stack` called `--prevent-shutdown` that will prevent the EC2 instance from terminating so that you can SSH into the instance and debug. Note that this will keep the instance online for 12 hours or until you manually terminate it.
#### Why did my EC2 instance randomly terminate?
If there wasn't an error notification, this is likely because the [Spot Instance](https://aws.amazon.com/ec2/spot/) max price was not high enough or EC2 is low on capacity and needs to reclaim instances. You can see historical spot instance pricing in the [EC2 console](https://console.aws.amazon.com/ec2sp/v1/spot/home). Click `Pricing History`, select c5.4xlarge for `Instance Type` and pick a date range. I would recommend not setting your `max-price` beyond the on demand price.

### Remote Attestation
#### What is remote attestation?
You can read more about remote attestation [here](https://attestation.app/about). 

#### How does the remote attestation feature work?
There are two primary pieces involved:
* [AttestationServer](https://github.com/GrapheneOS/AttestationServer) - this is the server component used for remote attestation and it is deployed as an [Elastic Beanstalk](https://aws.amazon.com/elasticbeanstalk/) Docker application running on a spot instance to keep costs to a minimum. A number of things are automated here like: SSL certificate generation with Letsencrypt, backups of sqlite database to S3, customization of app to use your personal keys, etc.
* [Auditor](https://github.com/GrapheneOS/Auditor) - this app is built with your keys and updated to point at your personal AttestationServer.

Steps to setup:
* On your computer, set `attestation-server = true` option in your config and `deploy`. This will setup all of the AWS infrastructure for running AttestationServer, but AttestationServer code isn't deployed yet. Note: you should get a few emails about the app being setup and an instance launching.
* On your computer, start a RattlesnakeOS build with the `build` command. As part of the build process AttestationServer is deployed (should get more emails) and the Auditor app will be built and included into your resulting RattlesnakeOS build.
* Install the resulting OS build on your phone.
* In emails (or within the Auditor app) you should have seen a URL that looks like https://attestation.azmgdmnqbn.us-west-2.elasticbeanstalk.com; visit this in your computer browser, create an account and don't lose the username/password as this has been locked down to only a single user registration.
* On your phone, in the Auditor app click the button in the top right, 'Enable remote verification', and then scan the barcode.

### Customizations
#### How do I customize RattlesnakeOS builds?
There are some advanced options that allow you to customize RattlesnakeOS builds to your liking by adding additional patches and prebuilt apps. These can only be setup in the config file and not through CLI flags.

<b>Important: using any Git repo here that is not in your control is a security risk, as you are giving control of your build process to the owner of the repo. They could steal your signing keys, inject malicious code, etc just by updating a patch file.</b>

##### Patches and Scripts
There is an option to execute patches and shell scripts against the AOSP build tree using `[[custom-patches]]` in the config file. This requires you provide a Git repo and a list of patches you want to apply during the build process. [There is a repo of useful patches that have been contributed by the community](https://github.com/RattlesnakeOS/community_patches) that are trusted and can be used here - or you could use your own if you wanted.

```toml
[[custom-patches]]
  repo = "https://github.com/RattlesnakeOS/community_patches"
  patches = [
      "00001-global-internet-permission-toggle.patch", "00002-global-sensors-permission-toggle.patch",
  ]

[[custom-scripts]]
  repo = "https://github.com/RattlesnakeOS/example_patch_shellscript"
  scripts = [ "00002-custom-boot-animation.sh" ]
```

##### Prebuilts
There is also an option to add prebuilt applications to the AOSP build tree using `[[custom-prebuilts]]` in the config file. This requires you provide a git repo and a list of module names defined in Android.mk files within this repository that you want to have included in the build.
```toml
[[custom-prebuilts]]
  modules = ["app1", "app2"]
  repo = "https://github.com/RattlesnakeOS/example_prebuilts"
```

##### Manifest Customizations
It's also possible to add remotes and projects to the AOSP build manifest file. These will get added to the manifest and get pulled into the AOSP build tree as part of normal build process.

```toml
# to add a remote line to manifest like this: <remote name="rattlesnakeos" fetch="https://github.com/RattlesnakeOS/" revision="master" />
[[custom-manifest-remotes]]
  name = "rattlesnakeos"
  fetch = "https://github.com/RattlesnakeOS/"
  revision = "master"

# to add a project line to manifest like this: <project path="packages/apps/Backup" name="backup" remote="rattlesnakeos" />
# you can also add modules here that you want to include into the build process
# in this example it is adding a really great backup app (https://github.com/stevesoltys/backup)
[[custom-manifest-projects]]
  path = "packages/apps/Backup"
  name = "backup"
  remote = "rattlesnakeos"
  modules = [ "Backup" ]
```

#### Can I change the boot animation?
It is possible to change the boot animation using patches, there is an example repo [here](https://github.com/RattlesnakeOS/example_patch_shellscript).
#### Can I add microG to the build?
I don't recommend installing microG as it requires you to enable signature spoofing. By enabling signature spoofing, this is a global change to the OS even though it has to be requested by each application as a permission. Just having the possibility for an application to request this ability reduces security of your OS. Having said all that, if you are fine with the security implications of doing so - it is possible to install microG using the custom patches and prebuilts features. See the [microG community supported repo](https://github.com/RattlesnakeOS/microg) for details on how to do this.

### Security
#### How secure is this?
Your abilty to secure your signing keys determines how secure RattlesnakeOS is. RattlesnakeOS generates and stores signing keys, optionally encrypted, in AWS, which means the security of your AWS account becomes critical to ensuring the security of your device. If you aren't able to properly secure your local workstation and your AWS account, then these additional security protections like verified boot become less useful. Using the encrypted signing keys option can reduce impact of an AWS account compromise by keeping signing keys encrypted at rest and only decrypted into memory at build time. Is this infallible? Absolutely not, as your unencrypted keys are going to be in memory on a VM running in the cloud. So if your threat model includes more targeted attacks that try to extract signing keys while they are in memory, then RattlesnakeOS is probably not for you.

Cloud based builds are never going to be as secure as a locally built AOSP signed with highly secured keys generated from an HSM or air gapped computer, so if this is the level of security you require then there really is no other way. Would I recommend cloud builds like this for a large OEM or a company like CopperheadOS where the signing key being generated is protecting thousands of users? No, this becomes a high profile target as getting a hold of these keys essentially gives an attacker access to thousands of devices. On the other hand, for a single user generating their own key protecting a single device, there is less concern in my mind unless your threat profile includes very targeted attacks. 
#### What are some security best practices for AWS accounts?
Some minimimum steps worth considering are having an account solely for building RattlesnakeOS with a strong password, enabling two factor authentication, enabling auditing with CloudTrail, and locking down access to your AWS API credentials.
#### What's the difference between the default option and encrypted signing keys option and what one should I use?
There are different configurations for RattlesnakeOS builds based on your threat model. Here's a breakdown of the two primary build configurations and how they compare:
* Using the standard RattlesnakeOS build process, your keys are autogenerated and stored in S3. This means your AWS account security become the most important part of maintaining secure signing keys. This default build option is a good fit for someone that is OK with putting some trust in AWS, wants hands off builds with no manual intervention, and doesn't want to deal with maintaining a passphrase for encrypting/decrypting signing keys. Even with this setup, this still means AWS has potential access to your signing keys. If your threat model included an attacker compromising your AWS account, then this would not suffice as they would be able to get access to your unencrypted signing keys.
* The encrypted signing keys option allows you to prevent storing signing keys in an unencrypted form within AWS. It does this by using GPG symmetric encryption to store your keys at rest. This means that even AWS or someone that got control of your account wouldn't be able to extract your signing keys assuming the passphrase used to encrypt them was strong enough to prevent a brute force attack. Using this option puts less trust in AWS and more trust in your ability to secure the passphrase used for encrypting/decrypting your signing keys.
#### How does the encrypted signing keys option work in practice?
When using the encrypted signing keys option - the workflow is not fully automated like the standard build process. It requires a user to provide a passphrase to encrypt/decrypt signing keys to be used during the build process. The general workflow looks like this:
* Stack is deployed with config option `encrypted-keys = true`.
* When a build starts, an email notification will be sent that your EC2 instance is waiting for a passphrase - or will timeout in 10 mins and terminate the build. This email notification will give you an SSH command to run to provide your passphrase to the build process running on an EC2 instance. If this is your first build, encrypted signing keys don't exist yet in S3, and this passphrase will be used to store newly generated signing keys in encrypted form in S3. On future builds, these encrypted signing keys will be detected and the email notification you get to provide your passphrase to the build process will be used to decrypt your signing keys for use in the build signing process.
* Build continues as usual
#### How do I migrate to using the encrypted signing key option?
If you have an existing stack and want to move to encrypted signing keys you'll need to migrate your keys. Note: if you don't do this migration process new signing keys will be generated during the build process and you'll need to flash a new factory image (losing all data) to be able to use these builds.
* First you'll need to update your stack config file to use `encrypted-keys = true` and then run `rattlesnakeos-stack deploy` to update your stack. 
* Next you'll need to copy your existing signing keys from S3 bucket `<rattlesnakeos-stackname>-keys`, encrypt them with GPG using a strong passphrase, and then copy over encrypted keys to S3 encrypted keys bucket `<rattlesnakeos-stackname>-keys-encrypted`.
    ```sh
    mkdir -p key-migration && cd key-migration
    aws s3 sync s3://<rattlesnakeos-stackname>-keys/ .
    echo -n "Encryption passphrase: "
    read -s key
    echo
    for f in $(find . -type f); do 
        gpg --symmetric --batch --passphrase "${key}" --cipher-algo AES256 $f
    done
    aws s3 sync . s3://<rattlesnakeos-stackname>-keys-encrypted/ --exclude "*" --include "*.gpg"
    ```
* After running a full build and updating your device, you can remove the keys from the original `s3://<rattlesnakeos-stackname>-keys` bucket.

## Uninstalling
### Remove AWS resources
If you decide this isn't for you and you want to remove all the provisioned AWS resources, there's a command for that. 

```sh
./rattlesnakeos-stack remove --name <rattlesnakeos-stackname> --region us-west-2
```

<b>Important note</b>: this will not terminate any running EC2 instances that may have launched and these will need to be terminated manually.

### Revert back to stock Android
For Pixel and Pixel XL, just unlock your bootloader and flash stock factory image.

For newer devices, you'll need to clear the configured AVB public key after unlocking the bootloader and before locking it again with the stock factory images.

```sh
fastboot erase avb_custom_key
```

## Donations
* [Liberapay](https://liberapay.com/rattlesnakeos/)
* [Bitcoin](https://www.blockchain.com/btc/address/17GHmnK3fyw9TBngvaM8Veh37UR65rmvZS)

## Powered by
* [android-prepare-vendor](https://github.com/anestisb/android-prepare-vendor)
* [GrapheneOS](https://github.com/GrapheneOS)
* [Terraform](https://www.terraform.io/)
* Huimin Zhang - author of the original underlying build script that was written for CopperheadOS.

## Build from Source
 * To compile from source you'll need to install Go (https://golang.org/) for your platform
  ```sh
  go get github.com/dan-v/rattlesnakeos-stack
  cd $GOPATH/src/github.com/dan-v/rattlesnakeos-stack/
  make tools
  make
  ```
