## What is RattlesnakeOS
RattlesnakeOS is privacy focused Android OS based on [AOSP](https://source.android.com/) for Google Pixel phones. It is my migration strategy away from a security hardened OS called [CopperheadOS](https://en.wikipedia.org/wiki/CopperheadOS) that is no longer maintained. RattlesnakeOS is just stock AOSP with a few things from CopperheadOS: [verified boot](https://source.android.com/security/verifiedboot/) with your own keys, latest Chromium ([webview](https://www.chromium.org/developers/how-tos/build-instructions-android-webview) + browser), [F-Droid](https://f-droid.org/) (with [priviledge extension](https://gitlab.com/fdroid/privileged-extension)), no Google apps, and OTA updates.

## What is rattlesnakeos-stack
Rather than providing random binaries of RattlesnakeOS to install on your phone, I've gone the route of creating a cross platform tool, `rattlesnakeos-stack`, that provisions all of the [AWS](https://aws.amazon.com/) infrastructure needed to automatically build your own RattlesnakeOS on a regular basis, with your own signing keys, and your own OTA updates. It uses [AWS Lambda](https://aws.amazon.com/lambda/features/) to provision [EC2 Spot Instances](https://aws.amazon.com/ec2/spot/) that build RattlesnakeOS and upload build artifacts to [S3](https://aws.amazon.com/s3/). Resulting OS builds are configured to receive over the air updates from this environment.

## Features
* Support for <b>Google Pixel, Pixel XL, Pixel 2 XL</b>
* Untested support for Google Pixel 2 
* Updates and monthly security fixes delivered through OTA updates - no need to manually flash your device
* Maintain [verified boot](https://source.android.com/security/verifiedboot/) with a locked bootloader just like official Android but with your own personal signing keys
* Latest Chromium [browser](https://www.chromium.org) and [webview](https://www.chromium.org/developers/how-tos/build-instructions-android-webview) with patches from [Bromite](https://github.com/bromite/bromite) for ad blocking and enhanced privacy
* Latest [F-Droid](https://f-droid.org/) client and [priviledge extension](https://gitlab.com/fdroid/privileged-extension)
* No Google apps pre-installed
* Full end to end setup of build environment for RattlesnakeOS in AWS
* Costs a few dollars a month to run (EC2 spot instance and S3 storage costs)

## Carrier Support
I only have access to a single device and carrier to test this on, so I can't make any promises about it working with your specific carrier. I'll try to keep this updated with any confirmed working carriers and devices that I've seen posted:
### Working:
* T-Mobile (USA): Pixel XL
### Likely to not work:
* Sprint (has requirements about specific carrier app being on phone to work)

## Installation
The easiest way is to download a pre-built binary from the [Github Releases](https://github.com/dan-v/rattlesnakeos-stack/releases) page.

## Prerequisites
* An AWS account - you can [create an AWS account](https://portal.aws.amazon.com/billing/signup) if you don't have one. 
  * <b>If this is a new AWS account, make sure you launch at least once paid instance before running through these steps.</b>  To do this you can navigate to the [EC2 console](https://us-west-2.console.aws.amazon.com/ec2/), click `Launch instance`, select any OS, pick a `c4.4xlarge`, and click `Review and launch`. After it launches you can terminate the instance through the console.
* You'll need AWS credentials with `AdministratorAccess` access. If you're not sure how to do that, you can follow [this step by step guide](https://serverless-stack.com/chapters/create-an-iam-user.html).
* Install the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/installing.html) for your platform and [configure](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html) it to use these credentials.

## Deployment
Pick a name for your stack and replace 'rattlesnakeos-\<yourstackname>' with your own name. <b>Note: this name has to be unique or it will fail to provision.</b>
* Deploy environment for Pixel XL (marlin)

    ```sh
    ./rattlesnakeos-stack --region us-west-2 --name rattlesnakeos-<yourstackname> --device marlin
    ```

* Deploy environment for Pixel (sailfish)

    ```sh
    ./rattlesnakeos-stack --region us-west-2 --name rattlesnakeos-<yourstackname> --device sailfish
    ```

* Deploy environment for Pixel 2 XL (taimen)

    ```sh
    ./rattlesnakeos-stack --region us-west-2 --name rattlesnakeos-<yourstackname> --device taimen
    ```

* Deploy environment for Pixel 2 (walleye)

    ```sh
    ./rattlesnakeos-stack --region us-west-2 --name rattlesnakeos-<yourstackname> --device walleye
    ```

* If you decide this isn't for you and you want to remove all the provisioned AWS resources, there's a command for that. Note: if you've already done a build, you'll need to manually remove all of the files from S3 buckets.

    ```sh
    ./rattlesnakeos-stack --remove --region us-west-2 --name rattlesnakeos-<yourstackname>
    ```

## First Time Setup After Deployment
* Setup email notifications for builds:
  * Go to the [AWS SNS](https://us-west-2.console.aws.amazon.com/sns/v2/home?region=us-west-2#/topics) console
  * Click on the topic named `rattlesnakeos-<yourstackname>`
  * Click on `Create subscription` button
  * In `Create subscription` dialog, in `Protocol` dropdown select `Email`
  * For `Endpoint`, enter your email address
  * Click `Create subscription` button
  * You should get an email link that you need to click in order to subscribe to messages in this topic
* After initial setup with `rattlesnakeos-stack` tool, a build should have automatically kicked off. You can check this by going to the [EC2 console](https://us-west-2.console.aws.amazon.com/ec2/v2/home) and verifying there is an EC2 instance running. If a build hasn't kicked off, check out the FAQ for how to manually start a build.
* The <b>initial build will likely take 5+ hours to complete</b>. 
* After the build finishes, a factory image should be uploaded to the S3 bucket that you can download:
  * Go to the [S3 console](https://s3.console.aws.amazon.com/s3/buckets/)
  * Click on `rattlesnakeos-<yourstackname>-release` bucket.
  * From this bucket, download the file `<device>-factory-latest.tar.xz`
* Use this factory image and [follow the instructions on flashing your device carefully](FLASHING.md).
* You followed the instructions until the end and you re-locked your bootloader and disabled OEM unlocking after flashing right? If not, go do that!
* After successfully flashing your device, you will now be running RattlesnakeOS and all future updates will happen through the built in OTA updater.

## How to update rattlesnakeos-stack
* Just download the new version of rattlesnakeos-stack and run the same command used previously (e.g. `rattlesnakeos-stack --region us-west-2 --name rattlesnakeos-<yourstackname> --device marlin`) to apply the updates

## FAQ
1. <b>Should I use rattlesnakeos-stack?</b> Use at your own risk.
2. <b>How much does this cost?</b> The costs are going to be variable by AWS region and by day and time you are running your builds as spot instances have a variable price depending on market demand. Below is an example scenario that should give you a rough estimate of costs:
   * The majority of the cost will come from builds on EC2. It currently launches spot instances of type c4.4xlarge which average maybe $.30 an hour in us-west-2 (will vary by region) but can get up over $1 an hour depending on the day and time. The `rattlesnakeos-stack` tool allows you define a maximum bid price (`--spot-price`) you are willing to pay and if market price exceeds that then your instance will be terminated. Builds can take anywhere from 2-6 hours depending on if Chromium needs to be built. So let's say you're doing a weekly build at $0.50 an hour and it is taking on average 4 hours - you'd pay ~$8 in EC2 costs per month. You could reduce this to a monthly build (see section how to change build frequency) and then you'd be looking at ~$2 in EC2 costs per month.
   * The other very minimal cost would be S3. Storage costs are almost non existent as a stack will only store about 3GB worth of files (factory image, ota file, target file) and at $0.023 per GB you're looking at $0.07 per month in S3 storage costs. The other S3 cost would be for data transfer out for OTA updates - let's say you are just downloading an update per week (~500MB file) at $0.09 per GB you're looking at $0.20 per month in S3 network costs.
3. <b>How do I change build frequency?</b> The current default is to do builds on a weekly basis. With `rattlesnakeos-stack` tool there is an option to specify how frequently builds are kicked off with option `--schedule`. For example you could set `--schedule "rate(30 days)"` to only build every 30 days. Also note, the default behavior is to only run a build if there have been version updates in AOSP build, Chromium version, or F-Droid versions.
4. <b>How do I manually start a build?</b>
   * Go to the [AWS Lambda](https://us-west-2.console.aws.amazon.com/lambda/) console
   * Click on the function named 'rattlesnakeos-\<yourstackname>-build'
   * Click on the 'Test' button
   * In 'Configure test event dialog', set event name to 'rattlesnakeos', keep the defaults, and click 'Create' button.
   * Click the 'Test' button again to kick off the build
5. <b>Where do I find logs for a build?</b> On build failure/success, the instance should terminate and upload its logs to S3 bucket called `<stackname>-logs` and it's in a file called `<device>/<timestamp>`.
6. <b>How can I connect to the EC2 instance and see the build status?</b> There are a few steps required to be able to do this:
   * Create an SSH keypair in the [EC2 console](https://us-west-2.console.aws.amazon.com/ec2/v2/home?region=us-west-2#KeyPairs:sort=keyName) and download it
   * Pass an additional flag to `rattlesnakeos-stack` command: `--ssh-key yourkeypairname`
   * Kick off a manual build through AWS Lambda console (see FAQ above)
   * In the default security group, you'll need to open up SSH access
   * You should be able to SSH into the instance: `ssh -i yourkeypairname.pem ubuntu@yourinstancepublicip`
   * Tail the cloud init logfile to view progress: `tail -f /var/log/cloud-init-output.log`
7. <b>How can I prevent the EC2 instance from immediately terminating on error so I can debug?</b> There is a flag you can pass `rattlesnakeos-stack` called `--prevent-shutdown`. Note that this will keep the instance online for 12 hours or until you manually terminate it.
8. <b>Why did my EC2 instance randomly terminate?</b> If there wasn't an error notification, this is likely because the [Spot Instance](https://aws.amazon.com/ec2/spot/) bid was not high enough at this specific time. You can see historical spot instance pricing in the [EC2 console](https://console.aws.amazon.com/ec2sp/v1/spot/home). Click `Pricing History`, select c4.4xlarge for `Instance Type` and pick a date range. If you want to avoid having your instance terminated, you can pass an additional flag to `rattlesnakeos-stack` with a higher than default bid: `--spot-price 1.50`
9. <b>How do OTA updates work?</b> If you go to `Settings->System update settings` you'll see the updater app settings. The updater app will ping S3 to see if there are updates and if it finds one will download and apply it your device. There is no progress indicator unfortunately - you'll just got a notification when it's done and it will ask you to reboot. If you want to force a check for OTA updates, you can toggle the `Require battery above warning level` setting and it will check for a new build in your S3 bucket.

## Powered by
* Huimin Zhang - he is the original author of the underlying build script that was written for CopperheadOS.
* [Terraform](https://www.terraform.io/) 

## Build from Source

  ```sh
  make tools && make
  ```

## To Do
* Restrict created IAM roles to minimum required privileges (currently all admin)