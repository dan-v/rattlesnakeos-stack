#!/usr/bin/env bash

########################################
######## BUILD ARGS ####################
########################################
RELEASE=$1
echo "RELEASE=${RELEASE}"
AOSP_BUILD_ID=$2
echo "AOSP_BUILD_ID=${AOSP_BUILD_ID}"
AOSP_TAG=$3
echo "AOSP_TAG=${AOSP_TAG}"
CHROMIUM_VERSION=$4
echo "CHROMIUM_VERSION=${CHROMIUM_VERSION}"
CHROMIUM_FORCE_BUILD=$5
echo "CHROMIUM_FORCE_BUILD=${CHROMIUM_FORCE_BUILD}"
LOCAL_MANIFEST_REVISIONS=$6
echo "LOCAL_MANIFEST_REVISIONS=${LOCAL_MANIFEST_REVISIONS}"

#### <generated_vars_and_funcs.sh> ####

########################################
######## OTHER VARS ####################
########################################
SECONDS=0
ROOT_DIR="${HOME}"
AOSP_BUILD_DIR="${ROOT_DIR}/aosp"
CORE_DIR="${ROOT_DIR}/core"
CUSTOM_DIR="${ROOT_DIR}/custom"
KEYS_DIR="${ROOT_DIR}/keys"
MISC_DIR="${ROOT_DIR}/misc"
RELEASE_TOOLS_DIR="${MISC_DIR}/releasetools"
PRODUCT_MAKEFILE="${AOSP_BUILD_DIR}/device/google/${DEVICE_FAMILY}/aosp_${DEVICE}.mk"
CORE_VENDOR_BASEDIR="${AOSP_BUILD_DIR}/vendor/core"
CORE_VENDOR_MAKEFILE="${CORE_VENDOR_BASEDIR}/vendor/config/main.mk"
CUSTOM_VENDOR_BASEDIR="${AOSP_BUILD_DIR}/vendor/custom"
CUSTOM_VENDOR_MAKEFILE="${CUSTOM_VENDOR_BASEDIR}/vendor/config/main.mk"

full_run() {
  log_header "${FUNCNAME[0]}"

  notify "RattlesnakeOS Build STARTED"
  setup_env
  import_keys
  aosp_repo_init
  aosp_local_repo_additions
  aosp_repo_sync
  chromium_build_if_required
  chromium_copy_to_build_tree_if_required
  setup_vendor
  aosp_build
  release
  upload
  checkpoint_versions
  notify "RattlesnakeOS Build SUCCESS"
}

setup_env() {
  log_header "${FUNCNAME[0]}"

  # install required packages
  sudo apt-get update
  sudo DEBIAN_FRONTEND=noninteractive apt-get -y install python python2 python3 gperf jq default-jdk git-core gnupg \
      flex bison build-essential zip curl zlib1g-dev gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev \
      x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev libxml2-utils xsltproc unzip liblz4-tool \
      libncurses5 wget parallel rsync python-protobuf python3-protobuf python3-pip

  retry curl --fail -s https://storage.googleapis.com/git-repo-downloads/repo > /tmp/repo
  chmod +x /tmp/repo
  sudo mv /tmp/repo /usr/local/bin/

  # setup git
  git config --get --global user.name || git config --global user.name 'aosp'
  git config --get --global user.email || git config --global user.email 'aosp@localhost'
  git config --global color.ui true

  # mount /tmp filesystem as tmpfs
  sudo mount -t tmpfs tmpfs /tmp || true

  # setup base directories
  mkdir -p "${AOSP_BUILD_DIR}"
  mkdir -p "${KEYS_DIR}"
  mkdir -p "${MISC_DIR}"
  mkdir -p "${RELEASE_TOOLS_DIR}"

  # get core repo
  rm -rf "${CORE_DIR}"
  retry git clone "${CORE_CONFIG_REPO}" "${CORE_DIR}"
  if [ -n "${CORE_CONFIG_REPO_BRANCH}" ]; then
    pushd "${CORE_DIR}"
    git checkout "${CORE_CONFIG_REPO_BRANCH}"
    popd
  fi

  # get custom repo if specified
  if [ -n "${CUSTOM_CONFIG_REPO}" ]; then
    rm -rf "${CUSTOM_DIR}"
    retry git clone "${CUSTOM_CONFIG_REPO}" "${CUSTOM_DIR}"
    if [ -n "${CUSTOM_CONFIG_REPO_BRANCH}" ]; then
      pushd "${CUSTOM_DIR}"
      git checkout "${CUSTOM_CONFIG_REPO_BRANCH}"
      popd
    fi
  fi

  # mount keys directory as tmpfs
  sudo mount -t tmpfs -o size=20m tmpfs "${KEYS_DIR}" || true
}

