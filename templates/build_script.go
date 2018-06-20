package templates

const ShellScriptTemplate = `
#!/bin/bash

read -rd '' HELP << ENDHELP
Usage: $(basename $0) DEVICE_NAME

Options:
	-A do a full run
ENDHELP

DEVICE=$1

# check if supported device
if [ "$DEVICE" == 'sailfish' ] || [ "$DEVICE" == 'marlin' ] || [ "$DEVICE" == 'walleye' ] || [ "$DEVICE" == 'taimen' ]; then
  echo "Supported device $DEVICE - continuing build"
else 
  echo "Unsupported device $DEVICE"
  exit 1
fi

PREVENT_SHUTDOWN=<% .PreventShutdown %>
AWS_KEYS_BUCKET='<% .Name %>-keys'
AWS_RELEASE_BUCKET='<% .Name %>-release'
AWS_LOGS_BUCKET='<% .Name %>-logs'
AWS_SNS_ARN=$(aws --region <% .Region %> sns list-topics --query 'Topics[0].TopicArn' --output text | cut -d":" -f1,2,3,4,5)':<% .Name %>'

BUILD_TARGET="release aosp_${DEVICE} user"
RELEASE_CHANNEL="${DEVICE}-stable"
BUILD_DATE=$(date +%Y.%m.%d.%H)
BUILD_TIMESTAMP=$(date +%s)
BUILD_DIR="$HOME/rattlesnake-os"
CERTIFICATE_SUBJECT='/CN=RattlesnakeOS'

RELEASE_URL="https://${AWS_RELEASE_BUCKET}.s3.amazonaws.com"
ANDROID_SDK_URL="https://dl.google.com/android/repository/sdk-tools-linux-4333796.zip"
CHROME_URL_LATEST="https://omahaproxy.appspot.com/all.json"
MANIFEST_URL='https://android.googlesource.com/platform/manifest'

FDROID_CLIENT_VERSION="1.2.2"
FDROID_PRIV_EXT_VERSION="0.2.8"
OFFICIAL_FDROID_KEY="43238d512c1e5eb2d6569f4a3afbf5523418b82e0a3ed1552770abb9a9c9ccab"

# attempt to automatically pick latest build version and branch. note this is likely to break with any page redesign.
if [ "$DEVICE" == 'sailfish' ] || [ "$DEVICE" == 'marlin' ]; then
  AOSP_BUILD=$(curl -s https://source.android.com/setup/start/build-numbers | grep -m1 -B3 'Pixel XL' | head -1 | cut -f2 -d">"|cut -f1 -d"<")
  AOSP_BRANCH=$(curl -s https://source.android.com/setup/start/build-numbers | grep -m1 -B3 'Pixel XL' | head -2 | tail -1 | cut -f2 -d">"|cut -f1 -d"<")
elif [ "$DEVICE" == 'walleye' ] || [ "$DEVICE" == 'taimen' ]; then
  AOSP_BUILD=$(curl -s https://source.android.com/setup/start/build-numbers | grep -m1 -B3 'Pixel 2 XL' | head -1 | cut -f2 -d">"|cut -f1 -d"<")
  AOSP_BRANCH=$(curl -s https://source.android.com/setup/start/build-numbers | grep -m1 -B3 'Pixel 2 XL' | head -2 | tail -1 | cut -f2 -d">"|cut -f1 -d"<")
fi

# pick kernel
if [ "$DEVICE" == 'sailfish' ] || [ "$DEVICE" == 'marlin' ]; then
  KERNEL_REPO="kernel/msm"
  KERNEL_BRANCH="android-msm-marlin-3.18-oreo-m4"
  KERNEL_NAME="marlin"
elif [ "$DEVICE" == 'walleye' ] || [ "$DEVICE" == 'taimen' ]; then
  KERNEL_REPO="kernel/msm"
  KERNEL_BRANCH="android-msm-wahoo-4.4-oreo-m4"
  KERNEL_NAME="wahoo"
fi

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
  aws_notify "Starting RattlesnakeOS build for ${DEVICE} (date=${BUILD_DATE} aosp_build=${AOSP_BUILD} aosp_branch=${AOSP_BRANCH} kernel_branch=${KERNEL_BRANCH})"
  setup_env
  check_chrome
  fetch_build
  setup_vendor
  aws_import_keys
  patch
  build_kernel
  build_aosp
  aws_release
}

setup_env() {
  # install packages
  sudo apt-get update
  sudo apt-get --assume-yes install openjdk-8-jdk git-core gnupg flex bison build-essential zip curl zlib1g-dev gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev libxml2-utils xsltproc unzip python-networkx liblz4-tool
  sudo apt-get --assume-yes build-dep "linux-image-$(uname --kernel-release)"
  sudo apt-get --assume-yes install repo gperf jq fuseext2

  # setup android sdk (required for fdroid build)
  mkdir -p ${HOME}/sdk
  pushd ${HOME}/sdk
  wget ${ANDROID_SDK_URL} -O sdk-tools.zip
  unzip sdk-tools.zip
  yes | ./tools/bin/sdkmanager --licenses
  ./tools/android update sdk -u --use-sdk-wrapper

  # setup git
  git config --get --global user.name || git config --global user.name 'unknown'
  git config --get --global user.email || git config --global user.email 'unknown@localhost'
  git config --global color.ui true

  mkdir -p "$BUILD_DIR"
}

check_chrome() {
  current=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/revision" - || true)
  echo "Chromium current: $current"

  mkdir -p $HOME/chromium
  cd $HOME/chromium
  latest=$(curl -s "$CHROME_URL_LATEST" | jq -r '.[] | select(.os == "android") | .versions[] | select(.channel == "stable") | .current_version' || true)
  echo "Chromium latest: $latest"

  if [ "$latest" == "$current" ]; then
    echo "Chromium latest ($latest) matches current ($current) - just copying s3 chromium artifact"
    aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/MonochromePublic.apk" ${BUILD_DIR}/external/chromium/prebuilt/arm64/
  else
    echo "Building chromium $latest"
    build_chrome $latest
  fi
  rm -rf $HOME/chromium
}

build_chrome() {
  CHROMIUM_REVISION=$1
  DEFAULT_VERSION=$(echo $CHROMIUM_REVISION | awk -F"." '{ printf "%s%03d52\n",$3,$4}')
  pushd "$BUILD_DIR" 
  git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git $HOME/depot_tools || true
  export PATH="$PATH:$HOME/depot_tools"
  mkdir -p $HOME/chromium
  cd $HOME/chromium
  fetch --nohooks android --target_os_only=true || true
  yes | gclient sync --with_branch_heads -r $CHROMIUM_REVISION --jobs 32
  cd src
  mkdir -p out/Default
  cat <<EOF > out/Default/args.gn
target_os = "android"
target_cpu = "arm64"
is_debug = false

is_official_build = true
is_component_build = false
symbol_level = 0

ffmpeg_branding = "Chrome"
proprietary_codecs = true

android_channel = "stable"
android_default_version_name = "$CHROMIUM_REVISION"
android_default_version_code = "$DEFAULT_VERSION"
EOF

  build/linux/sysroot_scripts/install-sysroot.py --arch=i386
  build/linux/sysroot_scripts/install-sysroot.py --arch=amd64
  gn gen out/Default
  ninja -C out/Default/ monochrome_public_apk

  cp out/Default/apks/MonochromePublic.apk ${BUILD_DIR}/external/chromium/prebuilt/arm64/
  aws s3 cp "${BUILD_DIR}/external/chromium/prebuilt/arm64/MonochromePublic.apk" "s3://${AWS_RELEASE_BUCKET}/chromium/MonochromePublic.apk" --acl public-read
  echo "${CHROMIUM_REVISION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/chromium/revision" --acl public-read
}

fetch_build() {
  pushd "${BUILD_DIR}"
  repo init --manifest-url "$MANIFEST_URL" --manifest-branch "$AOSP_BRANCH" || true

  # make modifications to default AOSP
  if ! grep -q "RattlesnakeOS" .repo/manifest.xml; then
    awk -i inplace \
      -v KERNEL_REPO="$KERNEL_REPO" \
      -v KERNEL_BRANCH="$KERNEL_BRANCH" \
      -v KERNEL_NAME="$KERNEL_NAME" \
      -v FDROID_CLIENT_VERSION="$FDROID_CLIENT_VERSION" \
      -v FDROID_PRIV_EXT_VERSION="$FDROID_PRIV_EXT_VERSION" \
      '1;/<repo-hooks in-project=/{
      print "  ";
      print "  <remote name=\"github\" fetch=\"https://github.com/RattlesnakeOS/\" revision=\"master\" />";
      print "  <remote name=\"fdroid\" fetch=\"https://gitlab.com/fdroid/\" />";
      print "  <remote name=\"prepare-vendor\" fetch=\"https://github.com/anestisb/\" revision=\"master\" />";
      print "  ";
      print "  <project path=\"script\" name=\"script\" remote=\"github\" />";
      print "  <project path=\"external/chromium\" name=\"platform_external_chromium\" remote=\"github\" />";
      print "  <project path=\"packages/apps/Updater\" name=\"platform_packages_apps_Updater\" remote=\"github\" />";
      print "  <project path=\"packages/apps/F-Droid\" name=\"fdroidclient\" remote=\"fdroid\" revision=\"refs/tags/" FDROID_CLIENT_VERSION "\" />";
      print "  <project path=\"packages/apps/F-DroidPrivilegedExtension\" name=\"privileged-extension\" remote=\"fdroid\" revision=\"refs/tags/" FDROID_PRIV_EXT_VERSION "\" />";
      print "  <project path=\"kernel/google/" KERNEL_NAME "\" name=\"" KERNEL_REPO "\" remote=\"aosp\" revision=\"" KERNEL_BRANCH "\" />";
      print "  <project path=\"vendor/android-prepare-vendor\" name=\"android-prepare-vendor\" remote=\"prepare-vendor\" />"}' .repo/manifest.xml
  else
    echo "Skipping modification of .repo/manifest.xml as they have already been made"
  fi
  
  # remove webview
  sed -i '/chromium-webview/d' .repo/manifest.xml
  rm -rf platform/external/chromium-webview
  sed -i '/webview \\/d' build/make/target/product/core_minimal.mk

  # remove Browser2
  sed -i '/packages\/apps\/Browser2/d' .repo/manifest.xml
  sed -i '/Browser2/d' build/make/target/product/core.mk
  rm -rf platform/packages/apps/Browser2

  # remove Calendar
  sed -i '/packages\/apps\/Calendar/d' .repo/manifest.xml
  sed -i '/Calendar/d' build/make/target/product/core.mk
  rm -rf platform/packages/apps/Calendar

  # remove QuickSearchBox
  sed -i '/packages\/apps\/QuickSearchBox/d' .repo/manifest.xml
  sed -i '/QuickSearchBox/d' build/make/target/product/core.mk
  rm -rf platform/packages/apps/QuickSearchBox

  # sync with retries
  for i in {1..10}; do
    repo sync --jobs 32 && break
  done
}

setup_vendor() {
  pushd "${BUILD_DIR}/vendor/android-prepare-vendor"
  sed -i.bkp 's/  USE_DEBUGFS=true/  USE_DEBUGFS=false/; s/  # SYS_TOOLS/  SYS_TOOLS/; s/  # _UMOUNT=/  _UMOUNT=/' execute-all.sh

  # get vendor files
  yes | "${BUILD_DIR}/vendor/android-prepare-vendor/execute-all.sh" --device "${DEVICE}" --buildID "${AOSP_BUILD}" --output "${BUILD_DIR}/vendor/android-prepare-vendor"
  aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-vendor" --acl public-read <<< "${AOSP_BUILD}" || true

  # copy vendor files to build tree
  mkdir --parents "${BUILD_DIR}/vendor/google_devices" || true
  rm --recursive --force "${BUILD_DIR}/vendor/google_devices/$DEVICE" || true
  mv "${BUILD_DIR}/vendor/android-prepare-vendor/${DEVICE}/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD}")/vendor/google_devices/${DEVICE}" "${BUILD_DIR}/vendor/google_devices"

  # smaller devices need big brother vendor files
  if [ "$DEVICE" == 'sailfish' ]; then
    rm --recursive --force "${BUILD_DIR}/vendor/google_devices/marlin" || true
    mv "${BUILD_DIR}/vendor/android-prepare-vendor/sailfish/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD}")/vendor/google_devices/marlin" "${BUILD_DIR}/vendor/google_devices"
  fi
  if [ "$DEVICE" == 'walleye' ]; then
    rm --recursive --force "${BUILD_DIR}/vendor/google_devices/muskie" || true
    mv "${BUILD_DIR}/vendor/android-prepare-vendor/walleye/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD}")/vendor/google_devices/muskie" "${BUILD_DIR}/vendor/google_devices"
  fi

  popd
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

patch() {
  patch_apps
  patch_chromium_webview
  patch_updater
  patch_fdroid
  patch_priv_ext
}

patch_chromium_webview() {
  sed -i -e 's/Android WebView/Chromium/; s/com.android.webview/org.chromium.chrome/;' ${BUILD_DIR}/frameworks/base/core/res/res/xml/config_webview_packages.xml
}

patch_fdroid() {
  echo "sdk.dir=${HOME}/sdk" > ${BUILD_DIR}/packages/apps/F-Droid/local.properties
  echo "sdk.dir=${HOME}/sdk" > ${BUILD_DIR}/packages/apps/F-Droid/app/local.properties
  sed -i 's/gradle assembleRelease/..\/gradlew assembleRelease/' ${BUILD_DIR}/packages/apps/F-Droid/Android.mk
  pushd ${BUILD_DIR}/packages/apps/F-Droid
  # for some reason first build fails - so do it now
  ./gradlew assembleRelease || true
}

patch_apps() {
  sed -i.original "\$aPRODUCT_PACKAGES += Updater" build/make/target/product/core.mk
  sed -i.original "\$aPRODUCT_PACKAGES += F-DroidPrivilegedExtension" build/make/target/product/core.mk
  sed -i.original "\$aPRODUCT_PACKAGES += F-Droid" build/make/target/product/core.mk
  sed -i.original "\$aPRODUCT_PACKAGES += chromium" build/make/target/product/core.mk
}

patch_updater() {
  pushd "$BUILD_DIR"/packages/apps/Updater/res/values
  sed --in-place \
    --expression "s@s3bucket@${RELEASE_URL}/@g" config.xml
}

fdpe_hash() {
  keytool -list -printcert -file "$1" | grep 'SHA256:' | tr --delete ':' | cut --delimiter ' ' --fields 3
}

patch_priv_ext() {
  unofficial_sailfish_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/sailfish/releasekey.x509.pem")
  unofficial_sailfish_platform_hash=$(fdpe_hash "${BUILD_DIR}/keys/sailfish/platform.x509.pem")
  unofficial_marlin_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/marlin/releasekey.x509.pem")
  unofficial_marlin_platform_hash=$(fdpe_hash "${BUILD_DIR}/keys/marlin/platform.x509.pem")
  unofficial_taimen_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/taimen/releasekey.x509.pem")
  unofficial_walleye_releasekey_hash=$(fdpe_hash "${BUILD_DIR}/keys/walleye/releasekey.x509.pem")

  if [ "$DEVICE" == 'marlin' ]; then
    sed -i 's/'${OFFICIAL_FDROID_KEY}'")/'${unofficial_marlin_releasekey_hash}'"),\n            new Pair<>("org.fdroid.fdroid", "'${unofficial_marlin_platform_hash}'")/' \
      "${BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
  fi
  if [ "$DEVICE" == 'sailfish' ]; then
    sed -i 's/'${OFFICIAL_FDROID_KEY}'")/'${unofficial_sailfish_releasekey_hash}'"),\n            new Pair<>("org.fdroid.fdroid", "'${unofficial_sailfish_platform_hash}'")/' \
      "${BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
  fi
  if [ "$DEVICE" == 'walleye' ]; then
    sed -i 's/'${OFFICIAL_FDROID_KEY}'")/'${unofficial_walleye_releasekey_hash}'")/' \
      "${BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
  fi
  if [ "$DEVICE" == 'taimen' ]; then
    sed -i 's/'${OFFICIAL_FDROID_KEY}'")/'${unofficial_taimen_releasekey_hash}'")/' \
      "${BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
  fi
}

build_kernel() {
  # run in another shell to avoid it mucking with environment variables for normal AOSP build
  bash -c "\
    cd ${BUILD_DIR};
    . build/envsetup.sh;
    make -j$(nproc --all) dtc mkdtimg;
    export PATH=${AOSP_FOLDER}/out/host/linux-x86/bin:${PATH};
    cd ${BUILD_DIR}/kernel/google/${KERNEL_NAME};
    make -j$(nproc --all) ARCH=arm64 ${KERNEL_NAME}_defconfig;
    make -j$(nproc --all) ARCH=arm64 CROSS_COMPILE=${BUILD_DIR}/prebuilts/gcc/linux-x86/aarch64/aarch64-linux-android-4.9/bin/aarch64-linux-android-;
    cp -f arch/arm64/boot/Image.lz4-dtb ${BUILD_DIR}/device/google/${KERNEL_NAME}-kernel/;
    rm -f ${BUILD_DIR}/out/build_*;
  "
}

build_aosp() {
  pushd "$BUILD_DIR"
  source "${BUILD_DIR}/script/setup.sh"

  choosecombo $BUILD_TARGET
  make -j $(nproc) target-files-package
  make -j $(nproc) brillo_update_payload

  "${BUILD_DIR}/script/release.sh" "$DEVICE"
}

aws_release() {
  pushd "${BUILD_DIR}/out"
  build_date="$(< build_number.txt)"
  build_timestamp="$(unzip -p "release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" META-INF/com/android/metadata | grep 'post-timestamp' | cut --delimiter "=" --fields 2)"

  read -r old_metadata <<< "$(wget -O - "${RELEASE_URL}/${DEVICE}-stable")"
  old_date="$(cut -d ' ' -f 1 <<< "${old_metadata}")"
  (
  aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "s3://${AWS_RELEASE_BUCKET}" --acl public-read &&
  echo "${build_date} ${build_timestamp} ${AOSP_BUILD}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}" --acl public-read &&
  echo "${BUILD_TIMESTAMP}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}-true-timestamp" --acl public-read
  ) && ( aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-ota_update-${old_date}.zip" || true )

  if [ "$(aws s3 ls "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-factory-latest.tar.xz" | wc -l)" == '0' ]; then
    aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-factory-${build_date}.tar.xz" "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-factory-latest.tar.xz"
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
  ln --verbose --symbolic "${BUILD_DIR}/keys/$1/verity_user.der.x509" "${BUILD_DIR}/kernel/google/${KERNEL_NAME}/verity_user.der.x509"
}

cleanup() {
  rv=$?
  aws_logging
  if [ $rv -ne 0 ]; then
    aws_notify "RattlesnakeOS build FAILED for ${DEVICE} (date=${BUILD_DATE} aosp_build=${AOSP_BUILD} aosp_branch=${AOSP_BRANCH} kernel_branch=${KERNEL_BRANCH})"
  else
    aws_notify "RattlesnakeOS build SUCCESS for ${DEVICE} (date=${BUILD_DATE} aosp_build=${AOSP_BUILD} aosp_branch=${AOSP_BRANCH} kernel_branch=${KERNEL_BRANCH})"
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
