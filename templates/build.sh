#!/usr/bin/env bash

AOSP_BUILD_ID=$1
echo "AOSP_BUILD_ID=${AOSP_BUILD_ID}"
AOSP_TAG=$2
echo "AOSP_TAG=${AOSP_TAG}"
CHROMIUM_VERSION=$3
echo "CHROMIUM_VERSION=${CHROMIUM_VERSION}"
FDROID_CLIENT_VERSION=$4
echo "FDROID_CLIENT_VERSION=${FDROID_CLIENT_VERSION}"
FDROID_PRIV_EXT_VERSION=$5
echo "FDROID_PRIV_EXT_VERSION=${FDROID_PRIV_EXT_VERSION}"

####REPLACE-VARS####

SECONDS=0
full_run() {
  log_header "${FUNCNAME[0]}"

  notify "RattlesnakeOS Build STARTED"
  setup_env
  import_keys
  # TODO: not sure if this is a great idea, but does speed up build
  check_chromium &
  aosp_setup &
  wait
  setup_vendor &
  build_fdroid &
  add_chromium &
  wait
  apply_patches
  build_aosp
  release
  upload
  checkpoint_versions
  notify "RattlesnakeOS Build SUCCESS"
}

setup_env() {
  log_header "${FUNCNAME[0]}"

  # install required packages
  sudo apt-get update
  sudo DEBIAN_FRONTEND=noninteractive apt-get -y install python2 python3 gperf jq default-jdk git-core gnupg \
      flex bison build-essential zip curl zlib1g-dev gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev \
      x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev libxml2-utils xsltproc unzip liblz4-tool libncurses5

  retry curl --fail -s https://storage.googleapis.com/git-repo-downloads/repo > /tmp/repo
  chmod +x /tmp/repo
  sudo mv /tmp/repo /usr/local/bin/

  # still some scripts that expect python2 as default
  sudo update-alternatives --install /usr/bin/python python /usr/bin/python2 1 || true
  sudo update-alternatives --install /usr/bin/python python /usr/bin/python3 2 || true
  sudo update-alternatives --config python <<< 1

  # setup git
  git config --get --global user.name || git config --global user.name 'aosp'
  git config --get --global user.email || git config --global user.email 'aosp@localhost'
  git config --global color.ui true

  # mount /tmp filesystem as tmpfs
  sudo mount -t tmpfs tmpfs /tmp || true

  # mount keys directory as tmpfs
  mkdir -p "${KEYS_DIR}"
  sudo mount -t tmpfs -o size=20m tmpfs "${KEYS_DIR}" || true
}

check_chromium() {
  log_header "${FUNCNAME[0]}"

  current=$(get_current_metadata "chromium/revision")
  log "Chromium current: ${current}"

  log "Chromium requested: ${CHROMIUM_VERSION}"
  if [ "${CHROMIUM_VERSION}" == "${current}" ]; then
    log "Chromium requested (${CHROMIUM_VERSION}) matches current (${current})"
  else
    log "Building chromium ${CHROMIUM_VERSION}"
    build_chromium "${CHROMIUM_VERSION}"
  fi

  log "Deleting chromium directory ${CHROMIUM_BUILD_DIR}"
  rm -rf "${CHROMIUM_BUILD_DIR}"
}

build_chromium() {
  log_header "${FUNCNAME[0]}"

  CHROMIUM_REVISION="$1"
  DEFAULT_VERSION=$(echo "${CHROMIUM_REVISION}" | awk -F"." '{ printf "%s%03d52\n",$3,$4}')

  # depot tools setup
  if [ ! -d "${MISC_DIR}/depot_tools" ]; then
    retry git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git "${MISC_DIR}/depot_tools"
  fi
  export PATH="${PATH}:${MISC_DIR}/depot_tools"

  # fetch chromium
  cd "${CHROMIUM_BUILD_DIR}"
  fetch --nohooks android
  cd src

  # checkout specific revision
  git checkout "${CHROMIUM_REVISION}" -f

  # install dependencies
  echo ttf-mscorefonts-installer msttcorefonts/accepted-mscorefonts-eula select true | sudo debconf-set-selections
  log "Installing chromium build dependencies"

  sudo ./build/install-build-deps-android.sh

  # run gclient sync (runhooks will run as part of this)
  log "Running gclient sync (this takes a while)"
  for i in {1..5}; do
    yes | gclient sync --with_branch_heads --jobs 32 -RDf && break
  done

  # cleanup any files in tree not part of this revision
  git clean -dff

  # reset any modifications
  git checkout -- .

  # generate configuration
  KEYSTORE="${KEYS_DIR}/${DEVICE}/chromium.keystore"
  trichrome_certdigest=$(keytool -export-cert -alias chromium -keystore "${KEYSTORE}" -storepass chromium | sha256sum | awk '{print $1}')
  log "trichrome_certdigest=${trichrome_certdigest}"
  mkdir -p out/Default
  cat <<EOF > out/Default/args.gn
target_os = "android"
target_cpu = "arm64"
android_channel = "stable"
android_default_version_name = "${CHROMIUM_REVISION}"
android_default_version_code = "${DEFAULT_VERSION}"
is_component_build = false
is_debug = false
is_official_build = true
symbol_level = 1
fieldtrial_testing_like_official_build = true
ffmpeg_branding = "Chrome"
proprietary_codecs = true
is_cfi = true
enable_gvr_services = false
enable_remoting = false
enable_reporting = true

trichrome_certdigest = "${trichrome_certdigest}"
chrome_public_manifest_package = "org.chromium.chrome"
system_webview_package_name = "org.chromium.webview"
trichrome_library_package = "org.chromium.trichromelibrary"
EOF
  gn gen out/Default

  log "Building trichrome"
  autoninja -C out/Default/ trichrome_webview_apk trichrome_chrome_bundle trichrome_library_apk

  log "Signing trichrome"
  BUNDLETOOL="${CHROMIUM_BUILD_DIR}/src/build/android/gyp/bundletool.py"
  AAPT2="${CHROMIUM_BUILD_DIR}/src/third_party/android_build_tools/aapt2/aapt2"
  find "${CHROMIUM_BUILD_DIR}/src" | grep 'apksigner' || true
  APKSIGNER="${CHROMIUM_BUILD_DIR}/src/third_party/android_sdk/public/build-tools/30.0.1/apksigner"
  cd out/Default/apks
  rm -rf release
  mkdir release
  cd release
  "${BUNDLETOOL}" build-apks --aapt2 "${AAPT2}" --bundle "../TrichromeChrome.aab" --output "TrichromeChrome.apks" \
      --mode=universal --ks "${KEYSTORE}" --ks-pass pass:chromium --ks-key-alias chromium
  unzip "TrichromeChrome.apks" "universal.apk"
  mv "universal.apk" "TrichromeChrome.apk"
  for app in TrichromeLibrary TrichromeWebView; do
    "${APKSIGNER}" sign --ks "${KEYSTORE}" --ks-pass pass:chromium --ks-key-alias chromium --in "../${app}.apk" --out "${app}.apk"
  done

  log "Uploading built trichrome apks"
  upload_build_artifact "TrichromeLibrary.apk" "chromium/TrichromeLibrary.apk"
  upload_build_artifact "TrichromeWebView.apk" "chromium/TrichromeWebView.apk"
  upload_build_artifact "TrichromeChrome.apk" "chromium/TrichromeChrome.apk"
  set_current_metadata "${CHROMIUM_REVISION}" "chromium/revision"
}

