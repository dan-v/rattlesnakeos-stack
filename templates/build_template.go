package templates

const BuildTemplate = `#!/bin/bash

if [ $# -lt 1 ]; then
  echo "Need to specify device name as argument"
  exit 1
fi

# check if supported device
DEVICE=$1
case "${DEVICE}" in
  taimen)
    DEVICE_FAMILY=taimen
    DEVICE_COMMON=wahoo
    AVB_MODE=vbmeta_simple
    ;;
  walleye)
    DEVICE_FAMILY=muskie
    DEVICE_COMMON=wahoo
    AVB_MODE=vbmeta_simple
    ;;
  crosshatch|blueline)
    DEVICE_FAMILY=crosshatch
    DEVICE_COMMON=crosshatch
    AVB_MODE=vbmeta_chained
    EXTRA_OTA=(--retrofit_dynamic_partitions)
    ;;
  sargo|bonito)
    DEVICE_FAMILY=bonito
    DEVICE_COMMON=bonito
    AVB_MODE=vbmeta_chained
    EXTRA_OTA=(--retrofit_dynamic_partitions)
    ;;
  flame|coral)
    DEVICE_FAMILY=coral
    DEVICE_COMMON=coral
    AVB_MODE=vbmeta_chained_v2
    ;;
  sunfish)
    DEVICE_FAMILY=sunfish
    DEVICE_COMMON=sunfish
    AVB_MODE=vbmeta_chained_v2
    ;;
  *)
    echo "error: unknown device ${DEVICE}"
    exit 1
    ;;
esac

# this is a build time option to override stack setting IGNORE_VERSION_CHECKS
FORCE_BUILD=false
if [ "$2" = true ]; then
  echo "Setting FORCE_BUILD=true"
  FORCE_BUILD=true
fi

# allow build and branch to be specified
AOSP_BUILD=$3
AOSP_BRANCH=$4
AOSP_VENDOR_BUILD=

# set region
REGION=<% .Region %>
export AWS_DEFAULT_REGION=${REGION}

# stack name
STACK_NAME=<% .Name %>

# version of stack running
STACK_VERSION=<% .Version %>

# prevent default action of shutting down on exit
PREVENT_SHUTDOWN=<% .PreventShutdown %>

# whether version checks should be ignored
IGNORE_VERSION_CHECKS=<% .IgnoreVersionChecks %>

# version of chromium to pin to if requested
CHROMIUM_PINNED_VERSION=<% .ChromiumVersion %>

# whether keys are client side encrypted or not
ENCRYPTED_KEYS="<% .EncryptedKeys %>"
ENCRYPTION_KEY=
ENCRYPTION_PIPE="/tmp/key"

# pin to specific version of android
ANDROID_VERSION="11.0"

# build type (user or userdebug)
BUILD_TYPE="user"

# build channel (stable or beta)
BUILD_CHANNEL="stable"

# user customizable things
HOSTS_FILE=<% .HostsFile %>

# aws settings
AWS_KEYS_BUCKET="${STACK_NAME}-keys"
AWS_ENCRYPTED_KEYS_BUCKET="${STACK_NAME}-keys-encrypted"
AWS_RELEASE_BUCKET="${STACK_NAME}-release"
AWS_LOGS_BUCKET="${STACK_NAME}-logs"
AWS_SNS_ARN=$(aws --region ${REGION} sns list-topics --query 'Topics[0].TopicArn' --output text | cut -d":" -f1,2,3,4,5)":${STACK_NAME}"
INSTANCE_TYPE=$(curl -s http://169.254.169.254/latest/meta-data/instance-type)
INSTANCE_REGION=$(curl -s http://169.254.169.254/latest/dynamic/instance-identity/document | awk -F\" '/region/ {print $4}')
INSTANCE_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)

# build settings
SECONDS=0
BUILD_TARGET="release aosp_${DEVICE} ${BUILD_TYPE}"
RELEASE_URL="https://${AWS_RELEASE_BUCKET}.s3.amazonaws.com"
RELEASE_CHANNEL="${DEVICE}-${BUILD_CHANNEL}"
BUILD_DATE=$(date +%Y.%m.%d.%H)
BUILD_TIMESTAMP=$(date +%s)
BUILD_DIR="${HOME}/rattlesnake-os"
KEYS_DIR="${BUILD_DIR}/keys"
CERTIFICATE_SUBJECT='/CN=RattlesnakeOS'
OFFICIAL_FDROID_KEY="43238d512c1e5eb2d6569f4a3afbf5523418b82e0a3ed1552770abb9a9c9ccab"
BUILD_REASON=""

# urls
MANIFEST_URL="https://android.googlesource.com/platform/manifest"
STACK_URL_LATEST="https://api.github.com/repos/dan-v/rattlesnakeos-stack/releases/latest"
RATTLESNAKEOS_LATEST_JSON="https://raw.githubusercontent.com/RattlesnakeOS/latest/${ANDROID_VERSION}/latest.json"

STACK_UPDATE_MESSAGE=
LATEST_STACK_VERSION=
LATEST_CHROMIUM=
FDROID_CLIENT_VERSION=
FDROID_PRIV_EXT_VERSION=

full_run() {
  log_header "${FUNCNAME[0]}"

  get_latest_versions
  check_for_new_versions
  initial_key_setup
  aws_notify "RattlesnakeOS Build STARTED"
  setup_env
  aws_import_keys
  check_chromium
  aosp_repo_init
  aosp_repo_modifications
  aosp_repo_sync
  setup_vendor
  build_fdroid
  add_chromium
  apply_patches
  build_aosp
  release
  aws_upload
  checkpoint_versions
  aws_notify "RattlesnakeOS Build SUCCESS"
}

get_latest_versions() {
  log_header "${FUNCNAME[0]}"

  sudo DEBIAN_FRONTEND=noninteractive apt-get -y install jq

  # check if running latest stack
  LATEST_STACK_VERSION=$(curl --fail -s "${STACK_URL_LATEST}" | jq -r '.name')
  if [ -z "${LATEST_STACK_VERSION}" ]; then
    aws_notify_simple "ERROR: Unable to get latest rattlesnakeos-stack version details. Stopping build."
    exit 1
  elif [ "${LATEST_STACK_VERSION}" == "${STACK_VERSION}" ]; then
    log "Running the latest rattlesnakeos-stack version ${LATEST_STACK_VERSION}"
  else
    STACK_UPDATE_MESSAGE="WARNING: you should upgrade to the latest version: ${LATEST_STACK_VERSION}"
  fi

  # download latest.json and use for remaining checks
  curl --fail -s "${RATTLESNAKEOS_LATEST_JSON}" > "${HOME}/latest.json"

  # check for latest chromium version
  LATEST_CHROMIUM=$(jq -r '.chromium' "${HOME}/latest.json")
  if [ -z "${LATEST_CHROMIUM}" ]; then
    aws_notify_simple "ERROR: Unable to get latest Chromium version details. Stopping build."
    exit 1
  fi
  log "LATEST_CHROMIUM=${LATEST_CHROMIUM}"

  FDROID_CLIENT_VERSION=$(jq -r '.fdroid.client' "${HOME}/latest.json")
  if [ -z "${FDROID_CLIENT_VERSION}" ]; then
    aws_notify_simple "ERROR: Unable to get latest F-Droid version details. Stopping build."
    exit 1
  fi
  log "FDROID_CLIENT_VERSION=${FDROID_CLIENT_VERSION}"

  FDROID_PRIV_EXT_VERSION=$(jq -r '.fdroid.privilegedextention' "${HOME}/latest.json")
  if [ -z "${FDROID_PRIV_EXT_VERSION}" ]; then
    aws_notify_simple "ERROR: Unable to get latest F-Droid privilege extension version details. Stopping build."
    exit 1
  fi
  log "FDROID_PRIV_EXT_VERSION=${FDROID_PRIV_EXT_VERSION}"

  AOSP_VENDOR_BUILD=$(jq -r ".devices.${DEVICE}.build_id" "${HOME}/latest.json")
  if [ -z "${AOSP_VENDOR_BUILD}" ]; then
    aws_notify_simple "ERROR: Unable to get latest AOSP build version details. Stopping build."
    exit 1
  fi
  if [ -z "${AOSP_BUILD}" ]; then
    AOSP_BUILD="${AOSP_VENDOR_BUILD}"
  fi
  log "AOSP_VENDOR_BUILD=${AOSP_VENDOR_BUILD}"
  log "AOSP_BUILD=${AOSP_BUILD}"

  if [ -z "${AOSP_BRANCH}" ]; then
    AOSP_BRANCH=$(jq -r ".devices.${DEVICE}.aosp_tag" "${HOME}/latest.json")
    if [ -z "${AOSP_BRANCH}" ]; then
      aws_notify_simple "ERROR: Unable to get latest AOSP branch details. Stopping build."
      exit 1
    fi
  fi
  log "AOSP_BRANCH=${AOSP_BRANCH}"

}

check_for_new_versions() {
  log_header "${FUNCNAME[0]}"

  log "Checking if any new versions of software exist"
  needs_update=false

  # check stack version
  existing_stack_version=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/rattlesnakeos-stack/revision" - || true)
  if [ "${existing_stack_version}" == "${STACK_VERSION}" ]; then
    log "Stack version (${existing_stack_version}) is up to date"
  else
    log "Last successful build (if there was one) is not with current stack version ${STACK_VERSION}"
    needs_update=true
    BUILD_REASON="'Stack version ${existing_stack_version} != ${STACK_VERSION}'"
  fi

  # check aosp
  existing_aosp_build=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-vendor" - || true)
  if [ "${existing_aosp_build}" == "${AOSP_VENDOR_BUILD}" ]; then
    log "AOSP build (${existing_aosp_build}) is up to date"
  else
    log "AOSP needs to be updated to ${AOSP_VENDOR_BUILD}"
    needs_update=true
    BUILD_REASON="${BUILD_REASON} 'AOSP build ${existing_aosp_build} != ${AOSP_VENDOR_BUILD}'"
  fi

  # check chromium
  if [ -n "${CHROMIUM_PINNED_VERSION}" ]; then
    log "Setting LATEST_CHROMIUM to pinned version ${CHROMIUM_PINNED_VERSION}"
    LATEST_CHROMIUM="${CHROMIUM_PINNED_VERSION}"
  fi
  existing_chromium=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/revision" - || true)
  chromium_included=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/included" - || true)
  if [ "${existing_chromium}" == "${LATEST_CHROMIUM}" ] && [ "${chromium_included}" == "yes" ]; then
    log "Chromium build (${existing_chromium}) is up to date"
  else
    log "Chromium needs to be updated to ${LATEST_CHROMIUM}"
    echo "no" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/chromium/included"
    needs_update=true
    if [ "${existing_chromium}" == "${LATEST_CHROMIUM}" ]; then
      BUILD_REASON="${BUILD_REASON} 'Chromium version ${existing_chromium} built but not installed'"
    else
      BUILD_REASON="${BUILD_REASON} 'Chromium version ${existing_chromium} != ${LATEST_CHROMIUM}'"
    fi
  fi

  # check fdroid
  existing_fdroid_client=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/fdroid/revision" - || true)
  if [ "${existing_fdroid_client}" == "${FDROID_CLIENT_VERSION}" ]; then
    log "F-Droid build (${existing_fdroid_client}) is up to date"
  else
    log "F-Droid needs to be updated to ${FDROID_CLIENT_VERSION}"
    needs_update=true
    BUILD_REASON="${BUILD_REASON} 'F-Droid version ${existing_fdroid_client} != ${FDROID_CLIENT_VERSION}'"
  fi

  # check fdroid priv extension
  existing_fdroid_priv_version=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/fdroid-priv/revision" - || true)
  if [ "${existing_fdroid_priv_version}" == "${FDROID_PRIV_EXT_VERSION}" ]; then
    log "F-Droid privileged extension build (${existing_fdroid_priv_version}) is up to date"
  else
    log "F-Droid privileged extension needs to be updated to ${FDROID_PRIV_EXT_VERSION}"
    needs_update=true
    BUILD_REASON="${BUILD_REASON} 'F-Droid privileged extension ${existing_fdroid_priv_version} != ${FDROID_PRIV_EXT_VERSION}'"
  fi

  if [ "${needs_update}" = true ]; then
    log "New build is required"
  else
    if [ "${FORCE_BUILD}" = true ]; then
      message="No build is required, but FORCE_BUILD=true"
      log "${message}"
      BUILD_REASON="${message}"
    elif [ "${IGNORE_VERSION_CHECKS}" = true ]; then
      message="No build is required, but IGNORE_VERSION_CHECKS=true"
      log "${message}"
      BUILD_REASON="${message}"
    else
      aws_notify "RattlesnakeOS build not required as all components are already up to date."
      exit 0
    fi
  fi

  if [ -z "${existing_stack_version}" ]; then
    BUILD_REASON="Initial build"
  fi
}

add_chromium() {
  log_header "${FUNCNAME[0]}"

  # add latest built chromium to external/chromium
  aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/TrichromeLibrary.apk" "${BUILD_DIR}/external/chromium/prebuilt/arm64/"
  aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/TrichromeWebView.apk" "${BUILD_DIR}/external/chromium/prebuilt/arm64/"
  aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/TrichromeChrome.apk" "${BUILD_DIR}/external/chromium/prebuilt/arm64/"

  cat <<EOF > "${BUILD_DIR}/frameworks/base/core/res/res/xml/config_webview_packages.xml"
<?xml version="1.0" encoding="utf-8"?>
<webviewproviders>
    <webviewprovider description="Chromium WebView" packageName="org.chromium.webview" availableByDefault="true">
    </webviewprovider>
</webviewproviders>
EOF
}

build_fdroid() {
  log_header "${FUNCNAME[0]}"

  pushd "${HOME}"
  # install gradle
  gradle_version="6.6.1"
  if [ ! -f "${HOME}/gradle/gradle-${gradle_version}/bin/gradle" ]; then
    retry wget -q "https://downloads.gradle-dn.com/distributions/gradle-${gradle_version}-bin.zip" -O "gradle-${gradle_version}-bin.zip"
    mkdir -p "${HOME}/gradle"
    unzip -d "${HOME}/gradle" "gradle-${gradle_version}-bin.zip"
  fi
  export PATH="${PATH}:${HOME}/gradle/gradle-${gradle_version}/bin"
  popd

  # setup android sdk root/paths, commandline tools and install build-tools
  export ANDROID_SDK_ROOT="${HOME}/sdk"
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
  rm -rf "${HOME}/fdroidclient"
  git clone https://gitlab.com/fdroid/fdroidclient "${HOME}/fdroidclient"
  pushd "${HOME}/fdroidclient"
  git checkout "${FDROID_CLIENT_VERSION}"
  retry gradle assembleRelease

  # copy to AOSP build tree
  cp -f "app/build/outputs/apk/full/release/app-full-release-unsigned.apk" "${BUILD_DIR}/packages/apps/F-Droid/F-Droid.apk"
  popd
}

get_encryption_key() {
  additional_message=""
  if [ "$(aws s3 ls "s3://${AWS_ENCRYPTED_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
    additional_message="Since you have no encrypted signing keys in s3://${AWS_ENCRYPTED_KEYS_BUCKET}/${DEVICE} yet - new signing keys will be generated and encrypted with provided passphrase."
  fi

  wait_time="10m"
  error_message=""
  while true; do
    aws sns publish --region ${REGION} --topic-arn "${AWS_SNS_ARN}" \
      --message="$(printf "%s Need to login to the EC2 instance and provide the encryption passphrase (${wait_time} timeout before shutdown). You may need to open up SSH in the default security group, see the FAQ for details. %s\n\nssh ubuntu@%s 'printf \"Enter encryption passphrase: \" && read -s k && echo \"\$k\" > %s'" "${error_message}" "${additional_message}" "${INSTANCE_IP}" "${ENCRYPTION_PIPE}")"
    error_message=""

    log "Waiting for encryption passphrase (with ${wait_time} timeout) to be provided over named pipe ${ENCRYPTION_PIPE}"
    set +e
    ENCRYPTION_KEY=$(timeout ${wait_time} cat ${ENCRYPTION_PIPE})
    if [ $? -ne 0 ]; then
      set -e
      log "Timeout (${wait_time}) waiting for encryption passphrase"
      aws_notify_simple "Timeout (${wait_time}) waiting for encryption passphrase. Terminating build process."
      exit 1
    fi
    set -e
    if [ -z "${ENCRYPTION_KEY}" ]; then
      error_message="ERROR: Empty encryption passphrase received - try again."
      log "${error_message}"
      continue
    fi
    log "Received encryption passphrase over named pipe ${ENCRYPTION_PIPE}"

    if [ "$(aws s3 ls "s3://${AWS_ENCRYPTED_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
      log "No existing encrypting keys - new keys will be generated later in build process."
    else
      log "Verifying encryption passphrase is valid by syncing encrypted signing keys from S3 and decrypting"
      aws s3 sync "s3://${AWS_ENCRYPTED_KEYS_BUCKET}" "${KEYS_DIR}"

      decryption_error=false
      set +e
      for f in $(find "${KEYS_DIR}" -type f -name '*.gpg'); do
        output_file=$(echo "${f}" | awk -F".gpg" '{print $1}')
        log "Decrypting ${f} to ${output_file}..."
        gpg -d --batch --passphrase "${ENCRYPTION_KEY}" "${f}" > "${output_file}"
        if [ $? -ne 0 ]; then
          log "Failed to decrypt ${f}"
          decryption_error=true
        fi
      done
      set -e
      if [ "${decryption_error}" = true ]; then
        log
        error_message="ERROR: Failed to decrypt signing keys with provided passphrase - try again."
        log "${error_message}"
        continue
      fi
    fi
    break
  done
}

initial_key_setup() {
  # setup in memory file system to hold keys
  log "Mounting in memory filesystem at ${KEYS_DIR} to hold keys"
  mkdir -p "${KEYS_DIR}"
  sudo mount -t tmpfs -o size=20m tmpfs "${KEYS_DIR}" || true

  # additional steps for getting encryption key up front
  if [ "${ENCRYPTED_KEYS}" = true ]; then
    log "Encrypted keys option was specified"

    # send warning if user has selected encrypted keys option but still has normal keys
    if [ "$(aws s3 ls "s3://${AWS_KEYS_BUCKET}/${DEVICE}" | wc -l)" != '0' ]; then
      if [ "$(aws s3 ls "s3://${AWS_ENCRYPTED_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
        aws_notify_simple "It looks like you have selected --encrypted-keys option and have existing signing keys in s3://${AWS_KEYS_BUCKET}/${DEVICE} but you haven't migrated your keys to s3://${AWS_ENCRYPTED_KEYS_BUCKET}/${DEVICE}. This means new encrypted signing keys will be generated and you'll need to flash a new factory image on your device. If you want to keep your existing keys - cancel this build and follow the steps on migrating your keys in the FAQ."
      fi
    fi

    sudo DEBIAN_FRONTEND=noninteractive apt-get -y install gpg
    if [ ! -e "${ENCRYPTION_PIPE}" ]; then
      mkfifo "${ENCRYPTION_PIPE}"
    fi

    get_encryption_key
  fi
}

setup_env() {
  log_header "${FUNCNAME[0]}"

  # setup build dir
  mkdir -p "${BUILD_DIR}"

  # install required packages
  sudo apt-get update
  sudo DEBIAN_FRONTEND=noninteractive apt-get -y install python3 repo gperf jq default-jdk git-core gnupg \
      flex bison build-essential zip curl zlib1g-dev gcc-multilib g++-multilib libc6-dev-i386 lib32ncurses5-dev \
      x11proto-core-dev libx11-dev lib32z-dev ccache libgl1-mesa-dev libxml2-utils xsltproc unzip python-networkx liblz4-tool pxz
  sudo DEBIAN_FRONTEND=noninteractive apt-get -y build-dep "linux-image-$(uname --kernel-release)"

  retry curl --fail -s https://storage.googleapis.com/git-repo-downloads/repo > /tmp/repo
  chmod +x /tmp/repo
  sudo mv /tmp/repo /usr/local/bin/

  # setup git
  git config --get --global user.name || git config --global user.name 'aosp'
  git config --get --global user.email || git config --global user.email 'aosp@localhost'
  git config --global color.ui true
}

check_chromium() {
  log_header "${FUNCNAME[0]}"

  current=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/chromium/revision" - || true)
  log "Chromium current: ${current}"

  log "Chromium latest: ${LATEST_CHROMIUM}"
  if [ "${LATEST_CHROMIUM}" == "${current}" ]; then
    log "Chromium latest (${LATEST_CHROMIUM}) matches current (${current})"
  else
    log "Building chromium ${LATEST_CHROMIUM}"
    build_chromium "${LATEST_CHROMIUM}"
  fi

  log "Deleting chromium directory ${HOME}/chromium"
  rm -rf "${HOME}/chromium"
}

build_chromium() {
  log_header "${FUNCNAME[0]}"

  CHROMIUM_REVISION="$1"
  DEFAULT_VERSION=$(echo "${CHROMIUM_REVISION}" | awk -F"." '{ printf "%s%03d52\n",$3,$4}')

  # depot tools setup
  if [ ! -d "${HOME}/depot_tools" ]; then
    retry git clone https://chromium.googlesource.com/chromium/tools/depot_tools.git "${HOME}/depot_tools"
  fi
  export PATH="${PATH}:${HOME}/depot_tools"

  # fetch chromium
  mkdir -p "${HOME}/chromium"
  cd "${HOME}/chromium"
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
  BUNDLETOOL="${HOME}/chromium/src/build/android/gyp/bundletool.py"
  AAPT2="${HOME}/chromium/src/third_party/android_build_tools/aapt2/aapt2"
  find "${HOME}/chromium/src" | grep 'apksigner' || true
  APKSIGNER="${HOME}/chromium/src/third_party/android_sdk/public/build-tools/30.0.1/apksigner"
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

  log "Uploading trichrome apks to s3"
  retry aws s3 cp "TrichromeLibrary.apk" "s3://${AWS_RELEASE_BUCKET}/chromium/TrichromeLibrary.apk"
  retry aws s3 cp "TrichromeWebView.apk" "s3://${AWS_RELEASE_BUCKET}/chromium/TrichromeWebView.apk"
  retry aws s3 cp "TrichromeChrome.apk" "s3://${AWS_RELEASE_BUCKET}/chromium/TrichromeChrome.apk"
  echo "${CHROMIUM_REVISION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/chromium/revision"
}

aosp_repo_init() {
  log_header "${FUNCNAME[0]}"
  cd "${BUILD_DIR}"

  retry repo init --manifest-url "${MANIFEST_URL}" --manifest-branch "${AOSP_BRANCH}" --depth 1 || true
}

aosp_repo_modifications() {
  log_header "${FUNCNAME[0]}"
  cd "${BUILD_DIR}"

  mkdir -p "${BUILD_DIR}/.repo/local_manifests"

  cat <<EOF > "${BUILD_DIR}/.repo/local_manifests/rattlesnakeos.xml"
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

  <% if .CustomManifestRemotes %>
  <% range $i, $r := .CustomManifestRemotes %>
  <remote name="<% .Name %>" fetch="<% .Fetch %>" revision="<% .Revision %>" />
  <% end %>
  <% end %>
  <% if .CustomManifestProjects %><% range $i, $r := .CustomManifestProjects %>
  <project path="<% .Path %>" name="<% .Name %>" remote="<% .Remote %>" />
  <% end %>
  <% end %>

</manifest>
EOF

}

aosp_repo_sync() {
  log_header "${FUNCNAME[0]}"
  cd "${BUILD_DIR}"

  # sync with retries
  for i in {1..10}; do
    log "aosp repo sync attempt ${i}/10"
    repo sync -c --no-tags --no-clone-bundle --jobs 32 && break
  done
}

setup_vendor() {
  log_header "${FUNCNAME[0]}"

  # new dependency to extract ota partitions
  sudo DEBIAN_FRONTEND=noninteractive apt-get -y install python-protobuf

  # get vendor files (with timeout)
  timeout 30m "${BUILD_DIR}/vendor/android-prepare-vendor/execute-all.sh" --debugfs --yes --device "${DEVICE}" \
      --buildID "${AOSP_VENDOR_BUILD}" --output "${BUILD_DIR}/vendor/android-prepare-vendor"
  
  # copy vendor files to build tree
  mkdir --parents "${BUILD_DIR}/vendor/google_devices" || true
  rm -rf "${BUILD_DIR}/vendor/google_devices/${DEVICE}" || true
  mv "${BUILD_DIR}/vendor/android-prepare-vendor/${DEVICE}/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_VENDOR_BUILD}")/vendor/google_devices/${DEVICE}" "${BUILD_DIR}/vendor/google_devices"

  # smaller devices need big brother vendor files
  if [ "${DEVICE}" != "${DEVICE_FAMILY}" ]; then
    rm -rf "${BUILD_DIR}/vendor/google_devices/${DEVICE_FAMILY}" || true
    mv "${BUILD_DIR}/vendor/android-prepare-vendor/${DEVICE}/$(tr '[:upper:]' '[:lower:]' <<< "${AOSP_VENDOR_BUILD}")/vendor/google_devices/${DEVICE_FAMILY}" "${BUILD_DIR}/vendor/google_devices"
  fi

  # workaround for libsdsprpc and libadsprpc not specifying LOCAL_SHARED_LIBRARIES
  sed -i '/LOCAL_MODULE := libsdsprpc/a LOCAL_SHARED_LIBRARIES := libc++ libc libcutils libdl libion liblog libm' "${BUILD_DIR}/vendor/google_devices/${DEVICE}/Android.mk" || true
  sed -i '/LOCAL_MODULE := libadsprpc/a LOCAL_SHARED_LIBRARIES := libc++ libc libcutils libdl libion liblog libm' "${BUILD_DIR}/vendor/google_devices/${DEVICE}/Android.mk" || true
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
  sed -i '1 i\BUILD_BROKEN_VINTF_PRODUCT_COPY_FILES := true' "${BUILD_DIR}/build/make/target/board/BoardConfigMainlineCommon.mk"

  # workaround for coral/flame build issue
  sed -i 's@PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@#PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@' "${BUILD_DIR}/device/google/coral/aosp_coral.mk" || true
  sed -i 's@PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@#PRODUCT_ENFORCE_ARTIFACT_PATH_REQUIREMENTS := strict@' "${BUILD_DIR}/device/google/coral/aosp_flame.mk" || true

  # biometrics was disabled (https://cs.android.com/android/_/android/platform/frameworks/base/+/ede919cace2a32ec235eefe86e04a75848bd1d5f)
  # but never added upstream to device specific overlays

  # ID0:Fingerprint:Strong
  biometric_sensors="0:2:15"
  if [ "${DEVICE_COMMON}" == "coral" ]; then
    # ID0:Face:Strong
    biometric_sensors="0:8:15"
  fi
  sed -i '$ s/^<\/resources>//' "${BUILD_DIR}/device/google/${DEVICE_COMMON}/overlay/frameworks/base/core/res/res/values/config.xml"
  cat <<EOF >> "${BUILD_DIR}/device/google/${DEVICE_COMMON}/overlay/frameworks/base/core/res/res/values/config.xml"
    <string-array name="config_biometric_sensors" translatable="false" >
        <item>${biometric_sensors}</item>
    </string-array>
</resources>
EOF

}

patch_launcher() {
  log_header "${FUNCNAME[0]}"

  # disable QuickSearchBox widget on home screen
  sed -i "s/QSB_ON_FIRST_SCREEN = true;/QSB_ON_FIRST_SCREEN = false;/" "${BUILD_DIR}/packages/apps/Launcher3/src/com/android/launcher3/config/FeatureFlags.java"
}

# currently don't have a need for apex updates (https://source.android.com/devices/tech/ota/apex)
patch_disable_apex() {
  log_header "${FUNCNAME[0]}"

  # pixel 2 devices opt in here
  sed -i 's@$(call inherit-product, $(SRC_TARGET_DIR)/product/updatable_apex.mk)@@' "${BUILD_DIR}/device/google/wahoo/device.mk" || true
  # all other devices use mainline and opt in here
  sed -i 's@$(call inherit-product, $(SRC_TARGET_DIR)/product/updatable_apex.mk)@@' "${BUILD_DIR}/build/make/target/product/mainline_system.mk"
}

# TODO: most of this is fragile and unforgiving
patch_custom() {
  log_header "${FUNCNAME[0]}"

  cd "${BUILD_DIR}"

  # allow custom patches to be applied
  patches_dir="${HOME}/patches"
  <% if .CustomPatches %>
  <% range $i, $r := .CustomPatches %>
    retry git clone <% if $r.Branch %>--branch <% $r.Branch %><% end %> <% $r.Repo %> ${patches_dir}/<% $i %>
    <% range $r.Patches %>
      log "Applying patch <% . %>"
      patch -p1 --no-backup-if-mismatch < ${patches_dir}/<% $i %>/<% . %>
    <% end %>
  <% end %>
  <% end %>

  # allow custom scripts to be applied
  scripts_dir="${HOME}/scripts"
  <% if .CustomScripts %>
  <% range $i, $r := .CustomScripts %>
    retry git clone <% if $r.Branch %>--branch <% $r.Branch %><% end %> <% $r.Repo %> ${scripts_dir}/<% $i %>
    <% range $r.Scripts %>
      log "Applying shell script <% . %>"
      . ${scripts_dir}/<% $i %>/<% . %>
    <% end %>
  <% end %>
  <% end %>

  # allow prebuilt applications to be added to build tree
  prebuilt_dir="${BUILD_DIR}/packages/apps/Custom"
  # TODO: should be able to specify where to add PRODUCT_PACKAGES
  mk_file="${BUILD_DIR}/build/make/target/product/handheld_system.mk"
  <% if .CustomPrebuilts %>
  <% range $i, $r := .CustomPrebuilts %>
    log "Putting custom prebuilts from <% $r.Repo %> in build tree location ${prebuilt_dir}/<% $i %>"
    retry git clone <% $r.Repo %> ${prebuilt_dir}/<% $i %>
    <% range .Modules %>
      log "Adding custom PRODUCT_PACKAGES += <% . %> to ${mk_file}"
      sed -i "\$aPRODUCT_PACKAGES += <% . %>" "${mk_file}"
    <% end %>
  <% end %>
  <% end %>

  # allow custom hosts file
  hosts_file_location="${BUILD_DIR}/system/core/rootdir/etc/hosts"
  if [ -z "${HOSTS_FILE}" ]; then
    log "No custom hosts file requested"
  else
    log "Replacing hosts file with ${HOSTS_FILE}"
    retry wget -q -O "${hosts_file_location}" "${HOSTS_FILE}"
  fi
}

patch_base_config() {
  log_header "${FUNCNAME[0]}"

  # enable doze and app standby
  sed -i 's@<bool name="config_enableAutoPowerModes">false</bool>@<bool name="config_enableAutoPowerModes">true</bool>@' "${BUILD_DIR}/frameworks/base/core/res/res/values/config.xml"
}

patch_device_config() {
  log_header "${FUNCNAME[0]}"

  # set proper model names
  sed -i 's@PRODUCT_MODEL := AOSP on taimen@PRODUCT_MODEL := Pixel 2 XL@' "${BUILD_DIR}/device/google/taimen/aosp_taimen.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on walleye@PRODUCT_MODEL := Pixel 2@' "${BUILD_DIR}/device/google/muskie/aosp_walleye.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on crosshatch@PRODUCT_MODEL := Pixel 3 XL@' "${BUILD_DIR}/device/google/crosshatch/aosp_crosshatch.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on blueline@PRODUCT_MODEL := Pixel 3@' "${BUILD_DIR}/device/google/crosshatch/aosp_blueline.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on bonito@PRODUCT_MODEL := Pixel 3a XL@' "${BUILD_DIR}/device/google/bonito/aosp_bonito.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on sargo@PRODUCT_MODEL := Pixel 3a@' "${BUILD_DIR}/device/google/bonito/aosp_sargo.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on coral@PRODUCT_MODEL := Pixel 4 XL@' "${BUILD_DIR}/device/google/coral/aosp_coral.mk" || true
  sed -i 's@PRODUCT_MODEL := AOSP on flame@PRODUCT_MODEL := Pixel 4@' "${BUILD_DIR}/device/google/coral/aosp_flame.mk" || true

  sed -i 's@PRODUCT_MODEL := AOSP on sunfish@PRODUCT_MODEL := Pixel 4A@' "${BUILD_DIR}/device/google/sunfish/aosp_sunfish.mk" || true
}

patch_add_apps() {
  log_header "${FUNCNAME[0]}"

  handheld_system_mk="${BUILD_DIR}/build/make/target/product/handheld_system.mk"
  sed -i "\$aPRODUCT_PACKAGES += Updater" "${handheld_system_mk}"
  sed -i "\$aPRODUCT_PACKAGES += F-DroidPrivilegedExtension" "${handheld_system_mk}"
  sed -i "\$aPRODUCT_PACKAGES += F-Droid" "${handheld_system_mk}"

  handheld_product_mk="${BUILD_DIR}/build/make/target/product/handheld_product.mk"
  sed -i 's/Browser2 \\/TrichromeChrome \\/' "${handheld_product_mk}"

  media_product_mk="${BUILD_DIR}/build/make/target/product/media_product.mk"
  sed -i 's/webview \\/TrichromeWebView \\/' "${media_product_mk}"

  # add any modules defined in custom manifest projects
  <% if .CustomManifestProjects %><% range $i, $r := .CustomManifestProjects %><% range $j, $q := .Modules %>
  log "Adding custom PRODUCT_PACKAGES += <% $q %> to ${handheld_system_mk}"
  sed -i "\$aPRODUCT_PACKAGES += <% $q %>" "${handheld_system_mk}"
  <% end %>
  <% end %>
  <% end %>
}

patch_updater() {
  log_header "${FUNCNAME[0]}"

  cd "${BUILD_DIR}/packages/apps/Updater/res/values"
  sed --in-place --expression "s@s3bucket@${RELEASE_URL}/@g" config.xml

  # TODO: just a hack to get 11 up and running
  # related commit: https://android.googlesource.com/platform/system/sepolicy/+/d61b0ce1bc8de2560f1fa173c8d01a09d039a12a%5E%21/#F0
  cat << 'EOF' > "${HOME}/updater-selinux.patch"
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
  pushd "${BUILD_DIR}/system/sepolicy"
  git apply "${HOME}/updater-selinux.patch"
  popd
}

fdpe_hash() {
  keytool -printcert -file "$1" | grep 'SHA256:' | tr --delete ':' | cut --delimiter ' ' --fields 3
}

patch_priv_ext() {
  log_header "${FUNCNAME[0]}"

  # 0.2.9 added whitelabel support, so BuildConfig.APPLICATION_ID needs to be set now
  sed -i 's@BuildConfig.APPLICATION_ID@"org.fdroid.fdroid.privileged"@' "${BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/PrivilegedService.java"

  unofficial_releasekey_hash=$(fdpe_hash "${KEYS_DIR}/${DEVICE}/releasekey.x509.pem")
  unofficial_platform_hash=$(fdpe_hash "${KEYS_DIR}/${DEVICE}/platform.x509.pem")
  sed -i 's/'${OFFICIAL_FDROID_KEY}'")/'${unofficial_releasekey_hash}'"),\n            new Pair<>("org.fdroid.fdroid", "'${unofficial_platform_hash}'")/' \
      "${BUILD_DIR}/packages/apps/F-DroidPrivilegedExtension/app/src/main/java/org/fdroid/fdroid/privileged/ClientWhitelist.java"
}

build_aosp() {
  log_header "${FUNCNAME[0]}"

  cd "${BUILD_DIR}"

  if [ "${AOSP_BUILD}" != "${AOSP_VENDOR_BUILD}" ]; then
    log "WARNING: Requested AOSP build does not match upstream vendor files. These images may not be functional."
    log "Patching build_id to match ${AOSP_BUILD}"
    echo "${AOSP_BUILD}" > "vendor/google_devices/${DEVICE}/build_id.txt"
  fi

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

  log "Running choosecombo ${BUILD_TARGET}"
  choosecombo ${BUILD_TARGET}

  log "Running target-files-package"
  retry make -j "$(nproc)" target-files-package

  log "Running brillo_update_payload"
  retry make -j "$(nproc)" brillo_update_payload

  log "Running m otatools-package"
  m otatools-package
  rm -rf "${HOME}/release"
  mkdir -p "${HOME}/release"
  unzip "${BUILD_DIR}/out/target/product/${DEVICE}/otatools.zip" -d "${HOME}/release"
}

get_radio_image() {
  grep -Po "require version-$1=\K.+" "vendor/$2/vendor-board-info.txt" | tr '[:upper:]' '[:lower:]'
}

release() {
  log_header "${FUNCNAME[0]}"

  cd "${BUILD_DIR}"

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
  KEY_DIR="keys/${DEVICE}"
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

  export PATH="${HOME}/release/bin:${PATH}"
  export PATH="${BUILD_DIR}/prebuilts/jdk/jdk9/linux-x86/bin:${PATH}"

  log "Running sign_target_files_apks"
  "${HOME}/release/releasetools/sign_target_files_apks" \
	-o -d "${KEY_DIR}" \
	-k "${BUILD_DIR}/build/target/product/security/networkstack=${KEY_DIR}/networkstack" "${AVB_SWITCHES[@]}" \
	"${BUILD_DIR}/out/target/product/${DEVICE}/obj/PACKAGING/target_files_intermediates/${PREFIX}${DEVICE}-target_files-${BUILD_NUMBER}.zip" \
	"${OUT}/${TARGET_FILES}"

  log "Running ota_from_target_files"
  "${HOME}/release/releasetools/ota_from_target_files" --block -k "${KEY_DIR}/releasekey" "${EXTRA_OTA[@]}" "${OUT}/${TARGET_FILES}" \
      "${OUT}/${DEVICE}-ota_update-${BUILD}.zip"

  log "Running img_from_target_files"
  sed -i 's/zipfile\.ZIP_DEFLATED/zipfile\.ZIP_STORED/' "${HOME}/release/releasetools/img_from_target_files.py"
  "${HOME}/release/releasetools/img_from_target_files" "${OUT}/${TARGET_FILES}" "${OUT}/${DEVICE}-img-${BUILD}.zip"

  log "Running generate-factory-images"
  cd "${OUT}"
  sed -i 's/zip -r/tar cvf/' "../../device/common/generate-factory-images-common.sh"
  sed -i 's/factory\.zip/factory\.tar/' "../../device/common/generate-factory-images-common.sh"
  sed -i '/^mv / d' "../../device/common/generate-factory-images-common.sh"
  source "../../device/common/generate-factory-images-common.sh"
  mv "${DEVICE}-${VERSION}-factory.tar" "${DEVICE}-factory-${BUILD_NUMBER}.tar"
  rm -f "${DEVICE}-factory-${BUILD_NUMBER}.tar.xz"

  log "Running compress of factory image with pxz"
  time pxz -v -T0 -9 -z "${DEVICE}-factory-${BUILD_NUMBER}.tar"
}

# TODO: cleanup this function
aws_upload() {
  log_header "${FUNCNAME[0]}"

  cd "${BUILD_DIR}/out"
  build_date="$(< soong/build_number.txt)"
  build_timestamp="$(unzip -p "release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "META-INF/com/android/metadata" | grep 'post-timestamp' | cut --delimiter "=" --fields 2)"

  # copy ota file to s3, update file metadata used by updater app, and remove old ota files
  read -r old_metadata <<< "$(wget -O - "${RELEASE_URL}/${RELEASE_CHANNEL}")"
  old_date="$(cut -d ' ' -f 1 <<< "${old_metadata}")"
  (
    aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-ota_update-${build_date}.zip" "s3://${AWS_RELEASE_BUCKET}" --acl public-read &&
    echo "${build_date} ${build_timestamp} ${AOSP_BUILD}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}" --acl public-read &&
    echo "${BUILD_TIMESTAMP}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${RELEASE_CHANNEL}-true-timestamp" --acl public-read
  ) && ( aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-ota_update-${old_date}.zip" || true )

  # upload factory image
  retry aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-factory-${build_date}.tar.xz" "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-factory-latest.tar.xz"

  # cleanup old target files if some exist
  if [ "$(aws s3 ls "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target" | wc -l)" != '0' ]; then
    cleanup_target_files
  fi

  # copy new target file to s3
  retry aws s3 cp "${BUILD_DIR}/out/release-${DEVICE}-${build_date}/${DEVICE}-target_files-${build_date}.zip" "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target/${DEVICE}-target-files-${build_date}.zip"
}

cleanup_target_files() {
  log_header "${FUNCNAME[0]}"

  aws s3 sync "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target" "${BUILD_DIR}/${DEVICE}-target"
  cd "${BUILD_DIR}/${DEVICE}-target"
  for target_file in ${DEVICE}-target-files-*.zip ; do
    old_date=$(echo "${target_file}" | cut --delimiter "-" --fields 4 | cut --delimiter "." --fields 5 --complement)
    aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-target/${DEVICE}-target-files-${old_date}.zip" || true
  done
}

checkpoint_versions() {
  log_header "${FUNCNAME[0]}"

  # checkpoint stack version
  echo "${STACK_VERSION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/rattlesnakeos-stack/revision"

  # checkpoint f-droid
  echo "${FDROID_PRIV_EXT_VERSION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/fdroid-priv/revision"
  echo "${FDROID_CLIENT_VERSION}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/fdroid/revision"
  
  # checkpoint aosp
  aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${DEVICE}-vendor" --acl public-read <<< "${AOSP_VENDOR_BUILD}" || true
  
  # checkpoint chromium
  echo "yes" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/chromium/included"
}

aws_notify_simple() {
  log_header "${FUNCNAME[0]}"

  aws sns publish --region ${REGION} --topic-arn "${AWS_SNS_ARN}" --message "$1"
}

aws_notify() {
  log_header "${FUNCNAME[0]}"

  LOGOUTPUT=
  if [ -n "$2" ]; then
    LOGOUTPUT=$(tail -c 20000 /var/log/cloud-init-output.log)
  fi
  ELAPSED="$((SECONDS / 3600))hrs $(((SECONDS / 60) % 60))min $((SECONDS % 60))sec"
  aws sns publish --region ${REGION} --topic-arn "${AWS_SNS_ARN}" \
    --message="$(printf "$1\n  Device: %s\n  Stack Name: %s\n  Stack Version: %s %s\n  Stack Region: %s\n  Release Channel: %s\n  Instance Type: %s\n  Instance Region: %s\n  Instance IP: %s\n  Build Date: %s\n  Elapsed Time: %s\n  AOSP Build: %s\n  AOSP Vendor Build: %s\n  AOSP Branch: %s\n  Chromium Version: %s\n  F-Droid Version: %s\n  F-Droid Priv Extension Version: %s\n  Build Reason: %s\n%s" \
      "${DEVICE}" "${STACK_NAME}" "${STACK_VERSION}" "${STACK_UPDATE_MESSAGE}" "${REGION}" "${RELEASE_CHANNEL}" "${INSTANCE_TYPE}" "${INSTANCE_REGION}" "${INSTANCE_IP}" "${BUILD_DATE}" "${ELAPSED}" "${AOSP_BUILD}" "${AOSP_VENDOR_BUILD}" "${AOSP_BRANCH}" "${LATEST_CHROMIUM}" "${FDROID_CLIENT_VERSION}" "${FDROID_PRIV_EXT_VERSION}" "${BUILD_REASON}" "${LOGOUTPUT}")" || true
}

aws_logging() {
  log_header "${FUNCNAME[0]}"

  df -h
  du -chs "${BUILD_DIR}" || true
  uptime
  aws s3 cp /var/log/cloud-init-output.log "s3://${AWS_LOGS_BUCKET}/${DEVICE}/$(date +%s)"
}

aws_import_keys() {
  log_header "${FUNCNAME[0]}"

  if [ "${ENCRYPTED_KEYS}" = true ]; then
    if [ "$(aws s3 ls "s3://${AWS_ENCRYPTED_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
      log "No encrypted keys were found - generating encrypted keys"
      gen_keys
      for f in $(find "${KEYS_DIR}" -type f); do
        log "Encrypting file ${f} to ${f}.gpg"
        gpg --symmetric --batch --passphrase "${ENCRYPTION_KEY}" --cipher-algo AES256 "${f}"
      done
      log "Syncing encrypted keys to S3 s3://${AWS_ENCRYPTED_KEYS_BUCKET}"
      aws s3 sync "${KEYS_DIR}" "s3://${AWS_ENCRYPTED_KEYS_BUCKET}" --exclude "*" --include "*.gpg"
    fi
  else
    if [ "$(aws s3 ls "s3://${AWS_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
      log "No keys were found - generating keys"
      gen_keys
      log "Syncing keys to S3 s3://${AWS_KEYS_BUCKET}"
      aws s3 sync "${KEYS_DIR}" "s3://${AWS_KEYS_BUCKET}"
    else
      log "Keys already exist for ${DEVICE} - syncing them from S3"
      aws s3 sync "s3://${AWS_KEYS_BUCKET}" "${KEYS_DIR}"
    fi
  fi

  # handle migration with chromium.keystore
  pushd "${KEYS_DIR}/${DEVICE}"
  if [ ! -f "${KEYS_DIR}/${DEVICE}/chromium.keystore" ]; then
    log "Did not find chromium.keystore - generating"
	keytool -genkey -v -keystore chromium.keystore -storetype pkcs12 -alias chromium -keyalg RSA -keysize 4096 \
        -sigalg SHA512withRSA -validity 10000 -dname "cn=RattlesnakeOS" -storepass chromium
    if [ "${ENCRYPTED_KEYS}" = true ]; then
      log "Encrypting and uploading new chromium.keystore to s3://${AWS_ENCRYPTED_KEYS_BUCKET}"
      gpg --symmetric --batch --passphrase "${ENCRYPTION_KEY}" --cipher-algo AES256 chromium.keystore
      aws s3 sync "${KEYS_DIR}" "s3://${AWS_ENCRYPTED_KEYS_BUCKET}" --exclude "*" --include "*.gpg"
    else
      log "Uploading new chromium.keystore to s3://${AWS_KEYS_BUCKET}"
      aws s3 sync "${KEYS_DIR}" "s3://${AWS_KEYS_BUCKET}"
    fi
  fi
  popd
}

gen_keys() {
  log_header "${FUNCNAME[0]}"

  # download make_key and avbtool as aosp tree isn't downloaded yet
  make_key="${HOME}/make_key"
  retry curl --fail -s "https://android.googlesource.com/platform/development/+/refs/tags/${AOSP_BRANCH}/tools/make_key?format=TEXT" | base64 --decode > "${make_key}"
  chmod +x "${make_key}"
  avb_tool="${HOME}/avbtool"
  retry curl --fail -s "https://android.googlesource.com/platform/external/avb/+/refs/tags/${AOSP_BRANCH}/avbtool?format=TEXT" | base64 --decode > "${avb_tool}"
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
  aws_logging
  if [ $rv -ne 0 ]; then
    aws_notify "RattlesnakeOS Build FAILED" 1
  fi
  if [ "${PREVENT_SHUTDOWN}" = true ]; then
    log "Skipping shutdown"
  else
    sudo shutdown -h now
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

trap cleanup 0

set -e

full_run
`
