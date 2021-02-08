ANDROID_VERSION="11.0"
DEVICE="<% .Device %>"
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

# TODO: AWS specific
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
    --message="$(printf "$1\n  Device: %s\n  Stack Name: %s\n  Stack Version: %s\n  Stack Region: %s\n  Release Channel: %s\n  Instance Type: %s\n  Instance Region: %s\n  Instance IP: %s\n  Build Date: %s\n  Elapsed Time: %s\n  AOSP Build ID: %s\n  AOSP Tag: %s\n  Chromium Version: %s\n  F-Droid Version: %s\n  F-Droid Priv Extension Version: %s\n  %s" \
      "${DEVICE}" "${STACK_NAME}" "${STACK_VERSION}" "${REGION}" "${RELEASE_CHANNEL}" "${INSTANCE_TYPE}" "${INSTANCE_REGION}" "${INSTANCE_IP}" "${BUILD_DATE}" "${ELAPSED}" "${AOSP_BUILD_ID}" "${AOSP_TAG}" "${CHROMIUM_VERSION}" "${FDROID_CLIENT_VERSION}" "${FDROID_PRIV_EXT_VERSION}" "${LOGOUTPUT}")" || true
}

# TODO: AWS specific
logging() {
  log_header "${FUNCNAME[0]}"

  df -h
  du -chs "${AOSP_BUILD_DIR}" || true
  uptime
  AWS_LOGS_BUCKET="${STACK_NAME}-logs"
  aws s3 cp /var/log/cloud-init-output.log "s3://${AWS_LOGS_BUCKET}/${DEVICE}/$(date +%s)" || true
}

# TODO: AWS specific
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