aosp_setup() {
  log_header "${FUNCNAME[0]}"
  aosp_repo_init
  log "Running aosp_repo_sync before modifications"
  aosp_repo_sync
  log "Running aosp_prebuild"
  aosp_prebuild
  log "Running aosp_repo_modifications"
  aosp_repo_modifications
  log "Running aosp_repo_sync after modifications"
  aosp_repo_sync
}

aosp_repo_init() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  retry repo init --manifest-url "${MANIFEST_URL}" --manifest-branch "${AOSP_TAG}" --depth 1 || true
}

aosp_repo_modifications() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  mkdir -p "${AOSP_BUILD_DIR}/.repo/local_manifests"

  cat <<EOF > "${AOSP_BUILD_DIR}/.repo/local_manifests/rattlesnakeos.xml"
<?xml version="1.0" encoding="UTF-8"?>
<manifest>
  <remote name="github" fetch="https://github.com/RattlesnakeOS/" revision="${ANDROID_VERSION}" />
  <remote name="fdroid" fetch="https://gitlab.com/fdroid/" />

  <project path="packages/apps/Updater" name="platform_packages_apps_Updater" remote="github" />
  <project path="packages/apps/F-Droid" name="platform_external_fdroid" remote="github" />
  <project path="packages/apps/F-DroidPrivilegedExtension" name="privileged-extension" remote="fdroid" revision="refs/tags/${FDROID_PRIV_EXT_VERSION}" />
  <project path="vendor/android-prepare-vendor" name="android-prepare-vendor" remote="github" />
  <project path="external/chromium" name="platform_external_chromium" remote="github" />

  <remove-project name="platform/external/chromium-webview" />
  <remove-project name="platform/packages/apps/Browser2" />

  ${CUSTOM_MANIFEST_REMOTES}
  ${CUSTOM_MANIFEST_PROJECTS}
</manifest>
EOF

}

aosp_repo_sync() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  # sync with retries
  for i in {1..10}; do
    log "aosp repo sync attempt ${i}/10"
    repo sync -c --no-tags --no-clone-bundle --jobs 32 && break
  done
}

aosp_prebuild() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  source build/envsetup.sh
  export LANG=C
  export _JAVA_OPTIONS=-XX:-UsePerfData
  export BUILD_NUMBER=$(cat out/soong/build_number.txt 2>/dev/null || date --utc +%Y.%m.%d.%H)
  export DISPLAY_BUILD_NUMBER=true
  chrt -b -p 0 $$

  log "Running aosp_prebuild choosecombo ${BUILD_TARGET}"
  choosecombo ${BUILD_TARGET}

  log "Running aosp_prebuild target-files-package"
  make -j "$(nproc)" target-files-package || true
}

