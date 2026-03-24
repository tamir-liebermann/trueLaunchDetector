# Alert Pipeline Test Plan

## Why alerts weren't arriving

The original implementation polled `oref.org.il` — which is **geo-blocked to Israeli IPs**.
Our server runs in Frankfurt, so every poll returned empty. The fix switches to the
**Tzevaadom WebSocket** (`wss://ws.tzevaadom.co.il/socket?platform=ANDROID`) which has
no geo-restrictions and pushes alerts in real time.

---

## Test 1 — WebSocket connectivity

Verify the server connects to Tzevaadom on startup.

```sh
flyctl logs --app truelaunchdetector | grep "Connected to Tzevaadom"
```

Expected: `Connected to Tzevaadom WebSocket`

---

## Test 2 — Health check

```sh
curl https://truelaunchdetector.fly.dev/health
```

Expected: `{"status":"ok"}`

---

## Test 3 — Inject a fake alert end-to-end

Tests the full pipeline: alert → subscriber → WhatsApp message.

```sh
# 1. Subscribe your number first
curl -s -X POST https://truelaunchdetector.fly.dev/chat \
  -H "Content-Type: application/json" \
  -d '{"user_text":"subscribe"}'

# 2. Inject a fake alert via the test endpoint
curl -s -X POST https://truelaunchdetector.fly.dev/test/alert \
  -H "Content-Type: application/json" \
  -d '{
    "title": "ירי רקטות וטילים",
    "data": ["אשדוד - צפון", "אשקלון - דרום"],
    "desc": "היכנסו למרחב המוגן ושהו בו 10 דקות"
  }'
```

Expected: WhatsApp message received within 5 seconds.

---

## Test 4 — Location prioritization

```sh
# Set location
curl -s -X POST https://truelaunchdetector.fly.dev/chat \
  -H "Content-Type: application/json" \
  -d '{"user_text":"my location is אשדוד"}'

# Inject alert targeting that area
curl -s -X POST https://truelaunchdetector.fly.dev/test/alert \
  -H "Content-Type: application/json" \
  -d '{"title":"ירי רקטות וטילים","data":["אשדוד - צפון"],"desc":"היכנסו למרחב המוגן"}'
```

Expected: Message prefixed with `🚨 *YOUR AREA*`

---

## Test 5 — Current alerts query

```sh
curl -s -X POST https://truelaunchdetector.fly.dev/chat \
  -H "Content-Type: application/json" \
  -d '{"user_text":"any alerts?"}'
```

Expected after injecting test alert: list of recent alerts.

---

## Test 6 — Summary

```sh
curl -s -X POST https://truelaunchdetector.fly.dev/chat \
  -H "Content-Type: application/json" \
  -d '{"user_text":"what happened today?"}'
```

Expected: Claude-generated summary of injected alerts.

---

## Test 7 — Reconnection

Kill and restart the server. Verify it reconnects automatically.

```sh
flyctl machine restart --app truelaunchdetector
sleep 10
flyctl logs --app truelaunchdetector | grep "Connected to Tzevaadom"
```

Expected: Reconnects within 5 seconds.
