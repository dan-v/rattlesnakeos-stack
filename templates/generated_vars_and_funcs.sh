########################################
######## STACK CONFIG VARS #############
########################################
DEVICE="<% .Device %>"
DEVICE_FRIENDLY="<% .DeviceDetails.Friendly %>"
DEVICE_FAMILY="<% .DeviceDetails.Family %>"
DEVICE_AVB_MODE="<% .DeviceDetails.AVBMode %>"
DEVICE_EXTRA_OTA=<% .DeviceDetails.ExtraOTA %>
STACK_NAME="<% .Name %>"
STACK_VERSION="<% .Version %>"
CHROMIUM_BUILD_DISABLED="<% .ChromiumBuildDisabled %>"
CORE_CONFIG_REPO="<% .CoreConfigRepo %>"
CUSTOM_CONFIG_REPO="<% .CustomConfigRepo %>"
LATEST_JSON_URL="<% .Config.LatestURL %>"

##########################################
###### CLOUD SPECIFIC VARS AND FUNCS #####
##########################################
<% if eq .Cloud "aws" -%>
REGION="<% .Region %>"
AWS_KEYS_BUCKET="${STACK_NAME}-keys"
AWS_RELEASE_BUCKET="${STACK_NAME}-release"
RELEASE_URL="https://${AWS_RELEASE_BUCKET}.s3.amazonaws.com"
<%- end %>

import_keys() {
  log_header "${FUNCNAME[0]}"

  <% if eq .Cloud "aws" -%>
  if [ "$(aws s3 ls "s3://${AWS_KEYS_BUCKET}/${DEVICE}" | wc -l)" == '0' ]; then
    log "No keys were found - generating keys"
    gen_keys
    log "Syncing keys to S3 s3://${AWS_KEYS_BUCKET}"
    aws s3 sync "${KEYS_DIR}" "s3://${AWS_KEYS_BUCKET}"
  else
    log "Keys already exist for ${DEVICE} - syncing them from S3"
    aws s3 sync "s3://${AWS_KEYS_BUCKET}" "${KEYS_DIR}"
  fi
  <%- else %>
  echo "todo"
  <%- end %>
}

notify() {
  log_header "${FUNCNAME[0]}"

  <% if eq .Cloud "aws" -%>
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
    --message="$(printf "$1\n  Device: %s\n  Stack Name: %s\n  Stack Version: %s\n  Stack Region: %s\n  Release Channel: %s\n  Instance Type: %s\n  Instance Region: %s\n  Instance IP: %s\n  Elapsed Time: %s\n  AOSP Build ID: %s\n  AOSP Tag: %s\n  %s" \
      "${DEVICE}" "${STACK_NAME}" "${STACK_VERSION}" "${REGION}" "${RELEASE_CHANNEL}" "${INSTANCE_TYPE}" "${INSTANCE_REGION}" "${INSTANCE_IP}" "${ELAPSED}" "${AOSP_BUILD_ID}" "${AOSP_TAG}" "${LOGOUTPUT}")" || true
  <%- else %>
  echo "todo"
  <%- end %>
}

cleanup() {
  <% if eq .Cloud "aws" -%>
  rv=$?
  df -h
  du -chs "${AOSP_BUILD_DIR}" || true
  uptime
  AWS_LOGS_BUCKET="${STACK_NAME}-logs"
  aws s3 cp /var/log/cloud-init-output.log "s3://${AWS_LOGS_BUCKET}/${DEVICE}/$(date +%s)" || true
  if [ $rv -ne 0 ]; then
    notify "RattlesnakeOS Build FAILED" 1
  fi
  sudo shutdown -h now
  <%- else %>
  echo "todo"
  <%- end %>
}

get_current_metadata() {
  <% if eq .Cloud "aws" -%>
    local metadata_location="${1}"
    local current=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${1}" - 2>/dev/null || true)
    echo "${current}"
  <%- else %>
  echo "todo"
  <%- end %>
}

set_current_metadata() {
  local metadata_location="${1}"
  local metadata_value="${2}"
  local public="${3}"
  <% if eq .Cloud "aws" -%>
  if [ -z "${public}" ]; then
    echo "${metadata_value}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${metadata_location}"
  else
    echo "${metadata_value}" | aws s3 cp - "s3://${AWS_RELEASE_BUCKET}/${metadata_location}" --acl public-read
  fi
  <%- else %>
  echo "todo"
  <%- end %>
}

upload_build_artifact() {
  local src_file="${1}"
  local dest_file="${2}"
  local public="${3}"
  <% if eq .Cloud "aws" -%>
  if [ -z "${public}" ]; then
    retry aws s3 cp "${src_file}" "s3://${AWS_RELEASE_BUCKET}/${dest_file}"
  else
    retry aws s3 cp "${src_file}" "s3://${AWS_RELEASE_BUCKET}/${dest_file}" --acl public-read
  fi
  <%- else %>
  echo "todo"
  <%- end %>
}

download_build_artifact() {
  local src_file="${1}"
  local dest_file="${2}"
  <% if eq .Cloud "aws" -%>
  retry aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${src_file}" "${dest_file}"
  <%- else %>
  echo "todo"
  <%- end %>
}

delete_build_artifact() {
  local dest_file="${1}"
  <% if eq .Cloud "aws" -%>
  aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${dest_file}" || true
  <%- else %>
  echo "todo"
  <%- end %>
}