setup_vendor() {
  log_header "${FUNCNAME[0]}"

  # new dependency to extract ota partitions
  sudo DEBIAN_FRONTEND=noninteractive apt-get -y install python-protobuf python3-protobuf python3-pip
  pip3 install --user protobuf -U

  # get vendor files (with timeout)
  timeout 30m "${AOSP_BUILD_DIR}/vendor/android-prepare-vendor/execute-all.sh" --debugfs --yes --device "${DEVICE}" \
      --buildID "${AOSP_BUILD_ID}" --output "${AOSP_BUILD_DIR}/vendor/android-prepare-vendor"

  # copy vendor files to build tree
  mkdir --parents "${AOSP_BUILD_DIR}/vendor/google_devices" || true
  rm -rf "${AOSP_BUILD_DIR}/vendor/google_devices/${DEVICE}" || true
  mv "${AOSP_BUILD_DIR}/vendor/android-prepare-vendor/${DEVICE}/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD_ID}")/vendor/google_devices/${DEVICE}" "${AOSP_BUILD_DIR}/vendor/google_devices"

  # smaller devices need big brother vendor files
  if [ "${DEVICE}" != "${DEVICE_FAMILY}" ]; then
    rm -rf "${AOSP_BUILD_DIR}/vendor/google_devices/${DEVICE_FAMILY}" || true
    mv "${AOSP_BUILD_DIR}/vendor/android-prepare-vendor/${DEVICE}/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_BUILD_ID}")/vendor/google_devices/${DEVICE_FAMILY}" "${AOSP_BUILD_DIR}/vendor/google_devices"
  fi

  # workaround for libsdsprpc and libadsprpc not specifying LOCAL_SHARED_LIBRARIES
  sed -i '/LOCAL_MODULE := libsdsprpc/a LOCAL_SHARED_LIBRARIES := libc++ libc libcutils libdl libion liblog libm' "${AOSP_BUILD_DIR}/vendor/google_devices/${DEVICE}/Android.mk" || true
  sed -i '/LOCAL_MODULE := libadsprpc/a LOCAL_SHARED_LIBRARIES := libc++ libc libcutils libdl libion liblog libm' "${AOSP_BUILD_DIR}/vendor/google_devices/${DEVICE}/Android.mk" || true
}

add_chromium() {
  log_header "${FUNCNAME[0]}"

  # add latest built chromium to external/chromium
  download_build_artifact "chromium/TrichromeLibrary.apk" "${AOSP_BUILD_DIR}/external/chromium/prebuilt/arm64/"
  download_build_artifact "chromium/TrichromeWebView.apk" "${AOSP_BUILD_DIR}/external/chromium/prebuilt/arm64/"
  download_build_artifact "chromium/TrichromeChrome.apk" "${AOSP_BUILD_DIR}/external/chromium/prebuilt/arm64/"

  cat <<EOF > "${AOSP_BUILD_DIR}/frameworks/base/core/res/res/xml/config_webview_packages.xml"
<?xml version="1.0" encoding="utf-8"?>
<webviewproviders>
    <webviewprovider description="Chromium WebView" packageName="org.chromium.webview" availableByDefault="true">
    </webviewprovider>
</webviewproviders>
EOF
}

build_fdroid() {
  log_header "${FUNCNAME[0]}"
  pushd "${FDROID_BUILD_DIR}"

  # install gradle
  gradle_version="6.6.1"
  if [ ! -f "${FDROID_BUILD_DIR}/gradle/gradle-${gradle_version}/bin/gradle" ]; then
    retry wget -q "https://downloads.gradle-dn.com/distributions/gradle-${gradle_version}-bin.zip" -O "gradle-${gradle_version}-bin.zip"
    mkdir -p "${FDROID_BUILD_DIR}/gradle"
    unzip -d "${FDROID_BUILD_DIR}/gradle" "gradle-${gradle_version}-bin.zip"
  fi
  export PATH="${PATH}:${FDROID_BUILD_DIR}/gradle/gradle-${gradle_version}/bin"
  popd

  # setup android sdk root/paths, commandline tools and install build-tools
  export ANDROID_SDK_ROOT="${FDROID_BUILD_DIR}/sdk"
  export ANDROID_HOME="${ANDROID_SDK_ROOT}"
  export PATH="${PATH}:${ANDROID_SDK_ROOT}/cmdline-tools/tools"
  export PATH="${PATH}:${ANDROID_SDK_ROOT}/cmdline-tools/tools/bin"
  export PATH="${PATH}:${ANDROID_SDK_ROOT}/platform-tools"
  if [ ! -f "${ANDROID_SDK_ROOT}/cmdline-tools/tools/bin/sdkmanager" ]; then
    mkdir -p "${ANDROID_SDK_ROOT}/cmdline-tools"
    pushd "${ANDROID_SDK_ROOT}/cmdline-tools"
    retry wget -q "https://dl.google.com/android/repository/commandlinetools-linux-6609375_latest.zip" -O commandline-tools.zip
    unzip commandline-tools.zip
    yes | sdkmanager --licenses
    sdkmanager --update
    popd
  fi

  # build it outside AOSP build tree or hit errors
  rm -rf "${FDROID_BUILD_DIR}/fdroidclient"
  git clone https://gitlab.com/fdroid/fdroidclient "${FDROID_BUILD_DIR}/fdroidclient"
  pushd "${FDROID_BUILD_DIR}/fdroidclient"
  git checkout "${FDROID_CLIENT_VERSION}"
  retry gradle assembleRelease

  # copy to AOSP build tree
  cp -f "app/build/outputs/apk/full/release/app-full-release-unsigned.apk" "${AOSP_BUILD_DIR}/packages/apps/F-Droid/F-Droid.apk"
  popd
}

apply_patches() {
  log_header "${FUNCNAME[0]}"

  patch_11_issues
  patch_launcher
  patch_disable_apex
  patch_custom
  patch_base_config
  patch_device_config
  patch_add_apps
  patch_updater
  patch_priv_ext
}

