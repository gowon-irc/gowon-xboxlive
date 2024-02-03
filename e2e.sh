#!/usr/bin/env bash


BROKER_HOST="${BROKER_HOST:-broker.emqx.io}"
BROKER_PORT="${BROKER_PORT:-1883}"

check_command() {
    if ! command -v "${1}" &> /dev/null ; then
        echo "${1} command not found"
        exit 1
    fi
}

check_command mosquitto_pub
check_command mosquitto_sub

get_command() {
    line="${1}"

    if [[ "${line:0:1}" == "." ]]; then
        f="${line%% *}"
        echo "${f#\.}"
    else
        echo ""
    fi
}

get_args() {
    line="${*}"

    command="$(get_command "${line}")"

    if [[ "${line}" = \.${command}* ]]; then
        echo "${line#\.${command} }"
    else
        echo "${line}"
    fi
}

mqtt_pub() {
    line="${1}"
    command="$(get_command "${line}")"
    args="$(get_args "${line}")"

	cat <<-EOF | mosquitto_pub -h "${BROKER_HOST}" -p "${BROKER_PORT}" -t "/gowon/input" -s
	{"module":"gowon","msg":"${line}","nick":"tester","dest":"#gowon","command":"${command}","args":"${args}"}
	EOF
}

extract_msg() {
    OUTPUT="${1##*msg\":\"}"
    OUTPUT="${OUTPUT%%\"*}"
    echo "${OUTPUT}"
}

blue() {
    echo "[0;34m${@}[0m"
}

green() {
    echo "[0;32m${@}[0m"
}

red() {
    echo "[0;31m${@}[0m"
}

# input message|expected output
TEST_LINES="$(
cat << EOF
.xbl invalid command|one of [s]et, [r]ecent, [a]chievements or [p]layer must be passed as a command
EOF
)"

MSG_COUNT="$(wc -l <<< "${TEST_LINES}")"

exec 3< <(mosquitto_sub -h "${BROKER_HOST}" -p "${BROKER_PORT}" -t "/gowon/output" -C "${MSG_COUNT}" &)

FAILED=false

while IFS="|" read INPUT EXPECTED ; do
    mqtt_pub "${INPUT}"
    read <&3 OUTPUT
    MSG="$(extract_msg "${OUTPUT}")"

    blue "${INPUT} -> ${EXPECTED}"
    if [[ "${MSG}" == "${EXPECTED}" ]]; then
        green "test passed"
    else
        red "test failed, got \"${MSG}\""
        FAILED=true
    fi
done <<< "${TEST_LINES}"

echo
if ! "${FAILED}" ; then
    green "End to end tests passed"
    RC=0
else
    red "End to end tests failed"
    RC=1
fi

exit "${RC}"
