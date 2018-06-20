## What is RattlesnakeOS
RattlesnakeOS is an Android ROM based on [AOSP](https://source.android.com/). It is my migration strategy away from [CopperheadOS](https://en.wikipedia.org/wiki/CopperheadOS). It is stock AOSP with a few things from CopperheadOS: [verified boot](https://source.android.com/security/verifiedboot/), Chromium (webview + browser), F-Droid (with priviledge extension), and OTA updates. This unfortunately doesn't include any of the hardening features from CopperheadOS.

## What is rattlesnakeos-stack
Rather than providing random binaries of RattlesnakeOS to install on your phone, I've gone the route of creating a cross platform tool, `rattlesnakeos-stack`, that provisions all of the [AWS](https://aws.amazon.com/) infrastructure needed to automatically build your own RattlesnakeOS on a regular basis, with your own signing keys, and your own OTA updates. It uses [AWS Lambda](https://aws.amazon.com/lambda/features/) to provision [EC2 Spot Instances](https://aws.amazon.com/ec2/spot/) that build RattlesnakeOS and upload build artifacts to [S3](https://aws.amazon.com/s3/). Resulting OS builds are configured to receive over the air updates from this environment.

## Features
* Support for Google Pixel XL
* Untested support for Google Pixel, Pixel 2, and Pixel 2 XL
* End to end setup of build environment for RattlesnakeOS in AWS
* Scheduled builds kicked off through AWS Lambda at regular interval
* OTA updates through built in updater app - no need to manually flash your device on each new release
* Costs a few dollars a month to run (EC2 spot instance and S3 storage costs)

## Installation
The easiest way is to download a pre-built binary from the [Github Releases](https://github.com/dan-v/rattlesnakeos-stack/releases) page.

## Prerequisites
* An AWS account - you can [create an AWS account](https://portal.aws.amazon.com/billing/signup) if you don't have one.
* [AWS CLI credentials configured with 'AdministratorAccess'](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html)

## Deployment
Pick a name for your stack and replace 'rattlesnakeos-\<yourstackname>' with your own name. <b>Note this name has to be unique or it will fail to provision.</b>
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
  * Go to [AWS SNS](https://us-west-2.console.aws.amazon.com/sns/v2/home?region=us-west-2#/topics) console
  * Click on the topic named 'rattlesnakeos-\<yourstackname>'
  * Click on 'Create subscription' button
  * In 'Create subscription' dialog, in Protocol dropdown select Email
  * Enter your email for Endpoint
  * Click 'Create subscription button'
  * You should get an email link that you need to click in order to subscribe to messages in this topic
* After initial setup with rattlesnakeos-stack tool, a build should have automatically kicked off. You can check this by going to the [EC2 console](https://us-west-2.console.aws.amazon.com/ec2/v2/home) and verifying there is an EC2 instance running. If a build hasn't kicked off, check out the FAQ for how to manually start a build.
* The initial build will likely take 4+ hours to complete. 
* After the build finishes, a factory image should be uploaded to the S3 bucket that you can download:
  * Go to the [S3 console](https://s3.console.aws.amazon.com/s3/buckets/)
  * Click on 'rattlesnakeos-\<yourstackname>-release' bucket.
  * From this bucket, download the file '\<device>-factory-latest.tar.xz'
* Use this factory image and [follow the instructions on flashing your device](https://copperhead.co/android/docs/install)
* After successfully flashing your device, you will now be running RattlesnakeOS and all future updates will happen through the built in OTA mechanism.

## Updates
* Just download the new version of rattlesnakeos-stack and run the same command used previously (e.g. ./rattlesnakeos-stack --region us-west-2 --name rattlesnakeos-\<yourstackname> --device marlin) to apply the updates

## FAQ
1. <b>Should I use rattlesnakeos-stack?</b> Probably not. Use at your own risk.
2. <b>How do I manually start a build?</b>
  * Go to the [AWS Lambda](https://us-west-2.console.aws.amazon.com/lambda/) console
  * Click on the function named 'rattlesnakeos-\<yourstackname>-build'
  * Click on the 'Test' button
  * In 'Configure test event dialog', set event name to 'rattlesnakeos', keep the defaults, and click 'Create' button.
  * Click the 'Test' button again to kick off the build

## Powered by
* Huimin Zhang - he is the original author of the underlying build script that was written for CopperheadOS.
* [Terraform](https://www.terraform.io/) 

## Build From Source

  ```sh
  make tools && make
  ```

## To Do
* Restrict created IAM roles to minimum required privileges (currently all admin)