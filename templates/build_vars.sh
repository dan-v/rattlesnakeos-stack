ANDROID_VERSION="11.0"
DEVICE="<% .Device %>"
DEVICE_FAMILY="<% .DeviceDetails.Family %>"
DEVICE_COMMON="<% .DeviceDetails.Common %>"
AVB_MODE="<% .DeviceDetails.AVBMode %>"
EXTRA_OTA=<% .DeviceDetails.ExtraOTA %>
REGION="<% .Region %>"
STACK_NAME="<% .Name %>"
STACK_VERSION="<% .Version %>"
ENCRYPTED_KEYS="<% .EncryptedKeys %>"
ENCRYPTION_KEY=
ENCRYPTION_PIPE="/tmp/key"
BUILD_TYPE="user"
BUILD_CHANNEL="stable"
HOSTS_FILE="<% .HostsFile %>"

AWS_KEYS_BUCKET="${STACK_NAME}-keys"
AWS_ENCRYPTED_KEYS_BUCKET="${STACK_NAME}-keys-encrypted"
AWS_RELEASE_BUCKET="${STACK_NAME}-release"
AWS_LOGS_BUCKET="${STACK_NAME}-logs"
AWS_SNS_ARN=$(aws --region ${REGION} sns list-topics --query 'Topics[0].TopicArn' --output text | cut -d":" -f1,2,3,4,5)":${STACK_NAME}"
INSTANCE_TYPE=$(curl -s http://169.254.169.254/latest/meta-data/instance-type)
INSTANCE_REGION=$(curl -s http://169.254.169.254/latest/dynamic/instance-identity/document | awk -F\" '/region/ {print $4}')
INSTANCE_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4)

BUILD_TARGET="release aosp_${DEVICE} ${BUILD_TYPE}"
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
STACK_URL_LATEST="https://api.github.com/repos/dan-v/rattlesnakeos-stack/releases/latest"
RATTLESNAKEOS_LATEST_JSON="https://raw.githubusercontent.com/RattlesnakeOS/latest/${ANDROID_VERSION}/latest.json"

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