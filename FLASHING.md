## Initial Flashing Guide
Below are instructions on how to initially flash your device to run RattlesnakeOS. This is a one time process and future updates will happen through the built in OTA updater. Please note most of this guide is directly copied from [CopperheadOS documentation](https://copperhead.co/android/docs/install). 

## Prerequisites
You should have at least 4GB of memory on your OS to avoid problems.

You can obtain the adb and fastboot tools from the Android SDK. Either install Android Studio or use the standalone SDK. Do not use distribution packages for adb and fastboot. Distribution packages are out-of-date and not compatible with the latest version of Android. <b>An obsolete fastboot will result in corrupted installations and potentially bricked devices</b>. Do not make the common mistake of assuming that everything will be fine and ignoring these instructions. Double check that the first fastboot in your PATH is indeed from an up-to-date SDK installation:

```
which fastboot
```

To set up a minimal SDK installation without Android Studio on Linux:
```
mkdir ~/sdk
cd ~/sdk
wget https://dl.google.com/android/repository/sdk-tools-linux-3859397.zip
unzip sdk-tools-linux-3859397.zip
```

Run an initial update, which will also install platform-tools and patcher;v4:
```
tools/bin/sdkmanager --update
```

To make your life easier, add the directories to your PATH in your shell profile configuration:
```
export PATH="$HOME/sdk/tools:$HOME/sdk/tools/bin:$HOME/sdk/platform-tools:$HOME/sdk/build-tools/25.0.3:$PATH"
export ANDROID_HOME="$HOME/sdk"
```

You should update the sdk before use from this point onwards:

sdkmanager --update

## Enabling OEM unlocking
OEM unlocking needs to be enabled from within the operating system.

Enable the developer settings menu by going to `Settings -> About device` and pressing on the build number menu entry until developer mode is enabled.

Next, go to `Settings -> Developer` settings and toggle on the `Enable OEM unlocking` setting.

## Updating stock before using fastboot
It’s important to have the latest bootloader firmware before installing RattlesnakeOS.

If you’re only behind one release, updating within the stock OS makes sense to get an incremental update. If you’re behind multiple releases, updating within the OS will usually require installing multiple updates to catch up to the current state of things. The quickest way to deal with that if you have plenty of bandwidth is sideloading the latest full over-the-air update from Google.

## Flashing factory images
The initial install should be performed by flashing the factory images. This will replace the existing OS installation and wipe all the existing data. First, boot into the bootloader interface. You can do this by turning off the device and then turning it on by holding both the Volume Down and Power buttons. Alternatively, you can use adb reboot bootloader from Android.

The bootloader now needs to be unlocked to allow flashing new images:

```
fastboot flashing unlock
```

On the <b>Pixel 2 XL (not the Pixel 2 or other devices)</b>, it’s currently necessary to unlock the critical partitions, but a future update will make the bootloader consistent with other devices:

```
fastboot flashing unlock_critical
```

The command needs to be confirmed on the device.

Next, extract the factory images and run the script to flash them. 
```
tar xvf taimen-factory-latest.tar.xz
cd taimen-opm1.171019.018
mkdir tmp
TMPDIR=$PWD/tmp ./flash-all.sh
```

## Setting custom AVB key
On the <b>Pixel 1 and Pixel 1 XL</b>, a locked bootloader will blindly accept whatever public key is embedded in the image.  It will display a yellow screen if an unknown public key is used, and skip the warning if Google's key is used.  This provides only limited protection from tampering when running custom images.  On these units, there is no way to set a custom AVB key.

On newer Pixel devices, a locked bootloader will verify the signing key to make sure it matches the same key set by the user (or Google's master key).  If the image was signed using the avb\_custom\_key, a yellow warning screen will be displayed. If the image was signed with Google's key, no warning will be shown.  If the image was signed with an unrecognized key, the device will refuse to boot.

Therefore, the public key needs to be set before locking the bootloader again.  The procedure for doing so is:
```
aws s3 cp s3://<rattlesnakeos-stackname>-keys/taimen/avb_pkmd.bin .
fastboot flash avb_custom_key avb_pkmd.bin
```

If you used `--encrypted-keys`, you will need to download the key from `s3://<rattlesnakeos-stackname>-keys-encrypted` and decrypt it manually.

To confirm that the key is set, verify that `avb_user_settable_key_set` is yes:
```
fastboot getvar avb_user_settable_key_set
```

## Locking the bootloader
Locking the bootloader is important as it enables full verified boot. It also prevents using fastboot to flash, format or erase partitions. Verified boot will detect modifications to any of the OS partitions (boot, recovery, system, vendor) and it will prevent reading any modified / corrupted data. If changes are detected, error correction data is used to attempt to obtain the original data at which point it’s verified again which makes verified boot robust to non-malicious corruption.

On the <b>Pixel 2 XL (not the Pixel 2 or other devices)</b>, lock the critical partitions again if this was unlocked:

```
fastboot flashing lock_critical
```

Reboot into the bootloader menu and set it to locked:
```
fastboot flashing lock
```

The command needs to be confirmed on the device since it needs to perform a factory reset.

Unlocking the bootloader again will perform a factory reset.

## Disable OEM unlocking
OEM unlocking needs to be disabled now from within the operating system.

Enable the developer settings menu by going to `Settings -> About device` and pressing on the build number menu entry until developer mode is enabled.

Next, go to `Settings -> Developer` settings and toggle off the `Enable OEM unlocking` setting.
