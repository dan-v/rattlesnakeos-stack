## Initial Flashing Guide
Below are instructions on how to initially flash your device to run RattlesnakeOS. This is a one time process and future updates will happen through the built in OTA updater. Please note most of this guide is directly copied from [GrapheneOS documentation](https://grapheneos.org/install). 

## Prerequisites
You should have at least 2GB of free memory available.

You need the unlocked variant of one of the supported devices, not a locked carrier specific variant.

It's best practice to update the stock OS on the device to make sure it's running the latest firmware before proceeding with these instructions. This avoids running into bugs in older firmware versions. It's known that the early Pixel 2 and Pixel 2 XL bootloader versions have weird quirks with unlocking. There aren't known issues on other devices, but this is still a good idea. You can either do this via over-the-air updates or sideload a full update from their full update package page.

## Obtaining fastboot
You need an updated copy of the fastboot tool and it needs to be included in your PATH environment variable. You can run fastboot --version to determine the current version. It should be at least 28.0.0. You can use a distribution package for this, but most of them mistakenly package development snapshots of fastboot, clobber the standard version scheme for platform-tools (adb, fastboot, etc.) with their own scheme and don't keep it up-to-date despite that being crucial.

If your distribution doesn't have a proper fastboot package, which is likely, consider using the official releases of platform-tools from Google. You can either obtain these as part of the standalone SDK or Android Studio which are self-updating or via the standalone platform-tools releases. For one time usage, it's easiest to obtain the latest standalone platform-tools release, extract it and add it to your PATH in the current shell. For example:

```
unzip platform-tools_r29.0.2-linux.zip
export PATH="$PWD/platform-tools:$PATH"
```

Sample output from `fastboot --version` afterwards:
```
fastboot version 29.0.2-5738569
Installed as /home/username/downloads/platform-tools/fastboot
```

Don't proceed with the installation process until this is set up properly in your current shell. A very common mistake is using an outdated copy of fastboot from a Linux distribution package not receiving regular updates. Make sure that the fastboot found earliest in your PATH is the correct one if you have multiple copies on your system. The fastboot --version output includes the installation path for the copy of fastboot that's being used. Older versions of fastboot do not have support for current devices and OS versions. Very old versions of fastboot from are still shipped by Linux distributions like Debian and lack the compatibility detection of modern versions so they can soft brick devices.

## Enabling OEM unlocking
OEM unlocking needs to be enabled from within the operating system.

Enable the developer settings menu by going to `Settings ➔ System ➔ About` phone and pressing on the build number menu entry until developer mode is enabled.

Next, go to `Settings ➔ System ➔ Advanced ➔ Developer settings` and toggle on the `Enable OEM unlocking` setting. This requires internet access on devices with Google Play Services as part of Factory Reset Protection (FRP) for anti-theft protection.

## Unlocking the bootloader
First, boot into the bootloader interface. You can do this by turning off the device and then turning it on by holding both the Volume Down and Power buttons.

The bootloader now needs to be unlocked to allow flashing new images:
```
fastboot flashing unlock
```

The command needs to be confirmed on the device.

## Obtaining factory images
Extract the factory image you just created for RattlesnakeOS and run the script to flash them. 
```
tar xvf crosshatch-factory-latest.tar.xz
cd crosshatch-pq3a.190605.003
./flash-all.sh
```
Wait for the flashing process to complete and for the device to boot up using the new operating system.

You should now proceed to setting custom AVB key and locking the bootloader before using the device as locking wipes the data again.

## Setting custom AVB key
On the <b>Pixel 1 and Pixel 1 XL</b>, a locked bootloader will blindly accept whatever public key is embedded in the image.  It will display a yellow screen if an unknown public key is used, and skip the warning if Google's key is used.  This provides only limited protection from tampering when running custom images.  On these units, there is no way to set a custom AVB key.

On newer Pixel devices (everything other than the original Pixel), a locked bootloader will verify the signing key to make sure it matches the same key set by the user (or Google's master key).  If the image was signed using the avb\_custom\_key, a yellow warning screen will be displayed. If the image was signed with Google's key, no warning will be shown.  If the image was signed with an unrecognized key, the device will refuse to boot.

Therefore, the public key needs to be set before locking the bootloader again. First get the generated public key from S3:
```
aws s3 cp s3://<rattlesnakeos-stackname>-keys/crosshatch/avb_pkmd.bin .
```

Now use fastboot to flash the avb_custom_key
```
fastboot flash avb_custom_key avb_pkmd.bin
```

## Locking the bootloader
Locking the bootloader is important as it enables full verified boot. It also prevents using fastboot to flash, format or erase partitions. Verified boot will detect modifications to any of the OS partitions (vbmeta, boot/dtbo, product, system, vendor) and it will prevent reading any modified / corrupted data. If changes are detected, error correction data is used to attempt to obtain the original data at which point it's verified again which makes verified boot robust to non-malicious corruption.

In the bootloader interface, set it to locked:
```
fastboot flashing lock
```

The command needs to be confirmed on the device since it needs to perform a factory reset.

Unlocking the bootloader again will perform a factory reset.

## Disable OEM unlocking
OEM unlocking can be disabled now from within the operating system.

Enable the developer settings menu by going to `Settings -> About device` and pressing on the build number menu entry until developer mode is enabled.

Next, go to `Settings -> Developer` settings and toggle off the `Enable OEM unlocking` setting.

<b>IMPORTANT: disabling OEM unlocking does significantly increase the security of your device by not allowing someone with physical access to unlock your bootloader and reset your device, but it also increases potential for bricking your phone. So it is up to you to determine if the extra security is worth the risk. In the locked state, your bootloader will only accept OTA updates signed by your custom key. This means if your device ever got into a non booting state with your bootloader locked and OEM unlocking disabled, the only way to fix it would be generating a booting OTA update signed with your keys and applied in recovery mode. So for example, if you lost your signing keys or just weren't able to generate a new booting OTA update for some reason, your device would be bricked.</b>