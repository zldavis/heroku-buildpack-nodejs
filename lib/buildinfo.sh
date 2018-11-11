source $BP_DIR/lib/kvstore.sh

BUILDINFO_FILE="$CACHE_DIR/build-info/node"

buildinfo_create() {
  kv_create $BUILDINFO_FILE
}

buildinfo_set() {
  kv_set $BUILDINFO_FILE $1 $2 
}

buildinfo_time() {
  local start="${2}"
  local end="${3:-$(nowms)}"
  local time="$(echo \"${start}\" \"${end}\" | awk '{ printf "%.3f", ($2 - $1)/1000 }')"
  kv_set $BUILDINFO_FILE $1 "$time"
}

log_buildinfo() {
  kv_list $BUILDINFO_FILE
}

# bootstrap
buildinfo_create