patch_11_issues() {
  log_header "${FUNCNAME[0]}"

  # workaround for vintf build issue
  sed -i '1 i\BUILD_BROKEN_VINTF_PRODUCT_COPY_FILES := true' "${AOSP_BUILD_DIR}/build/make/target/board/BoardConfigMainlineCommon.mk"

  # workaround for coral/flame build issue
  sed -i 's@PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@#PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@' "${AOSP_BUILD_DIR}/device/google/coral/aosp_coral.mk" || true
  sed -i 's@PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@#PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@' "${AOSP_BUILD_DIR}/device/google/coral/aosp_flame.mk" || true

  # biometrics was disabled (https://cs.android.com/android/_/android/platform/frameworks/base/+/ede919cace2a32ec235eefe86e04a75848bd1d5f)
  # but never added upstream to device specific overlays

  # ID0:Fingerprint:Strong
  biometric_sensors="0:2:15"
  if [ "${DEVICE_COMMON}" == "coral" ]; then
    # ID0:Face:Strong
    biometric_sensors="0:8:15"
  fi
  if [ "${DEVICE_COMMON}" == "redfin" ]
  then
    sed -i '$ s/^<\/resources>//' "${AOSP_BUILD_DIR}/device/google/${DEVICE_COMMON}/${DEVICE_COMMON}/overlay/frameworks/base/core/res/res/values/config.xml"
    cat <<EOF >> "${AOSP_BUILD_DIR}/device/google/${DEVICE_COMMON}/${DEVICE_COMMON}/overlay/frameworks/base/core/res/res/values/config.xml"
    <string-array name="config_biometric_sensors" translatable="false" >
        <item>${biometric_sensors}</item>
    </string-array>
</resources>
EOF
  else
    sed -i '$ s/^<\/resources>//' "${AOSP_BUILD_DIR}/device/google/${DEVICE_COMMON}/overlay/frameworks/base/core/res/res/values/config.xml"
    cat <<EOF >> "${AOSP_BUILD_DIR}/device/google/${DEVICE_COMMON}/overlay/frameworks/base/core/res/res/values/config.xml"
    <string-array name="config_biometric_sensors" translatable="false" >
        <item>${biometric_sensors}</item>
    </string-array>
</resources>
EOF
  fi
}

patch_launcher() {
  log_header "${FUNCNAME[0]}"

  # disable QuickSearchBox widget on home screen
  sed -i "s/QSB_ON_FIRST_SCREEN = true;/QSB_ON_FIRST_SCREEN = false;/" "${AOSP_BUILD_DIR}/packages/apps/Launcher3/src/com/android/launcher3/config/FeatureFlags.java"
}

# currently don't have a need for apex updates (https://source.android.com/devices/tech/ota/apex)
patch_disable_apex() {
  log_header "${FUNCNAME[0]}"

  # pixel 2 devices opt in here
  sed -i 's@$(call inherit-product, $(SRC_TARGET_DIR)/product/updatable_apex.mk)@@' "${AOSP_BUILD_DIR}/device/google/wahoo/device.mk" || true
  # all other devices use mainline and opt in here
  sed -i 's@$(call inherit-product, $(SRC_TARGET_DIR)/product/updatable_apex.mk)@@' "${AOSP_BUILD_DIR}/build/make/target/product/mainline_system.mk"
}

patch_base_config() {
  log_header "${FUNCNAME[0]}"

  # enable doze and app standby
  sed -i 's@<bool name="config_enableAutoPowerModes">false</bool>@<bool name="config_enableAutoPowerModes">true</bool>@' "${AOSP_BUILD_DIR}/frameworks/base/core/res/res/values/config.xml"
}

patch_device_config() {
  log_header "${FUNCNAME[0]}"

  # set proper model names
  sed -i 's@PRODUCT_MODEL := AOSP on taimen@PRODUCT_MODEL := Pixel 2 XL@' "${AOSP_BUILD_DIR}/device/google/taimen/aosp_taimen.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on walleye@PRODUCT_MODEL := Pixel 2@' "${AOSP_BUILD_DIR}/device/google/muskie/aosp_walleye.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on crosshatch@PRODUCT_MODEL := Pixel 3 XL@' "${AOSP_BUILD_DIR}/device/google/crosshatch/aosp_crosshatch.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on blueline@PRODUCT_MODEL := Pixel 3@' "${AOSP_BUILD_DIR}/device/google/crosshatch/aosp_blueline.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on bonito@PRODUCT_MODEL := Pixel 3a XL@' "${AOSP_BUILD_DIR}/device/google/bonito/aosp_bonito.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on sargo@PRODUCT_MODEL := Pixel 3a@' "${AOSP_BUILD_DIR}/device/google/bonito/aosp_sargo.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on coral@PRODUCT_MODEL := Pixel 4 XL@' "${AOSP_BUILD_DIR}/device/google/coral/aosp_coral.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on flame@PRODUCT_MODEL := Pixel 4@' "${AOSP_BUILD_DIR}/device/google/coral/aosp_flame.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on sunfish@PRODUCT_MODEL := Pixel 4A@' "${AOSP_BUILD_DIR}/device/google/sunfish/aosp_sunfish.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on redfin@PRODUCT_MODEL := Pixel 5@' "${AOSP_BUILD_DIR}/device/google/redfin/aosp_redfin.mk" || true
}