aosp_repo_init() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  run_hook_if_exists "aosp_repo_init_pre"

  MANIFEST_URL="https://android.googlesource.com/platform/manifest"
  retry repo init --manifest-url "${MANIFEST_URL}" --manifest-branch "${AOSP_TAG}" --depth 1

  run_hook_if_exists "aosp_repo_init_post"
}

aosp_local_repo_additions() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  run_hook_if_exists "aosp_local_repo_additions_pre"

  rm -rf "${AOSP_BUILD_DIR}/.repo/local_manifests"
  mkdir -p "${AOSP_BUILD_DIR}/.repo/local_manifests"
  cp -f "${CORE_DIR}"/local_manifests/*.xml "${AOSP_BUILD_DIR}/.repo/local_manifests"

  if [ "${CHROMIUM_BUILD_DISABLED}" == "true" ]; then
    local_chromium_manifest="${AOSP_BUILD_DIR}/.repo/local_manifests/001-chromium.xml"
    if [ -f "${local_chromium_manifest}" ]; then
      log "Removing ${local_chromium_manifest} as chromium build is disabled"
      rm -f "${local_chromium_manifest}" || true
    fi
  fi

  if [ -n "${CUSTOM_CONFIG_REPO}" ]; then
    cp -f "${CUSTOM_DIR}"/local_manifests/*.xml "${AOSP_BUILD_DIR}/.repo/local_manifests" || true
  fi

  run_hook_if_exists "aosp_local_repo_additions_post"
}

aosp_repo_sync() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  run_hook_if_exists "aosp_repo_sync_pre"

  if [ "$(ls -l "${AOSP_BUILD_DIR}" | wc -l)" -gt 0 ]; then
    log "Running git reset and clean as environment appears to already have been synced previously"
    repo forall -c 'git reset --hard ; git clean --force -dx'
  fi

  for i in {1..10}; do
    log "Running aosp repo sync attempt ${i}/10"
    repo sync -c --no-tags --no-clone-bundle --jobs 32 && break
  done

  run_hook_if_exists "aosp_repo_sync_post"
}

setup_vendor() {
  log_header "${FUNCNAME[0]}"
  run_hook_if_exists "setup_vendor_pre"

  # skip if already downloaded
  current_vendor_build_id=""
  vendor_build_id_file="${AOSP_BUILD_DIR}/vendor/google_devices/${DEVICE}/build_id.txt"
  if [ -f "${vendor_build_id_file}" ]; then
    current_vendor_build_id=$(cat "${vendor_build_id_file}")
  fi
  if [ "${current_vendor_build_id}" == "${AOSP_BUILD_ID}" ]; then
    log "Skipping vendor download as ${AOSP_BUILD_ID} already exists at ${vendor_build_id_file}"
    return
  fi

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

  run_hook_if_exists "setup_vendor_post"
}

insert_vendor_includes() {
  log_header "${FUNCNAME[0]}"

  if ! grep -q "${CORE_VENDOR_MAKEFILE}" "${PRODUCT_MAKEFILE}"; then
    sed -i "\@vendor/google_devices/${DEVICE_FAMILY}/proprietary/device-vendor.mk)@a \$(call inherit-product, ${CORE_VENDOR_MAKEFILE})" "${PRODUCT_MAKEFILE}"
  fi

  if [ -n "${CUSTOM_CONFIG_REPO}" ]; then
    if ! grep -q "${CUSTOM_VENDOR_MAKEFILE}" "${PRODUCT_MAKEFILE}"; then
      sed -i "\@vendor/google_devices/${DEVICE_FAMILY}/proprietary/device-vendor.mk)@a \$(call inherit-product, ${CUSTOM_VENDOR_MAKEFILE})" "${PRODUCT_MAKEFILE}"
    fi
  fi
}

env_setup_script() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  source build/envsetup.sh
  export LANG=C
  export _JAVA_OPTIONS=-XX:-UsePerfData
  # shellcheck disable=SC2155
  export BUILD_NUMBER=$(cat out/soong/build_number.txt 2>/dev/null || date --utc +%Y.%m.%d.%H)
  log "BUILD_NUMBER=${BUILD_NUMBER}"
  export DISPLAY_BUILD_NUMBER=true
  chrt -b -p 0 $$
}

aosp_build() {
  log_header "${FUNCNAME[0]}"
  run_hook_if_exists "aosp_build_pre"
  cd "${AOSP_BUILD_DIR}"

  insert_vendor_includes

  if [ "${CHROMIUM_BUILD_DISABLED}" == "true" ]; then
    log "Removing TrichromeChrome and TrichromeWebView as chromium build is disabled"
    sed -i '/PRODUCT_PACKAGES += TrichromeChrome/d' "${CORE_VENDOR_MAKEFILE}" || true
    sed -i '/PRODUCT_PACKAGES += TrichromeWebView/d' "${CORE_VENDOR_MAKEFILE}" || true
  fi

  (
    env_setup_script

    build_target="release aosp_${DEVICE} user"
    log "Running choosecombo ${build_target}"
    choosecombo ${build_target}

    log "Running target-files-package"
    retry m target-files-package

    if [ ! -f "${RELEASE_TOOLS_DIR}/releasetools/sign_target_files_apks" ]; then
      log "Running m otatools-package"
      m otatools-package
      rm -rf "${RELEASE_TOOLS_DIR}"
      unzip "${AOSP_BUILD_DIR}/out/target/product/${DEVICE}/otatools.zip" -d "${RELEASE_TOOLS_DIR}"
    fi
  )

  run_hook_if_exists "aosp_build_post"
}

release() {
  log_header "${FUNCNAME[0]}"
  run_hook_if_exists "release_pre"
  cd "${AOSP_BUILD_DIR}"

  (
    env_setup_script

    KEY_DIR="${KEYS_DIR}/${DEVICE}"
    OUT="out/release-${DEVICE}-${BUILD_NUMBER}"
    device="${DEVICE}"

    log "Running clear-factory-images-variables.sh"
    source "device/common/clear-factory-images-variables.sh"
    DEVICE="${device}"
    PREFIX="aosp_"
    BUILD="${BUILD_NUMBER}"
    # shellcheck disable=SC2034
    PRODUCT="${DEVICE}"
    TARGET_FILES="${DEVICE}-target_files-${BUILD}.zip"
    BOOTLOADER=$(grep -Po "require version-bootloader=\K.+" "vendor/google_devices/${DEVICE}/vendor-board-info.txt" | tr '[:upper:]' '[:lower:]')
    RADIO=$(grep -Po "require version-baseband=\K.+" "vendor/google_devices/${DEVICE}/vendor-board-info.txt" | tr '[:upper:]' '[:lower:]')
    VERSION=$(grep -Po "BUILD_ID=\K.+" "build/core/build_id.mk" | tr '[:upper:]' '[:lower:]')
    log "BOOTLOADER=${BOOTLOADER} RADIO=${RADIO} VERSION=${VERSION} TARGET_FILES=${TARGET_FILES}"

    # make sure output directory exists
    mkdir -p "${OUT}"

    # pick avb mode depending on device and determine key size
    avb_key_size=$(openssl rsa -in "${KEY_DIR}/avb.pem" -text -noout | grep 'Private-Key' | awk -F'[()]' '{print $2}' | awk '{print $1}')
    log "avb_key_size=${avb_key_size}"
    avb_algorithm="SHA256_RSA${avb_key_size}"
    log "avb_algorithm=${avb_algorithm}"
    case "${DEVICE_AVB_MODE}" in
      vbmeta_chained)
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
    # shellcheck disable=SC2068
    "${RELEASE_TOOLS_DIR}/releasetools/ota_from_target_files" --block -k "${KEY_DIR}/releasekey" ${DEVICE_EXTRA_OTA[@]} "${OUT}/${TARGET_FILES}" \
      "${OUT}/${DEVICE}-ota_update-${BUILD}.zip"

    log "Running img_from_target_files"
    "${RELEASE_TOOLS_DIR}/releasetools/img_from_target_files" "${OUT}/${TARGET_FILES}" "${OUT}/${DEVICE}-img-${BUILD}.zip"

    log "Running generate-factory-images"
    cd "${OUT}"
    source "../../device/common/generate-factory-images-common.sh"
    mv "${DEVICE}"-*-factory-*.zip "${DEVICE}-factory-${BUILD_NUMBER}.zip"
  )

  run_hook_if_exists "release_post"
}

upload() {
  log_header "${FUNCNAME[0]}"
  run_hook_if_exists "upload_pre"
  cd "${AOSP_BUILD_DIR}/out"

  build_channel="stable"
  release_channel="${DEVICE}-${build_channel}"
  build_date="$(< soong/build_number.txt)"
  build_timestamp="$(unzip -p "release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "META-INF/com/android/metadata" | grep 'post-timestamp' | cut --delimiter "=" --fields 2)"
  old_metadata=$(get_current_metadata "${release_channel}")
  old_date="$(cut -d ' ' -f 1 <<< "${old_metadata}")"

  # upload ota and set metadata
  upload_build_artifact "${AOSP_BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "${DEVICE}-ota_update-${build_date}.zip" "public"
  set_current_metadata "${release_channel}" "${build_date} ${build_timestamp} ${AOSP_BUILD_ID}" "public"
  set_current_metadata "${release_channel}-true-timestamp" "${build_timestamp}" "public"

  # cleanup old ota
  delete_build_artifact "${DEVICE}-ota_update-${old_date}.zip"

  # upload factory image
  upload_build_artifact "${AOSP_BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-factory-${build_date}.zip" "${DEVICE}-factory-latest.zip"

  run_hook_if_exists "upload_post"
}

checkpoint_versions() {
  log_header "${FUNCNAME[0]}"
  run_hook_if_exists "checkpoint_versions_pre"

  set_current_metadata "${DEVICE}-vendor" "${AOSP_BUILD_ID}" "public"
  set_current_metadata "release" "${RELEASE}"
  set_current_metadata "rattlesnakeos-stack/revision" "${STACK_VERSION}"
  if [ "${CHROMIUM_BUILD_DISABLED}" == "false" ]; then
      set_current_metadata "chromium/included" "yes"
  fi

  run_hook_if_exists "checkpoint_versions_post"
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
    ! "${make_key}" "${key}" "/CN=RattlesnakeOS"
  done

  # generate avb key
  openssl genrsa -out "${KEYS_DIR}/${DEVICE}/avb.pem" 4096
  "${avb_tool}" extract_public_key --key "${KEYS_DIR}/${DEVICE}/avb.pem" --output "${KEYS_DIR}/${DEVICE}/avb_pkmd.bin"

  # generate chromium.keystore
  cd "${KEYS_DIR}/${DEVICE}"
  keytool -genkey -v -keystore chromium.keystore -storetype pkcs12 -alias chromium -keyalg RSA -keysize 4096 \
        -sigalg SHA512withRSA -validity 10000 -dname "cn=RattlesnakeOS" -storepass chromium
}

run_hook_if_exists() {
  local hook_name="${1}"
  local core_hook_file="${CORE_DIR}/hooks/${hook_name}.sh"
  local custom_hook_file="${CUSTOM_DIR}/hooks/${hook_name}.sh"

  if [ -n "${core_hook_file}" ]; then
    if [ -f "${core_hook_file}" ]; then
      log "Running ${core_hook_file}"
      # shellcheck disable=SC1090
      (source "${core_hook_file}")
    fi
  fi

  if [ -n "${custom_hook_file}" ]; then
    if [ -f "${custom_hook_file}" ]; then
      log "Running ${custom_hook_file}"
      # shellcheck disable=SC1090
      (source "${custom_hook_file}")
    fi
  fi
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

########################################
######## CHROMIUM ######################
########################################

chromium_build_if_required() {
  log_header "${FUNCNAME[0]}"

  if [ "${CHROMIUM_BUILD_DISABLED}" == "true" ]; then
    log "Chromium build is disabled"
    return
  fi

  current=$(get_current_metadata "chromium/revision")
  log "Chromium current: ${current}"

  log "Chromium requested: ${CHROMIUM_VERSION}"
  if [ "${CHROMIUM_VERSION}" == "${current}" ] && [ "${CHROMIUM_FORCE_BUILD}" != "true" ]; then
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
  CHROMIUM_DEFAULT_VERSION=$(echo "${CHROMIUM_REVISION}" | awk -F"." '{ printf "%s%03d52\n",$3,$4}')

  (
    # depot tools setup
    if [ ! -d "${MISC_DIR}/depot_tools" ]; then
      retry git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git "${MISC_DIR}/depot_tools"
    fi
    export PATH="${PATH}:${MISC_DIR}/depot_tools"

    # fetch chromium
    CHROMIUM_BUILD_DIR="${ROOT_DIR}/chromium"
    mkdir -p "${CHROMIUM_BUILD_DIR}"
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

    # apply required patches
    if [ -n "$(ls -A ${AOSP_BUILD_DIR}/external/chromium/patches)" ]; then
      git am --whitespace=nowarn ${AOSP_BUILD_DIR}/external/chromium/patches/*.patch
    fi

    # generate configuration
    KEYSTORE="${KEYS_DIR}/${DEVICE}/chromium.keystore"
    trichrome_certdigest=$(keytool -export-cert -alias chromium -keystore "${KEYSTORE}" -storepass chromium | sha256sum | awk '{print $1}')
    log "trichrome_certdigest=${trichrome_certdigest}"
    mkdir -p out/Default
    cp -f "${AOSP_BUILD_DIR}/external/chromium/args.gn" out/Default/args.gn
    cat <<EOF >> out/Default/args.gn

android_default_version_name = "${CHROMIUM_REVISION}"
android_default_version_code = "${CHROMIUM_DEFAULT_VERSION}"
trichrome_certdigest = "${trichrome_certdigest}"
chrome_public_manifest_package = "org.chromium.chrome"
system_webview_package_name = "org.chromium.webview"
trichrome_library_package = "org.chromium.trichromelibrary"
EOF
    gn gen out/Default

    run_hook_if_exists "build_chromium_pre"

    log "Building trichrome"
    autoninja -C out/Default/ trichrome_webview_64_32_apk trichrome_chrome_64_32_apk trichrome_library_64_32_apk

    log "Signing trichrome"
    APKSIGNER="${CHROMIUM_BUILD_DIR}/src/third_party/android_sdk/public/build-tools/30.0.1/apksigner"
    cd out/Default/apks
    rm -rf release
    mkdir release
    cd release
    for app in TrichromeChrome TrichromeLibrary TrichromeWebView; do
      "${APKSIGNER}" sign --ks "${KEYSTORE}" --ks-pass pass:chromium --ks-key-alias chromium --in "../${app}6432.apk" --out "${app}.apk"
    done

    log "Uploading trichrome apks"
    upload_build_artifact "TrichromeLibrary.apk" "chromium/TrichromeLibrary.apk"
    upload_build_artifact "TrichromeWebView.apk" "chromium/TrichromeWebView.apk"
    upload_build_artifact "TrichromeChrome.apk" "chromium/TrichromeChrome.apk"
    set_current_metadata "chromium/revision" "${CHROMIUM_REVISION}"

    run_hook_if_exists "build_chromium_post"
  )
}

chromium_copy_to_build_tree_if_required() {
  log_header "${FUNCNAME[0]}"

  if [ "${CHROMIUM_BUILD_DISABLED}" == "true" ]; then
    log "Chromium build is disabled"
    return
  fi

  # add latest built chromium to external/chromium
  download_build_artifact "chromium/TrichromeLibrary.apk" "${AOSP_BUILD_DIR}/external/chromium/prebuilt/arm64/"
  download_build_artifact "chromium/TrichromeWebView.apk" "${AOSP_BUILD_DIR}/external/chromium/prebuilt/arm64/"
  download_build_artifact "chromium/TrichromeChrome.apk" "${AOSP_BUILD_DIR}/external/chromium/prebuilt/arm64/"
}

trap cleanup 0

set -e

full_run