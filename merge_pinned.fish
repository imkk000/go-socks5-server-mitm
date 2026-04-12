#!/usr/bin/env fish

# source from chromium
set url https://raw.githubusercontent.com/chromium/chromium/refs/heads/main/net/http/transport_security_state_static_pins.json
set chromium_list (curl $url | grep -v "^//" | jq -r '.entries[] | select(.pins != "test") | "s " + .name')

# source from firefox
set url https://hg-edge.mozilla.org/mozilla-central/raw-file/tip/security/manager/ssl/StaticHPKPins.h
set firefox_list (curl $url | grep -oP '"\K[^"]+(?=",)' | grep -v '^kPinset' | xargs -I{} echo "s {}")

# load base config
set base_list (cat base_config.txt)

# merge
string join \n $base_list $chromium_list $firefox_list | sort -u >config.txt
