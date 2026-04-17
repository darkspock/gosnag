#!/usr/bin/env bash
set -euo pipefail

DSN="${1:-}"
COUNT="${2:-200}"

if [ -z "$DSN" ]; then
  echo "Usage: $0 <DSN> [count]"
  echo "  DSN:   project DSN from settings (e.g. http://key@localhost:8099/5)"
  echo "  count: number of events to send (default: 200)"
  exit 1
fi

# Parse DSN: http://PUBLIC_KEY@HOST/PROJECT_ID
PROTO=$(echo "$DSN" | sed -E 's|^(https?)://.*|\1|')
KEY=$(echo "$DSN" | sed -E 's|^https?://([^@]+)@.*|\1|')
HOST=$(echo "$DSN" | sed -E 's|^https?://[^@]+@([^/]+).*|\1|')
PROJECT_ID=$(echo "$DSN" | sed -E 's|.*/([^/]+)$|\1|')
BASE="${PROTO}://${HOST}"

echo "Seeding ${COUNT} events to project ${PROJECT_ID} at ${BASE}"
echo "Public key: ${KEY}"
echo "Includes SQL breadcrumbs for DB analysis, N+1 heuristics, long-query preview, and EXPLAIN testing."

LEVELS=("error" "error" "error" "warning" "fatal")
RELEASES=("1.0.0" "1.0.1" "1.1.0" "1.2.0" "2.0.0-beta")
ENVS=("production" "staging" "development")
SERVERS=("web-01" "web-02" "worker-01" "api-01" "api-02")

uuid_lower() {
  uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-'
}

json_escape() {
  python3 - <<'PY' "$1"
import json, sys
print(json.dumps(sys.argv[1]))
PY
}

timestamp_days_ago() {
  local seconds_back="$1"
  date -u -v-"${seconds_back}"S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "-${seconds_back} seconds" +"%Y-%m-%dT%H:%M:%SZ"
}

