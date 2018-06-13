package templates

const ShellScriptTemplate = `
#!/bin/bash

read -rd '' HELP << ENDHELP
Usage: $(basename $0) DEVICE_NAME

Options:
	-A do a full run
ENDHELP

DEVICE=$1

PREVENT_SHUTDOWN=<% .PreventShutdown %>

# AWS config
AWS_KEYS_BUCKET='<% .Name %>-keys'
AWS_RELEASE_BUCKET='<% .Name %>-release'
AWS_LOGS_BUCKET='<% .Name %>-logs'
AWS_SNS_ARN=$(aws --region <% .Region %> sns list-topics --query 'Topics[0].TopicArn' --output text | cut -d":" -f1,2,3,4,5)':<% .Name %>'

# targets
BUILD_TARGET="release aosp_${DEVICE} user"
RELEASE_CHANNEL="${DEVICE}-stable"

BUILD_DIR="$HOME/rattlesnake-os"
CERTIFICATE_SUBJECT='/CN=RattlesnakeOS'
RELEASE_URL="https://${AWS_RELEASE_BUCKET}.s3.amazonaws.com"

OFFICIAL_DATE=$(date +%Y.%m.%d.%H)
OFFICIAL_TIMESTAMP=$(date +%s)
OFFICIAL_VERSION="OPM4.171019.021.D1"

# make getopts ignore $1 since it is $DEVICE
OPTIND=2
FULL_RUN=false
while getopts ":hA" opt; do
  case $opt in
    h)
      echo "${HELP}"
      ;;
    A)
      FULL_RUN=true
      ;;
    \?)
      echo "${HELP}"
      ;;
  esac
done

full_run() {
  aws_notify "Starting RattlesnakeOS Build ($OFFICIAL_DATE)"
  setup_env
  check_chrome
  fetch_build
  setup_vendor
  aws_import_keys
  patch
  build
  aws_release
}

setup_env() {
  ubuntu_setup_packages
  setup_git
  aws_setup_build_dir
}

fetch_build() {
  pushd "${BUILD_DIR}"
  repo init --manifest-url 'https://github.com/dan-v/platform_manifest.git' --manifest-branch "master"
  pushd "${BUILD_DIR}"
  sed -i '/platform_external_chromium/d' .repo/manifest.xml || true
  for i in {1..10}; do
    repo sync --jobs 32 && break
  done
}

patch() {
  patch_manifest
  patch_updater
  patch_priv_ext
}

check_chrome() {
  current=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/revision" - || true)
  echo "Chromium current: $current"

  mkdir -p $HOME/chromium
  cd $HOME/chromium
  git clone https://github.com/AndroidHardening/chromium_patches.git || true
  cd chromium_patches
  git checkout oreo-m2-s3-release
  latest=$(awk /android_default_version_name/'{print $3}' args.gn | cut -d'"' -f2)
  echo "Chromium latest: $latest"

  if [ "$latest" == "$current" ]; then
    echo "Chromium latest ($latest) matches current ($current) - just copying s3 chromium artifact"
    copy_chrome
  else
    echo "Building chromium $latest"
    build_chrome $latest
  fi
}

copy_chrome() {
  aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/MonochromePublic.apk" ${BUILD_DIR}/external/chromium/prebuilt/arm64/
}

build_chrome() {
  CHROMIUM_REVISION=$1
  pushd "$BUILD_DIR" 
  git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git $HOME/depot_tools
  export PATH="$PATH:$HOME/depot_tools"
  mkdir -p $HOME/chromium
  cd $HOME/chromium
  fetch --nohooks android --target_os_only=true
  echo -e "y\n" | gclient sync --with_branch_heads -r $CHROMIUM_REVISION --jobs 32
  cd src
  git am ../chromium_patches/*.patch
  mkdir -p out/Default
  cp ../chromium_patches/args.gn out/Default/args.gn

  build/linux/sysroot_scripts/install-sysroot.py --arch=i386
  build/linux/sysroot_scripts/install-sysroot.py --arch=amd64
  gn gen out/Default
  ninja -C out/Default/ monochrome_public_apk

  cp out/Default/apks/MonochromePublic.apk ${BUILD_DIR}/external/chromium/prebuilt/arm64/
  aws s3 cp "${BUILD_DIR}/external/chromium/prebuilt/arm64/MonochromePublic.apk" "s3://${AWS_RELEASE_BUCKET}/chromium/MonochromePublic.apk" --acl public-read
  echo "${CHROMIUM_REVISION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/chromium/revision" --acl public-read

  rm -rf $HOME/chromium
}

build() {
  pushd "$BUILD_DIR"
  source "${BUILD_DIR}/script/setup.sh"

  choosecombo $BUILD_TARGET
  make -j $(nproc) target-files-package
  make -j $(nproc) brillo_update_payload

  "${BUILD_DIR}/script/release.sh" "$DEVICE"
}

ubuntu_setup_packages() {
  sudo apt-get update
  sudo apt-get --assume-yes install openjdk-8-jdk git-core gnupg flex bison build-essential zip curl zlib1g-dev gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev libxml2-utils xsltproc unzip python-networkx liblz4-tool
  sudo apt-get --assume-yes build-dep "linux-image-$(uname --kernel-release)"
  sudo apt-get --assume-yes install repo gperf jq
}

setup_git() {
  git config --get --global user.name || git config --global user.name 'user'
  git config --get --global user.email || git config --global user.email 'user@localhost'
  git config --global color.ui true
}

aws_setup_build_dir() {
  mkdir --parents "$BUILD_DIR"
}

# call with argument: .x509.pem file
fdpe_hash() {
  keytool -list -printcert -file "$1" | grep 'SHA256:' | tr --delete ':' | cut --delimiter ' ' --fields 3
}

patch_manifest() {
  if [ "$DEVICE" == 'sailfish' ] || [ "$DEVICE" == 'marlin' ]; then
    pushd "$BUILD_DIR"/device/google/marlin
    perl -0777 -i.original -pe 's/ifeq \(\$\(OFFICIAL_BUILD\),true\)\n    PRODUCT_PACKAGES \+= Updater\nendif/PRODUCT_PACKAGES \+= Updater/' device-common.mk  
  fi

  if [ "$DEVICE" == 'walleye' ]; then
    pushd "$BUILD_DIR"/device/google/muskie
    sed -i.original "\$aPRODUCT_PACKAGES += Updater" device-common.mk
  fi

  if [ "$DEVICE" == 'taimen' ]; then
    pushd "$BUILD_DIR"/device/google/taimen
    sed -i.original "\$aPRODUCT_PACKAGES += Updater" device.mk
  fi
}

patch_updater() {
  pushd "$BUILD_DIR"/packages/apps/Updater/res/values
  sed --in-place \
    --expression "s@s3bucket@${RELEASE_URL}@g" config.xml
}

patch_priv_ext() {
  official_sailfish_releasekey_hash='B919FFF979EAC18DF3E65C6D2EBE63F393F11B4BAB344ADE255B2465F49836BC'
  official_sailfish_platform_hash='1C3FBC736E9B7B09E309B8379FF954BF5BD9F95ED399741D7D1D3A42F8ADB757'
  official_marlin_releasekey_hash='6425C9DE6219056CCE62F73E7AD9F92C940B83BAC1D5516ABEBCE1D38F85E4CF'
  official_marlin_platform_hash='CC1E06EAD3E9CA2C4E46073172E92BAD4AFB02D4D21EDDC3F4D9A50C2FBD639D'
  official_taimen_releasekey_hash='12AB56E8D6411DC215448EAC69DFC21AB28164B79DBD3EADD1C70D6A70CD862A'
  official_walleye_releasekey_hash='7CF1C0DD717C52C6EB2B6430E140A586AC5E7652BF0F0D40F428302D735E4CC2'

  unofficial_sailfish_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/sailfish/releasekey.x509.pem")
  unofficial_sailfish_platform_hash=$(fdpe_hash "${BUILD_DIR}/keys/sailfish/platform.x509.pem")
  unofficial_marlin_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/marlin/releasekey.x509.pem")
  unofficial_marlin_platform_hash=$(fdpe_hash "${BUILD_DIR}/keys/marlin/platform.x509.pem")
  unofficial_taimen_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/taimen/releasekey.x509.pem")
  unofficial_walleye_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/walleye/releasekey.x509.pem")

  sed --in-place \
    --expression "s/${official_marlin_releasekey_hash}/${unofficial_marlin_releasekey_hash}/g" \
    --expression "s/${official_marlin_platform_hash}/${unofficial_marlin_platform_hash}/g" \
    --expression "s/${official_sailfish_releasekey_hash}/${unofficial_sailfish_releasekey_hash}/g" \
    --expression "s/${official_sailfish_platform_hash}/${unofficial_sailfish_platform_hash}/g" \
    --expression "s/${official_taimen_releasekey_hash}/${unofficial_taimen_releasekey_hash}/g" \
    --expression "s/${official_walleye_releasekey_hash}/${unofficial_walleye_releasekey_hash}/g" \
    "${BUILD_DIR}/packages/apps/F-Droid/privileged-extension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
}

aws_import_keys() {
  if [ "$(aws s3 ls "s3://${AWS_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
    aws_gen_keys
  else
    mkdir -p "${BUILD_DIR}/keys"
    aws s3 sync "s3://${AWS_KEYS_BUCKET}" "${BUILD_DIR}/keys"
    ln --verbose --symbolic "${BUILD_DIR}/keys/${DEVICE}/verity_user.der.x509" "${BUILD_DIR}/kernel/google/marlin/verity_user.der.x509"
  fi
}

setup_vendor() {
  pushd "${BUILD_DIR}/vendor/android-prepare-vendor"

  sudo apt-get --assume-yes install fuseext2

  patch -p1 <<'ENDDEBUGFSPATCH'
diff --git a/execute-all.sh b/execute-all.sh
index 41c9713..74f3995 100755
--- a/execute-all.sh
+++ b/execute-all.sh
@@ -319,9 +319,9 @@ if isDarwin; then
    SYS_TOOLS+=("umount")
   _UMOUNT=umount
 else
   # For Linux use debugfs
-  USE_DEBUGFS=true
-  # SYS_TOOLS+=("fusermount")
-  # _UMOUNT="fusermount -u"
+  USE_DEBUGFS=false
+  SYS_TOOLS+=("fusermount")
+  _UMOUNT="fusermount -u"
 fi

 # Check that system tools exist
ENDDEBUGFSPATCH

  vendor_version="$OFFICIAL_VERSION"
  yes | "${BUILD_DIR}/vendor/android-prepare-vendor/execute-all.sh" --device "${DEVICE}" --buildID "${OFFICIAL_VERSION}" --output "${BUILD_DIR}/vendor/android-prepare-vendor"
  aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-vendor" --acl public-read <<< "${vendor_version}" || true

  mkdir --parents "${BUILD_DIR}/vendor/google_devices" || true
  rm --recursive --force "${BUILD_DIR}/vendor/google_devices/$DEVICE" || true
  mv "${BUILD_DIR}/vendor/android-prepare-vendor/${DEVICE}/$(tr '[:upper:]' '[:lower:]' <<< "${vendor_version}")/vendor/google_devices/${DEVICE}" "${BUILD_DIR}/vendor/google_devices"

  if [ "$DEVICE" == 'sailfish' ]; then
    rm --recursive --force "${BUILD_DIR}/vendor/google_devices/marlin" || true
    mv "${BUILD_DIR}/vendor/android-prepare-vendor/sailfish/$(tr '[:upper:]' '[:lower:]' <<< "${vendor_version}")/vendor/google_devices/marlin" "${BUILD_DIR}/vendor/google_devices"
  fi

  if [ "$DEVICE" == 'walleye' ]; then
    rm --recursive --force "${BUILD_DIR}/vendor/google_devices/muskie" || true
    mv "${BUILD_DIR}/vendor/android-prepare-vendor/walleye/$(tr '[:upper:]' '[:lower:]' <<< "${vendor_version}")/vendor/google_devices/muskie" "${BUILD_DIR}/vendor/google_devices"
  fi

  popd
}

aws_release() {
  pushd "${BUILD_DIR}/out"
  build_date="$(< build_number.txt)"
  build_timestamp="$(unzip -p "release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" META-INF/com/android/metadata | grep 'post-timestamp' | cut --delimiter "=" --fields 2)"

  read -r old_metadata <<< "$(wget -O - "${RELEASE_URL}/${DEVICE}-stable")"
  old_date="$(cut -d ' ' -f 1 <<< "${old_metadata}")"
  (
  aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "s3://${AWS_RELEASE_BUCKET}" --acl public-read &&
  echo "${build_date} ${build_timestamp} ${OFFICIAL_VERSION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}" --acl public-read &&
  echo "${OFFICIAL_TIMESTAMP}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}-true-timestamp" --acl public-read
  ) && ( aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-ota_update-${old_date}.zip" || true )

  if [ "$(aws s3 ls "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-factory-latest.tar.xz" | wc -l)" == '0' ]; then
    aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-factory-${build_date}.tar.xz" "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-factory-latest.tar.xz" --acl public-read
  fi

  if [ "$(aws s3 ls "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target" | wc -l)" != '0' ]; then
    aws_gen_deltas
  fi
  aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-target_files-${build_date}.zip" "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target/${DEVICE}-target-files-${build_date}.zip" --acl public-read
}

aws_gen_deltas() {
  aws s3 sync "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target" "${BUILD_DIR}/${DEVICE}-target"
  pushd "${BUILD_DIR}/out"
  current_date="$(< build_number.txt)"
  pushd "${BUILD_DIR}/${DEVICE}-target"
  for target_file in ${DEVICE}-target-files-*.zip ; do
    old_date=$(echo "$target_file" | cut --delimiter "-" --fields 4 | cut --delimiter "." --fields 5 --complement)
    pushd "${BUILD_DIR}"
    "${BUILD_DIR}/build/tools/releasetools/ota_from_target_files" --block --package_key "${BUILD_DIR}/keys/${DEVICE}/releasekey" \
    --incremental_from "${BUILD_DIR}/${DEVICE}-target/${DEVICE}-target-files-${old_date}.zip" \
    "${BUILD_DIR}/out/release-${DEVICE}-${current_date}/${DEVICE}-target_files-${current_date}.zip" \
    "${BUILD_DIR}/out/release-${DEVICE}-${current_date}/${DEVICE}-incremental-${old_date}-${current_date}.zip"
    popd
  done
  for incremental in ${BUILD_DIR}/out/release-${DEVICE}-${current_date}/${DEVICE}-incremental-*-*.zip ; do
    ( aws s3 cp "$incremental" "s3://${AWS_RELEASE_BUCKET}/" --acl public-read && aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target/${DEVICE}-target-files-${old_date}.zip") || true
  done
}

aws_notify() {
  aws sns publish --region <% .Region %> --topic-arn "$AWS_SNS_ARN" --message "$1" || true
}

aws_logging() {
  df -h
  du -chs "${BUILD_DIR}"
  uptime
  aws s3 cp /var/log/cloud-init-output.log "s3://${AWS_LOGS_BUCKET}/${DEVICE}/$(date +%s)"
}

aws_gen_keys() {
  gen_keys
  aws s3 sync "${BUILD_DIR}/keys" "s3://${AWS_KEYS_BUCKET}"
}

gen_keys() {
  mkdir --parents "${BUILD_DIR}/keys/${DEVICE}"
  pushd "${BUILD_DIR}/keys/${DEVICE}"
  for key in {releasekey,platform,shared,media,verity} ; do
    # make_key exits with unsuccessful code 1 instead of 0, need ! to negate
    ! "${BUILD_DIR}/development/tools/make_key" "$key" "$CERTIFICATE_SUBJECT"
  done

  if [ "${DEVICE}" == "marlin" ] || [ "${DEVICE}" == "sailfish" ]; then
    gen_verity_key "${DEVICE}"
  fi

  if [ "${DEVICE}" == "walleye" ] || [ "${DEVICE}" == "taimen" ]; then
    gen_avb_key "${DEVICE}"
  fi
}

gen_avb_key() {
  pushd "$BUILD_DIR"
  openssl genrsa -out "${BUILD_DIR}/keys/$1/avb.pem" 2048
  ${BUILD_DIR}/external/avb/avbtool extract_public_key --key "${BUILD_DIR}/keys/$1/avb.pem" --output "${BUILD_DIR}/keys/$1/avb_pkmd.bin"
}

gen_verity_key() {
  pushd "$BUILD_DIR"

  make -j 20 generate_verity_key
  "${BUILD_DIR}/out/host/linux-x86/bin/generate_verity_key" -convert "${BUILD_DIR}/keys/$1/verity.x509.pem" "${BUILD_DIR}/keys/$1/verity_key"
  make clobber

  openssl x509 -outform der -in "${BUILD_DIR}/keys/$1/verity.x509.pem" -out "${BUILD_DIR}/keys/$1/verity_user.der.x509"
  ln --verbose --symbolic "${BUILD_DIR}/keys/$1/verity_user.der.x509" "${BUILD_DIR}/kernel/google/marlin/verity_user.der.x509"
}

cleanup() {
  rv=$?
  aws_logging
  if [ $rv -ne 0 ]; then
    aws_notify "RattlesnakeOS Build FAILED ($OFFICIAL_DATE)"
  else
    aws_notify "RattlesnakeOS Build SUCCESS ($OFFICIAL_DATE)"
  fi
  if ${PREVENT_SHUTDOWN}; then
    echo "Skipping shutdown"
  else
    sudo shutdown -h now
  fi
}

trap cleanup 0

set -e

if [ "$FULL_RUN" = true ]; then
  full_run
fi
`