patch_priv_ext() {
  log_header "${FUNCNAME[0]}"

  # 0.2.9 added whitelabel support, so BuildConfig.APPLICATION_ID needs to be set now
  sed -i 's@BuildConfig.APPLICATION_ID@"org.fdroid.fdroid.privileged"@' "${AOSP_BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/PrivilegedService.java"

  unofficial_releasekey_hash=$(fdpe_hash "${KEYS_DIR}/${DEVICE}/releasekey.x509.pem")
  unofficial_platform_hash=$(fdpe_hash "${KEYS_DIR}/${DEVICE}/platform.x509.pem")
  sed -i 's/'${OFFICIAL_FDROID_KEY}'")/'${unofficial_releasekey_hash}'"),\n            new Pair<>("org.fdroid.fdroid", "'${unofficial_platform_hash}'")/' \
      "${AOSP_BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
}

build_aosp() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  source build/envsetup.sh
  export LANG=C
  export _JAVA_OPTIONS=-XX:-UsePerfData
  export BUILD_NUMBER=$(cat out/soong/build_number.txt 2>/dev/null || date --utc +%Y.%m.%d.%H)
  log "BUILD_NUMBER=${BUILD_NUMBER}"
  export DISPLAY_BUILD_NUMBER=true
  chrt -b -p 0 $$

  log "Running choosecombo ${BUILD_TARGET}"
  choosecombo ${BUILD_TARGET}

  log "Running target-files-package"
  retry make -j "$(nproc)" target-files-package

  log "Running m otatools-package"
  m otatools-package
  rm -rf "${RELEASE_TOOLS_DIR}"
  unzip "${AOSP_BUILD_DIR}/out/target/product/${DEVICE}/otatools.zip" -d "${RELEASE_TOOLS_DIR}"
}

get_radio_image() {
  grep -Po "require version-$1=\K.+" "vendor/$2/vendor-board-info.txt" | tr '[:upper:]' '[:lower:]'
}

release() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  ############################
  # from original setup.sh script
  ############################
  source build/envsetup.sh
  export LANG=C
  export _JAVA_OPTIONS=-XX:-UsePerfData
  export BUILD_NUMBER=$(cat out/soong/build_number.txt 2>/dev/null || date --utc +%Y.%m.%d.%H)
  log "BUILD_NUMBER=${BUILD_NUMBER}"
  export DISPLAY_BUILD_NUMBER=true
  chrt -b -p 0 $$

  ############################
  # from original release.sh script
  ############################
  KEY_DIR="${KEYS_DIR}/${DEVICE}"
  OUT="out/release-${DEVICE}-${BUILD_NUMBER}"
  device="${DEVICE}"
  source "device/common/clear-factory-images-variables.sh"

  DEVICE="${device}"
  BOOTLOADER=$(get_radio_image bootloader "google_devices/${DEVICE}")
  RADIO=$(get_radio_image baseband "google_devices/${DEVICE}")
  PREFIX="aosp_"
  BUILD="${BUILD_NUMBER}"
  VERSION=$(grep -Po "BUILD_ID=\K.+" "build/core/build_id.mk" | tr '[:upper:]' '[:lower:]')
  PRODUCT="${DEVICE}"
  TARGET_FILES="${DEVICE}-target_files-${BUILD}.zip"

  # make sure output directory exists
  mkdir -p "${OUT}"

  # pick avb mode depending on device and determine key size
  avb_key_size=$(openssl rsa -in "${KEY_DIR}/avb.pem" -text -noout | grep 'Private-Key' | awk -F'[()]' '{print $2}' | awk '{print $1}')
  log "avb_key_size=${avb_key_size}"
  avb_algorithm="SHA256_RSA${avb_key_size}"
  log "avb_algorithm=${avb_algorithm}"
  case "${AVB_MODE}" in
    vbmeta_simple)
      # Pixel 2: one vbmeta struct, no chaining
      AVB_SWITCHES=(--avb_vbmeta_key "${KEY_DIR}/avb.pem"
                    --avb_vbmeta_algorithm "${avb_algorithm}")
      ;;
    vbmeta_chained)
      # Pixel 3: main vbmeta struct points to a chained vbmeta struct in system.img
      AVB_SWITCHES=(--avb_vbmeta_key "${KEY_DIR}/avb.pem"
                    --avb_vbmeta_algorithm "${avb_algorithm}"
                    --avb_system_key "${KEY_DIR}/avb.pem"
                    --avb_system_algorithm "${avb_algorithm}")
      ;;
    vbmeta_chained_v2)
      AVB_SWITCHES=(--avb_vbmeta_key "${KEY_DIR}/avb.pem"
                    --avb_vbmeta_algorithm "${avb_algorithm}"
                    --avb_system_key "${KEY_DIR}/avb.pem"
                    --avb_system_algorithm "${avb_algorithm}"
                    --avb_vbmeta_system_key "${KEY_DIR}/avb.pem"
                    --avb_vbmeta_system_algorithm "${avb_algorithm}")
      ;;
  esac

  export PATH="${RELEASE_TOOLS_DIR}/bin:${PATH}"
  export PATH="${AOSP_BUILD_DIR}/prebuilts/jdk/jdk9/linux-x86/bin:${PATH}"

  log "Running sign_target_files_apks"
  "${RELEASE_TOOLS_DIR}/releasetools/sign_target_files_apks" \
	-o -d "${KEY_DIR}" \
	-k "${AOSP_BUILD_DIR}/build/target/product/security/networkstack=${KEY_DIR}/networkstack" "${AVB_SWITCHES[@]}" \
	"${AOSP_BUILD_DIR}/out/target/product/${DEVICE}/obj/PACKAGING/target_files_intermediates/${PREFIX}${DEVICE}-target_files-${BUILD_NUMBER}.zip" \
	"${OUT}/${TARGET_FILES}"

  log "Running ota_from_target_files"
  "${RELEASE_TOOLS_DIR}/releasetools/ota_from_target_files" --block -k "${KEY_DIR}/releasekey" "${EXTRA_OTA[@]}" "${OUT}/${TARGET_FILES}" \
      "${OUT}/${DEVICE}-ota_update-${BUILD}.zip"

  log "Running img_from_target_files"
  "${RELEASE_TOOLS_DIR}/releasetools/img_from_target_files" "${OUT}/${TARGET_FILES}" "${OUT}/${DEVICE}-img-${BUILD}.zip"

  log "Running generate-factory-images"
  cd "${OUT}"
  source "../../device/common/generate-factory-images-common.sh"
  mv "${DEVICE}"-*-factory-*.zip "${DEVICE}-factory-${BUILD_NUMBER}.zip"
}

