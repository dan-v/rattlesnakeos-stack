ANDROID_VERSION="11.0"
DEVICE="<% .Device %>"
DEVICE_FRIENDLY="<% .DeviceDetails.Friendly %>"
DEVICE_FAMILY="<% .DeviceDetails.Family %>"
DEVICE_COMMON="<% .DeviceDetails.Common %>"
AVB_MODE="<% .DeviceDetails.AVBMode %>"
EXTRA_OTA=<% .DeviceDetails.ExtraOTA %>
REGION="<% .Region %>"
STACK_NAME="<% .Name %>"
STACK_VERSION="<% .Version %>"
BUILD_TYPE="user"
BUILD_CHANNEL="stable"
HOSTS_FILE="<% .HostsFile %>"
AWS_KEYS_BUCKET="${STACK_NAME}-keys"
AWS_RELEASE_BUCKET="${STACK_NAME}-release"
MANIFEST_URL="https://android.googlesource.com/platform/manifest"
CORE_URL="https://github.com/rattlesnakeos/core.git"
CUSTOM_URL="https://github.com/dan-v/custom.git"
BUILD_TARGET="release aosp_${DEVICE} ${BUILD_TYPE}"
RELEASE_URL="https://${AWS_RELEASE_BUCKET}.s3.amazonaws.com"
RELEASE_CHANNEL="${DEVICE}-${BUILD_CHANNEL}"
ROOT_DIR="${HOME}"
AOSP_BUILD_DIR="${ROOT_DIR}/aosp"
CHROMIUM_BUILD_DIR="${ROOT_DIR}/chromium"
CORE_DIR="${ROOT_DIR}/core"
CUSTOM_DIR="${ROOT_DIR}/custom"
KEYS_DIR="${ROOT_DIR}/keys"
MISC_DIR="${ROOT_DIR}/misc"
RELEASE_TOOLS_DIR="${MISC_DIR}/releasetools"

notify() {
  log_header "${FUNCNAME[0]}"

  LOGOUTPUT=
  if [ -n "$2" ]; then
    LOGOUTPUT=$(tail -c 20000 /var/log/cloud-init-output.log)
  fi

  AWS_SNS_ARN=$(aws --region ${REGION} sns list-topics --query 'Topics[0].TopicArn' --output text | cut -d":" -f1,2,3,4,5)":${STACK_NAME}"
  INSTANCE_TYPE=$(curl -s http://169.254.169.254/latest/meta-data/instance-type)
  INSTANCE_REGION=$(curl -s http://169.254.169.254/latest/dynamic/instance-identity/document | awk -F\" '/region/ {print $4}')
  INSTANCE_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)
  ELAPSED="$((SECONDS / 3600))hrs $(((SECONDS / 60) % 60))min $((SECONDS % 60))sec"
  aws sns publish --region ${REGION} --topic-arn "${AWS_SNS_ARN}" \
    --message="$(printf "$1\n  Device: %s\n  Stack Name: %s\n  Stack Version: %s\n  Stack Region: %s\n  Release Channel: %s\n  Instance Type: %s\n  Instance Region: %s\n  Instance IP: %s\n  Elapsed Time: %s\n  AOSP Build ID: %s\n  AOSP Tag: %s\n  Chromium Version: %s\n %s" \
      "${DEVICE}" "${STACK_NAME}" "${STACK_VERSION}" "${REGION}" "${RELEASE_CHANNEL}" "${INSTANCE_TYPE}" "${INSTANCE_REGION}" "${INSTANCE_IP}" "${ELAPSED}" "${AOSP_BUILD_ID}" "${AOSP_TAG}" "${CHROMIUM_VERSION}" "${LOGOUTPUT}")" || true
}

logging() {
  log_header "${FUNCNAME[0]}"

  df -h
  du -chs "${AOSP_BUILD_DIR}" || true
  uptime
  AWS_LOGS_BUCKET="${STACK_NAME}-logs"
  aws s3 cp /var/log/cloud-init-output.log "s3://${AWS_LOGS_BUCKET}/${DEVICE}/$(date +%s)" || true
}

import_keys() {
  log_header "${FUNCNAME[0]}"

  if [ "$(aws s3 ls "s3://${AWS_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
    log "No keys were found - generating keys"
    gen_keys
    log "Syncing keys to S3 s3://${AWS_KEYS_BUCKET}"
    aws s3 sync "${KEYS_DIR}" "s3://${AWS_KEYS_BUCKET}"
  else
    log "Keys already exist for ${DEVICE} - syncing them from S3"
    aws s3 sync "s3://${AWS_KEYS_BUCKET}" "${KEYS_DIR}"
  fi

  # handle migration with chromium.keystore
  pushd "${KEYS_DIR}/${DEVICE}"
  if [ ! -f "${KEYS_DIR}/${DEVICE}/chromium.keystore" ]; then
    log "Did not find chromium.keystore - generating"
	  keytool -genkey -v -keystore chromium.keystore -storetype pkcs12 -alias chromium -keyalg RSA -keysize 4096 \
        -sigalg SHA512withRSA -validity 10000 -dname "cn=RattlesnakeOS" -storepass chromium
    log "Uploading new chromium.keystore to s3://${AWS_KEYS_BUCKET}"
    aws s3 sync "${KEYS_DIR}" "s3://${AWS_KEYS_BUCKET}"
  fi
  popd
}

get_current_metadata() {
  local metadata_location="${1}"
  local current=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${1}" - 2>/dev/null || true)
  echo "${current}"
}

set_current_metadata() {
  local metadata_location="${1}"
  local metadata_value="${2}"
  local public="${3}"
  if [ -z "${public}" ]; then
    echo "${metadata_value}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${metadata_location}"
  else
    echo "${metadata_value}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${metadata_location}" --acl public-read
  fi
}

upload_build_artifact() {
  local src_file="${1}"
  local dest_file="${2}"
  local public="${3}"
  if [ -z "${public}" ]; then
    retry aws s3 cp "${src_file}" "s3://${AWS_RELEASE_BUCKET}/${dest_file}"
  else
    retry aws s3 cp "${src_file}" "s3://${AWS_RELEASE_BUCKET}/${dest_file}" --acl public-read
  fi
}

download_build_artifact() {
  local src_file="${1}"
  local dest_file="${2}"
  retry aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${src_file}" "${dest_file}"
}

delete_build_artifact() {
  local dest_file="${1}"
  aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${dest_file}" || true
}