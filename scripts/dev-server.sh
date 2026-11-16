#!/bin/sh

set -eu

http_addr=${HTTP_ADDR:-:8080}
http_port=${http_addr##*:}
lan_ip=""

if command -v ipconfig >/dev/null 2>&1; then
	interface=""
	if command -v route >/dev/null 2>&1; then
		interface=$(route -n get default 2>/dev/null | awk '/interface:/{print $2; exit}')
	fi
	if [ -n "$interface" ]; then
		lan_ip=$(ipconfig getifaddr "$interface" 2>/dev/null || true)
	fi
	if [ -z "$lan_ip" ]; then
		lan_ip=$(ipconfig getifaddr en0 2>/dev/null || ipconfig getifaddr en1 2>/dev/null || true)
	fi
elif command -v hostname >/dev/null 2>&1; then
	lan_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
fi

local_url="http://localhost:$http_port"
network_url=""
if [ -n "$lan_ip" ]; then
	network_url="http://$lan_ip:$http_port"
fi

if [ -z "${TRUSTED_ORIGIN:-}" ]; then
	TRUSTED_ORIGIN=${network_url:-$local_url}
	export TRUSTED_ORIGIN
fi

printf '\nFlashka запущена:\n'
if [ -n "$network_url" ]; then
	printf '  На компьютере или телефоне по Wi-Fi: %s\n' "$network_url"
else
	printf '  На этом компьютере: %s\n' "$local_url"
	printf '  IP для телефона определить не удалось\n'
fi
printf '\nДля остановки нажмите Ctrl+C.\n\n'

exec go run ./cmd/server