build_n1_breadcrumbs() {
  local restaurant_id="$1"
  local date_value="$2"
  local values="["
  local user_id=$((RANDOM % 9000 + 1000))

  values+='{"category":"log","level":"info","message":"Loading booking calendar","timestamp":'$(date +%s)'}'

  for reserv_id in 101 102 103 104 105 106; do
    local duration
    duration=$(printf "%.2f" "$(awk "BEGIN { print 1 + (${reserv_id} % 5) * 0.37 }")")
    values+=',{"category":"db.query","level":"info","message":"SELECT * FROM reservs WHERE id_restaurant = '"${restaurant_id}"' AND id_reserv = '"${reserv_id}"'","data":{"duration_ms":'"${duration}"',"index":'"${reserv_id}"',"source":"local-seed"}}'
  done

  values+=',{"category":"db.query","level":"info","message":"SELECT id_reserv, time, `for`, tables, duration FROM reservs WHERE id_restaurant = '"${restaurant_id}"' AND date = '\'''"${date_value}"''\'' AND status IN (0,1,2) ORDER BY time ASC","data":{"duration_ms":6.40,"index":7,"source":"local-seed"}}'
  values+=',{"category":"db.query","level":"info","message":"SELECT id_client, email, phone FROM clients WHERE id_client = '"${user_id}"'","data":{"index":8,"source":"local-seed"}}'
  values+=']'
  printf '%s' "$values"
}

build_duplicate_breadcrumbs() {
  local restaurant_id="$1"
  local table_id="$2"
  local date_value="$3"
  cat <<EOF
[
  {"category":"db.query","level":"info","message":"SELECT * FROM tablespecial WHERE id_restaurant = '${restaurant_id}' AND date = '${date_value}' AND id_table = '${table_id}'","data":{"duration_ms":0.78,"index":1,"source":"local-seed"}},
  {"category":"db.query","level":"info","message":"INSERT INTO tablespecial (id_restaurant, date, id_table, name_table, luner, max, min, id_zone, x, y) VALUES ('${restaurant_id}', '${date_value}', ${table_id}, '', 1, '2', '1', '112914', '311', '548')","data":{"duration_ms":1.34,"index":2,"source":"local-seed"}},
  {"category":"db.query","level":"info","message":"UPDATE tablespecial SET x = 559, y = 116 WHERE id_restaurant = '${restaurant_id}' AND date = '${date_value}' AND id_table = '${table_id}'","data":{"duration_ms":0.92,"index":3,"source":"local-seed"}}
]
EOF
}

build_long_report_breadcrumbs() {
  local restaurant_id="$1"
  local start_date="$2"
  local end_date="$3"
  cat <<EOF
[
  {"category":"db.query","level":"info","message":"SELECT r.id_reserv, r.date, r.time, r.tables, r.duration, c.email, c.phone, p.channel, p.total_amount, p.currency, rs.service_name, rs.turn_name FROM reservs r LEFT JOIN clients c ON c.id_client = r.id_client LEFT JOIN payments p ON p.id_reserv = r.id_reserv LEFT JOIN reserv_services rs ON rs.id_reserv = r.id_reserv WHERE r.id_restaurant = '${restaurant_id}' AND r.date BETWEEN '${start_date}' AND '${end_date}' AND (r.status = 0 OR r.status = 1 OR r.status = 2 OR r.status = 4) AND (c.email IS NOT NULL OR c.phone IS NOT NULL) ORDER BY r.date ASC, r.time ASC, r.id_reserv ASC","data":{"duration_ms":48.60,"index":1,"source":"local-seed"}},
  {"category":"db.query","level":"info","message":"SELECT COUNT(*) AS total FROM reservs WHERE id_restaurant = '${restaurant_id}' AND date BETWEEN '${start_date}' AND '${end_date}'","data":{"duration_ms":3.90,"index":2,"source":"local-seed"}}
]
EOF
}

build_payment_breadcrumbs() {
  local restaurant_id="$1"
  local order_id="$2"
  cat <<EOF
[
  {"category":"db.query","level":"info","message":"SELECT id_order, id_restaurant, status, total_amount, currency FROM orders WHERE id_restaurant = '${restaurant_id}' AND id_order = '${order_id}'","data":{"duration_ms":2.10,"index":1,"source":"local-seed"}},
  {"category":"db.query","level":"info","message":"SELECT id_payment, provider, provider_reference, amount, currency FROM payments WHERE id_order = '${order_id}'","data":{"duration_ms":1.25,"index":2,"source":"local-seed"}},
  {"category":"db.query","level":"info","message":"UPDATE orders SET status = 'captured', updated_at = NOW() WHERE id_order = '${order_id}'","data":{"duration_ms":4.70,"index":3,"source":"local-seed"}}
]
EOF
}

build_scenario() {
  local scenario="$1"
  local url method err_type err_msg breadcrumbs transaction

  case "$scenario" in
    n1)
      local restaurant_id=$((RANDOM % 9000 + 1000))
      local day=$((RANDOM % 20 + 1))
      local month=$((RANDOM % 6 + 4))
      local year=2026
      local date_value
      date_value=$(printf "%04d-%02d-%02d" "$year" "$month" "$day")
      url="/coverApp/Reserv/getCalendar/4/2026"
      method="GET"
      err_type="Error"
      err_msg="[ExcessiveQueries] 184 queries (117.82ms total) — ${url}"
      breadcrumbs=$(build_n1_breadcrumbs "$restaurant_id" "$date_value")
      transaction="${method} ${url}"
      ;;
    duplicate)
      local restaurant_id=$((RANDOM % 9000 + 1000))
      local table_id=$((RANDOM % 30 + 1))
      local date_value="2026-04-17"
      url="/Tables/update_table_position"
      method="POST"
      err_type="mysqli_sql_exception"
      err_msg="Duplicate entry '${restaurant_id}-${date_value}-${table_id}-1' for key 'tablespecial.PRIMARY'"
      breadcrumbs=$(build_duplicate_breadcrumbs "$restaurant_id" "$table_id" "$date_value")
      transaction="${method} ${url}"
      ;;
    report)
      local restaurant_id=$((RANDOM % 9000 + 1000))
      url="/reports/export-bookings"
      method="GET"
      err_type="TimeoutError"
      err_msg="Report query exceeded timeout while exporting bookings"
      breadcrumbs=$(build_long_report_breadcrumbs "$restaurant_id" "2026-04-01" "2026-04-30")
      transaction="${method} ${url}"
      ;;
    payment)
      local restaurant_id=$((RANDOM % 9000 + 1000))
      local order_id=$((RANDOM % 50000 + 10000))
      url="/api/v1/payments/capture"
      method="POST"
      err_type="Error"
      err_msg="Payment capture failed with status 500"
      breadcrumbs=$(build_payment_breadcrumbs "$restaurant_id" "$order_id")
      transaction="${method} ${url}"
      ;;
    *)
      echo "Unknown scenario: $scenario" >&2
      return 1
      ;;
  esac

  printf '%s\n%s\n%s\n%s\n%s\n%s' "$url" "$method" "$err_type" "$err_msg" "$breadcrumbs" "$transaction"
}

OK=0
FAIL=0

for i in $(seq 1 "$COUNT"); do
  case $((i % 4)) in
    1) SCENARIO="n1" ;;
    2) SCENARIO="duplicate" ;;
    3) SCENARIO="report" ;;
    0) SCENARIO="payment" ;;
  esac

  mapfile -t SCENARIO_DATA < <(build_scenario "$SCENARIO")
  URL="${SCENARIO_DATA[0]}"
  METHOD="${SCENARIO_DATA[1]}"
  ERR_TYPE="${SCENARIO_DATA[2]}"
  ERR_MSG="${SCENARIO_DATA[3]}"
  BREADCRUMBS="${SCENARIO_DATA[4]}"
  TRANSACTION="${SCENARIO_DATA[5]}"

  LEVEL="${LEVELS[$((RANDOM % ${#LEVELS[@]}))]}"
  RELEASE="${RELEASES[$((RANDOM % ${#RELEASES[@]}))]}"
  ENV="${ENVS[$((RANDOM % ${#ENVS[@]}))]}"
  SERVER="${SERVERS[$((RANDOM % ${#SERVERS[@]}))]}"

  OFFSET=$((RANDOM % 604800))
  TS=$(timestamp_days_ago "$OFFSET")
  EVENT_ID=$(uuid_lower)

  PAYLOAD=$(cat <<EOF
{
  "event_id": "${EVENT_ID}",
  "timestamp": "${TS}",
  "level": "${LEVEL}",
  "platform": "php",
  "release": "${RELEASE}",
  "environment": "${ENV}",
  "server_name": "${SERVER}",
  "transaction": "${TRANSACTION}",
  "message": $(json_escape "${ERR_MSG}"),
  "exception": {
    "values": [{
      "type": "${ERR_TYPE}",
      "value": $(json_escape "${ERR_MSG}"),
      "stacktrace": {
        "frames": [
          {"filename": "/application/controllers/Reserv.php", "abs_path": "/var/app/current/application/controllers/Reserv.php", "lineno": 587, "function": "Reserv::getCalendar", "in_app": true},
          {"filename": "/application/models/Tablemodel.php", "abs_path": "/var/app/current/application/models/Tablemodel.php", "lineno": 863, "function": "Tablemodel::modify_single_table", "in_app": true},
          {"filename": "/system/database/DB_driver.php", "abs_path": "/var/app/current/system/database/DB_driver.php", "lineno": 655, "function": "CI_DB_driver::query", "in_app": false}
        ]
      }
    }]
  },
  "request": {
    "method": "${METHOD}",
    "url": "https://www.covermanager.com${URL}",
    "headers": {
      "User-Agent": "Mozilla/5.0",
      "Content-Type": "application/json"
    }
  },
  "breadcrumbs": {
    "values": ${BREADCRUMBS}
  },
  "tags": {
    "browser": "Chrome",
    "os": "macOS",
    "seed_scenario": "${SCENARIO}"
  },
  "user": {
    "id": "user-$((RANDOM % 50))",
    "email": "user$((RANDOM % 50))@example.com"
  }
}
EOF
)

  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${BASE}/api/${PROJECT_ID}/store/" \
    -H "Content-Type: application/json" \
    -H "X-Sentry-Auth: Sentry sentry_key=${KEY}" \
    -d "$PAYLOAD")

  if [ "$STATUS" = "200" ]; then
    OK=$((OK + 1))
  else
    FAIL=$((FAIL + 1))
  fi

  if [ $((i % 20)) -eq 0 ]; then
    echo "  ${i}/${COUNT} sent (ok: ${OK}, fail: ${FAIL})"
  fi
done

echo ""
echo "Done: ${OK} ok, ${FAIL} failed out of ${COUNT}"
echo ""
echo "Seeded scenarios:"
echo "  - n1: repeated SELECTs with one missing duration_ms"
echo "  - duplicate: SELECT + INSERT + UPDATE"
echo "  - report: very long SELECT for query preview/modal testing"
echo "  - payment: mixed SELECT + UPDATE"