upload() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}/out"

  build_date="$(< soong/build_number.txt)"
  build_timestamp="$(unzip -p "release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "META-INF/com/android/metadata" | grep 'post-timestamp' | cut --delimiter "=" --fields 2)"
  old_metadata=$(get_current_metadata "${RELEASE_CHANNEL}")
  old_date="$(cut -d ' ' -f 1 <<< "${old_metadata}")"

  # upload ota and set metadata
  upload_build_artifact "${AOSP_BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "${DEVICE}-ota_update-${build_date}.zip" "public"
  set_current_metadata "${RELEASE_CHANNEL}" "${build_date} ${build_timestamp} ${AOSP_BUILD_ID}" "public"
  set_current_metadata "${BUILD_TIMESTAMP}" "${RELEASE_CHANNEL}-true-timestamp" "public"

  # cleanup old ota
  delete_build_artifact "${DEVICE}-ota_update-${old_date}.zip"

  # upload factory image
  upload_build_artifact "${AOSP_BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-factory-${build_date}.zip" "${DEVICE}-factory-latest.zip"
}

checkpoint_versions() {
  log_header "${FUNCNAME[0]}"

  set_current_metadata "rattlesnakeos-stack/revision" "${STACK_VERSION}"
  set_current_metadata "fdroid/revision" "${FDROID_CLIENT_VERSION}"
  set_current_metadata "fdroid-priv/revision" "${FDROID_PRIV_EXT_VERSION}"
  set_current_metadata "fdroid-priv/revision" "${FDROID_PRIV_EXT_VERSION}"
  set_current_metadata "fdroid-priv/revision" "${FDROID_PRIV_EXT_VERSION}"
  set_current_metadata "${DEVICE}-vendor" "${AOSP_BUILD_ID}" "public"
  set_current_metadata "chromium/included" "yes"
}

gen_keys() {
  log_header "${FUNCNAME[0]}"

  # download make_key and avbtool as aosp tree isn't downloaded yet
  make_key="${MISC_DIR}/make_key"
  retry curl --fail -s "https://android.googlesource.com/platform/development/+/refs/tags/${AOSP_TAG}/tools/make_key?format=TEXT" | base64 --decode > "${make_key}"
  chmod +x "${make_key}"
  avb_tool="${MISC_DIR}/avbtool"
  retry curl --fail -s "https://android.googlesource.com/platform/external/avb/+/refs/tags/${AOSP_TAG}/avbtool?format=TEXT" | base64 --decode > "${avb_tool}"
  chmod +x "${avb_tool}"

  # generate releasekey,platform,shared,media,networkstack keys
  mkdir -p "${KEYS_DIR}/${DEVICE}"
  cd "${KEYS_DIR}/${DEVICE}"
  for key in {releasekey,platform,shared,media,networkstack} ; do
    # make_key exits with unsuccessful code 1 instead of 0, need ! to negate
    ! "${make_key}" "${key}" "${CERTIFICATE_SUBJECT}"
  done

  # generate avb key
  openssl genrsa -out "${KEYS_DIR}/${DEVICE}/avb.pem" 4096
  "${avb_tool}" extract_public_key --key "${KEYS_DIR}/${DEVICE}/avb.pem" --output "${KEYS_DIR}/${DEVICE}/avb_pkmd.bin"
}

cleanup() {
  rv=$?
  logging
  if [ $rv -ne 0 ]; then
    notify "RattlesnakeOS Build FAILED" 1
  fi
  sudo shutdown -h now
}

log_header() {
  echo "=================================="
  echo "$(date "+%Y-%m-%d %H:%M:%S"): Running $1"
  echo "=================================="
}

log() {
  echo "$(date "+%Y-%m-%d %H:%M:%S"): $1"
}

retry() {
  set +e
  local max_attempts=${ATTEMPTS-3}
  local timeout=${TIMEOUT-1}
  local attempt=0
  local exitCode=0

  while [[ $attempt < $max_attempts ]]
  do
    "$@"
    exitCode=$?

    if [[ $exitCode == 0 ]]
    then
      break
    fi

    log "Failure! Retrying ($*) in $timeout.."
    sleep "${timeout}"
    attempt=$(( attempt + 1 ))
    timeout=$(( timeout * 2 ))
  done

  if [[ $exitCode != 0 ]]
  then
    log "Failed too many times! ($*)"
  fi

  set -e

  return $exitCode
}