# TODO: AWS specific
get_current_metadata() {
  local metadata_location="${1}"
  local current=$(aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${1}" - 2>/dev/null || true)
  echo "${current}"
}

# TODO: AWS specific
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

# TODO: AWS specific
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

# TODO: AWS specific
download_build_artifact() {
  local src_file="${1}"
  local dest_file="${2}"
  retry aws s3 cp "s3://${AWS_RELEASE_BUCKET}/${src_file}" "${dest_file}"
}

# TODO: AWS specific
delete_build_artifact() {
  local dest_file="${1}"
  aws s3 rm "s3://${AWS_RELEASE_BUCKET}/${dest_file}" || true
}

BUILD_TARGET="release aosp_${DEVICE} ${BUILD_TYPE}"
# TODO: this needs to be configurable and not AWS specific
RELEASE_URL="https://${AWS_RELEASE_BUCKET}.s3.amazonaws.com"
RELEASE_CHANNEL="${DEVICE}-${BUILD_CHANNEL}"
BUILD_DATE=$(date +%Y.%m.%d.%H)
BUILD_TIMESTAMP=$(date +%s)
ROOT_DIR="${HOME}"
AOSP_BUILD_DIR="${ROOT_DIR}/aosp"
mkdir -p "${AOSP_BUILD_DIR}"
FDROID_BUILD_DIR="${ROOT_DIR}/fdroid"
mkdir -p "${FDROID_BUILD_DIR}"
CHROMIUM_BUILD_DIR="${ROOT_DIR}/chromium"
mkdir -p "${CHROMIUM_BUILD_DIR}"
KEYS_DIR="${ROOT_DIR}/keys"
mkdir -p "${KEYS_DIR}"
MISC_DIR="${ROOT_DIR}/misc"
mkdir -p "${MISC_DIR}"
RELEASE_TOOLS_DIR="${MISC_DIR}/releasetools"
mkdir -p "${RELEASE_TOOLS_DIR}"
CERTIFICATE_SUBJECT='/CN=RattlesnakeOS'
OFFICIAL_FDROID_KEY="43238d512c1e5eb2d6569f4a3afbf5523418b82e0a3ed1552770abb9a9c9ccab"
MANIFEST_URL="https://android.googlesource.com/platform/manifest"

<% if gt (len .CustomManifestRemotes) 0 -%>
CUSTOM_MANIFEST_REMOTES=$(cat <<-END
<% range $i, $r := .CustomManifestRemotes -%>
  <remote name="<% .Name %>" fetch="<% .Fetch %>" revision="<% .Revision %>" />
<% end -%>
END
)
<%- else %>
CUSTOM_MANIFEST_REMOTES=
<%- end %>

<% if gt (len .CustomManifestProjects) 0 -%>
CUSTOM_MANIFEST_PROJECTS=$(cat <<-END
<% range $i, $r := .CustomManifestProjects -%>
  <project path="<% .Path %>" name="<% .Name %>" remote="<% .Remote %>" />
<% end -%>
END
)
<%- else %>
CUSTOM_MANIFEST_REMOTES=
<%- end %>

patch_custom() {
  log_header "${FUNCNAME[0]}"
  cd "${AOSP_BUILD_DIR}"

  <% if gt (len .CustomPatches) 0 -%>
  patches_dir="${HOME}/patches"
  <% range $i, $r := .CustomPatches -%>
  retry git clone <% if $r.Branch %>--branch <% $r.Branch %><% end %> <% $r.Repo %> ${patches_dir}/<% $i %>
  <% range $r.Patches -%>
  log "Applying patch <% . %>"
  patch -p1 --no-backup-if-mismatch < ${patches_dir}/<% $i %>/<% . %>
  <% end -%>
  <%- end %>
  <%- else %>
  # no custom patches specified
  <%- end %>

  <% if gt (len .CustomScripts) 0 -%>
  scripts_dir="${HOME}/scripts"
  <% range $i, $r := .CustomScripts -%>
  retry git clone <% if $r.Branch %>--branch <% $r.Branch %><% end %> <% $r.Repo %> ${scripts_dir}/<% $i %>
  <% range $r.Scripts -%>
  log "Applying shell script <% . %>"
  . ${scripts_dir}/<% $i %>/<% . %>
  <% end -%>
  <%- end %>
  <%- else %>
  # no custom scripts specified
  <%- end %>

  <% if gt (len .CustomPrebuilts) 0 -%>
  prebuilt_dir="${AOSP_BUILD_DIR}/packages/apps/Custom"
  mk_file="${AOSP_BUILD_DIR}/build/make/target/product/handheld_system.mk"
  <% range $i, $r := .CustomPrebuilts -%>
  log "Putting custom prebuilts from <% $r.Repo %> in build tree location ${prebuilt_dir}/<% $i %>"
  retry git clone <% $r.Repo %> ${prebuilt_dir}/<% $i %>
  <% range .Modules -%>
  log "Adding custom PRODUCT_PACKAGES += <% . %> to ${mk_file}"
  sed -i "\$aPRODUCT_PACKAGES += <% . %>" "${mk_file}"
  <% end -%>
  <%- end %>
  <%- else %>
  # no custom prebuilts specified
  <%- end %>

  <% if .HostsFile -%>
  log "Replacing hosts file with ${HOSTS_FILE}"
  hosts_file_location="${AOSP_BUILD_DIR}/system/core/rootdir/etc/hosts"
  retry wget -q -O "${hosts_file_location}" "${HOSTS_FILE}"
  <%- else %>
  # no custom hosts file specified
  <%- end %>
}

patch_add_apps() {
  log_header "${FUNCNAME[0]}"

  handheld_system_mk="${AOSP_BUILD_DIR}/build/make/target/product/handheld_system.mk"
  sed -i "\$aPRODUCT_PACKAGES += Updater" "${handheld_system_mk}"
  sed -i "\$aPRODUCT_PACKAGES += F-DroidPrivilegedExtension" "${handheld_system_mk}"
  sed -i "\$aPRODUCT_PACKAGES += F-Droid" "${handheld_system_mk}"

  handheld_product_mk="${AOSP_BUILD_DIR}/build/make/target/product/handheld_product.mk"
  sed -i 's/Browser2 \\/TrichromeChrome \\/' "${handheld_product_mk}"

  media_product_mk="${AOSP_BUILD_DIR}/build/make/target/product/media_product.mk"
  sed -i 's/webview \\/TrichromeWebView \\/' "${media_product_mk}"

  <% if gt (len .CustomManifestProjects) 0 -%>
  <% range $i, $r := .CustomManifestProjects %><% range $j, $q := .Modules %>
  log "Adding custom PRODUCT_PACKAGES += <% $q %> to ${handheld_system_mk}"
  sed -i "\$aPRODUCT_PACKAGES += <% $q %>" "${handheld_system_mk}"
  <% end %>
  <% end %>
  <% else %>
  # no custom manifest projects specified
  <%- end %>
}