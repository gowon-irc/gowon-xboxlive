#!/usr/bin/env bash

BROKER_HOST="${BROKER_HOST:-localhost}"
BROKER_PORT="${BROKER_PORT:-1883}"

ACTION="${1}"

mqtt_msg() {
	cat <<-EOF
	{"module":"gowon","msg":".xbl $@","nick":"tester","dest":"#gowon","command":"xbl","args":"${@}"}
	EOF
}

pub() {
    mqtt_msg "${@}" | mosquitto_pub -h "${BROKER_HOST}" -p "${BROKER_PORT}" -t "/gowon/input" -s
}

sub() {
    mosquitto_sub -h "${BROKER_HOST}" -p "${BROKER_PORT}" -t "/gowon/output" | jq -r '.msg'
}

case "${ACTION}" in
    pub)
        pub "${@:2}"
        ;;
    sub)
        sub "${@:2}"
        ;;
    *)
        echo "First argument must be either pub or sub" >&2
        ;;
esac