patch_updater() {
  log_header "${FUNCNAME[0]}"

  cd "${AOSP_BUILD_DIR}/packages/apps/Updater/res/values"
  sed --in-place --expression "s@s3bucket@${RELEASE_URL}/@g" config.xml

  # TODO: just a hack to get 11 up and running
  # related commit: https://android.googlesource.com/platform/system/sepolicy/+/d61b0ce1bc8de2560f1fa173c8d01a09d039a12a%5E%21/#F0
  cat << 'EOF' > "${MISC_DIR}/updater-selinux.patch"
From 33fa92c37dd0101164a55ea1584cef6450fa641b Mon Sep 17 00:00:00 2001
From: Daniel Micay <danielmicay@gmail.com>
Date: Tue, 15 Sep 2020 00:08:40 -0400
Subject: [PATCH] add SELinux domain for Updater app

---
 prebuilts/api/30.0/private/app_neverallows.te   |  2 +-
 .../30.0/private/compat/29.0/29.0.ignore.cil    |  1 +
 prebuilts/api/30.0/private/seapp_contexts       |  9 +++++----
 prebuilts/api/30.0/private/updater_app.te       | 17 +++++++++++++++++
 prebuilts/api/30.0/public/update_engine.te      |  1 +
 prebuilts/api/30.0/public/updater_app.te        |  5 +++++
 private/app_neverallows.te                      |  2 +-
 private/compat/29.0/29.0.ignore.cil             |  1 +
 private/seapp_contexts                          |  1 +
 private/updater_app.te                          | 17 +++++++++++++++++
 public/update_engine.te                         |  1 +
 public/updater_app.te                           |  5 +++++
 12 files changed, 56 insertions(+), 6 deletions(-)
 create mode 100644 prebuilts/api/30.0/private/updater_app.te
 create mode 100644 prebuilts/api/30.0/public/updater_app.te
 create mode 100644 private/updater_app.te
 create mode 100644 public/updater_app.te

diff --git a/prebuilts/api/30.0/private/app_neverallows.te b/prebuilts/api/30.0/private/app_neverallows.te
index 115718700..32980b354 100644
--- a/prebuilts/api/30.0/private/app_neverallows.te
+++ b/prebuilts/api/30.0/private/app_neverallows.te
@@ -130,7 +130,7 @@ neverallow { all_untrusted_apps -mediaprovider } { cache_file cache_recovery_fil
 # World accessible data locations allow application to fill the device
 # with unaccounted for data. This data will not get removed during
 # application un-installation.
-neverallow { all_untrusted_apps -mediaprovider } {
+neverallow { all_untrusted_apps -mediaprovider -updater_app } {
   fs_type
   -sdcard_type
   file_type
diff --git a/prebuilts/api/30.0/private/compat/29.0/29.0.ignore.cil b/prebuilts/api/30.0/private/compat/29.0/29.0.ignore.cil
index fdea691ea..730695e8e 100644
--- a/prebuilts/api/30.0/private/compat/29.0/29.0.ignore.cil
+++ b/prebuilts/api/30.0/private/compat/29.0/29.0.ignore.cil
@@ -113,6 +113,7 @@
     traced_perf_socket
     timezonedetector_service
     untrusted_app_29
+    updater_app
     usb_serial_device
     userspace_reboot_config_prop
     userspace_reboot_exported_prop
diff --git a/prebuilts/api/30.0/private/seapp_contexts b/prebuilts/api/30.0/private/seapp_contexts
index a8c61be8f..e8951230d 100644
--- a/prebuilts/api/30.0/private/seapp_contexts
+++ b/prebuilts/api/30.0/private/seapp_contexts
@@ -162,10 +162,11 @@ user=_app isPrivApp=true name=com.android.providers.media.module domain=mediapro
 user=_app isPrivApp=true name=com.google.android.providers.media.module domain=mediaprovider_app type=privapp_data_file levelFrom=all
 user=_app seinfo=platform isPrivApp=true name=com.android.permissioncontroller domain=permissioncontroller_app type=privapp_data_file levelFrom=all
 user=_app isPrivApp=true name=com.android.vzwomatrigger domain=vzwomatrigger_app type=privapp_data_file levelFrom=all
 user=_app isPrivApp=true name=com.google.android.gms domain=gmscore_app type=privapp_data_file levelFrom=user
 user=_app isPrivApp=true name=com.google.android.gms.* domain=gmscore_app type=privapp_data_file levelFrom=user
 user=_app isPrivApp=true name=com.google.android.gms:* domain=gmscore_app type=privapp_data_file levelFrom=user
 user=_app isPrivApp=true name=com.google.android.gsf domain=gmscore_app type=privapp_data_file levelFrom=user
+user=_app isPrivApp=true name=app.seamlessupdate.client domain=updater_app type=app_data_file levelFrom=user
 user=_app minTargetSdkVersion=30 domain=untrusted_app type=app_data_file levelFrom=all
 user=_app minTargetSdkVersion=29 domain=untrusted_app_29 type=app_data_file levelFrom=all
 user=_app minTargetSdkVersion=28 domain=untrusted_app_27 type=app_data_file levelFrom=all
diff --git a/prebuilts/api/30.0/private/updater_app.te b/prebuilts/api/30.0/private/updater_app.te
new file mode 100644
index 000000000..0ce047b97
--- /dev/null
+++ b/prebuilts/api/30.0/private/updater_app.te
@@ -0,0 +1,17 @@
+###
+### Updater app
+###
+
+typeattribute updater_app coredomain;
+
+app_domain(updater_app)
+untrusted_app_domain(updater_app)
+net_domain(updater_app)
+
+# Write to /data/ota_package for OTA packages.
+allow updater_app ota_package_file:dir rw_dir_perms;
+allow updater_app ota_package_file:file create_file_perms;
+
+# Allow Updater to communicate with update_engine for A/B update.
+binder_call(updater_app, update_engine)
+allow updater_app update_engine_service:service_manager find;
diff --git a/prebuilts/api/30.0/public/update_engine.te b/prebuilts/api/30.0/public/update_engine.te
index 8b767bea0..4dd951c9d 100644
--- a/prebuilts/api/30.0/public/update_engine.te
+++ b/prebuilts/api/30.0/public/update_engine.te
@@ -46,6 +46,7 @@ userdebug_or_eng(` + "`" + `
 ')

 binder_call(update_engine, gmscore_app)
+binder_call(update_engine, updater_app)

 # Allow update_engine to call the callback function provided by system_server.
 binder_call(update_engine, system_server)
diff --git a/prebuilts/api/30.0/public/updater_app.te b/prebuilts/api/30.0/public/updater_app.te
new file mode 100644
index 000000000..97a850ba1
--- /dev/null
+++ b/prebuilts/api/30.0/public/updater_app.te
@@ -0,0 +1,5 @@
+###
+### Updater app
+###
+
+type updater_app, domain;
diff --git a/private/app_neverallows.te b/private/app_neverallows.te
index 115718700..32980b354 100644
--- a/private/app_neverallows.te
+++ b/private/app_neverallows.te
@@ -130,7 +130,7 @@ neverallow { all_untrusted_apps -mediaprovider } { cache_file cache_recovery_fil
 # World accessible data locations allow application to fill the device
 # with unaccounted for data. This data will not get removed during
 # application un-installation.
-neverallow { all_untrusted_apps -mediaprovider } {
+neverallow { all_untrusted_apps -mediaprovider -updater_app } {
   fs_type
   -sdcard_type
   file_type
diff --git a/private/compat/29.0/29.0.ignore.cil b/private/compat/29.0/29.0.ignore.cil
index fdea691ea..730695e8e 100644
--- a/private/compat/29.0/29.0.ignore.cil
+++ b/private/compat/29.0/29.0.ignore.cil
@@ -113,6 +113,7 @@
     traced_perf_socket
     timezonedetector_service
     untrusted_app_29
+    updater_app
     usb_serial_device
     userspace_reboot_config_prop
     userspace_reboot_exported_prop
diff --git a/private/seapp_contexts b/private/seapp_contexts
index d0898bd16..e8951230d 100644
--- a/private/seapp_contexts
+++ b/private/seapp_contexts
@@ -166,6 +166,7 @@ user=_app isPrivApp=true name=com.android.vzwomatrigger domain=vzwomatrigger_app
 user=_app isPrivApp=true name=com.google.android.gms.* domain=gmscore_app type=privapp_data_file levelFrom=user
 user=_app isPrivApp=true name=com.google.android.gms:* domain=gmscore_app type=privapp_data_file levelFrom=user
 user=_app isPrivApp=true name=com.google.android.gsf domain=gmscore_app type=privapp_data_file levelFrom=user
+user=_app isPrivApp=true name=app.seamlessupdate.client domain=updater_app type=app_data_file levelFrom=user
 user=_app minTargetSdkVersion=30 domain=untrusted_app type=app_data_file levelFrom=all
 user=_app minTargetSdkVersion=29 domain=untrusted_app_29 type=app_data_file levelFrom=all
 user=_app minTargetSdkVersion=28 domain=untrusted_app_27 type=app_data_file levelFrom=all
diff --git a/private/updater_app.te b/private/updater_app.te
new file mode 100644
index 000000000..0ce047b97
--- /dev/null
+++ b/private/updater_app.te
@@ -0,0 +1,17 @@
+###
+### Updater app
+###
+
+typeattribute updater_app coredomain;
+
+app_domain(updater_app)
+untrusted_app_domain(updater_app)
+net_domain(updater_app)
+
+# Write to /data/ota_package for OTA packages.
+allow updater_app ota_package_file:dir rw_dir_perms;
+allow updater_app ota_package_file:file create_file_perms;
+
+# Allow Updater to communicate with update_engine for A/B update.
+binder_call(updater_app, update_engine)
+allow updater_app update_engine_service:service_manager find;
diff --git a/public/update_engine.te b/public/update_engine.te
index 8b767bea0..4dd951c9d 100644
--- a/public/update_engine.te
+++ b/public/update_engine.te
@@ -46,6 +46,7 @@ userdebug_or_eng(` + "`" + `
 ')

 binder_call(update_engine, gmscore_app)
+binder_call(update_engine, updater_app)

 # Allow update_engine to call the callback function provided by system_server.
 binder_call(update_engine, system_server)
diff --git a/public/updater_app.te b/public/updater_app.te
new file mode 100644
index 000000000..97a850ba1
--- /dev/null
+++ b/public/updater_app.te
@@ -0,0 +1,5 @@
+###
+### Updater app
+###
+
+type updater_app, domain;
--
EOF
  pushd "${AOSP_BUILD_DIR}/system/sepolicy"
  git apply "${MISC_DIR}/updater-selinux.patch"
  popd
}

fdpe_hash() {
  keytool -printcert -file "$1" | grep 'SHA256:' | tr --delete ':' | cut --delimiter ' ' --fields 3
}

trap cleanup 0

set -e

full_run