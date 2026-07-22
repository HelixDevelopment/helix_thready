# Max ("OneMe") Messenger Protocol — Research Notes

Research target for the `digital.vasic.maxadapter` Go adapter. Covers the two
Max (max.ru, by VK; internal codename **OneMe**) surfaces relevant to Helix
Thready ingestion: the **official Bot API** (REST) and the **internal user-session
WebSocket "OneMe" API** used by community reverse-engineered clients.

## Conventions

- **CONFIRMED** — read directly from primary source: reference-client source code
  or official docs (URL + file cited inline). Reproducible.
- **INFERRED** — deduced from surrounding evidence (send-side shapes, partial
  captures, cross-client agreement) but **not** verified against a live server in
  this environment. Treat as a hypothesis the on-wire capture must confirm.
- **UNVERIFIED** — claimed by a source but not corroborated; flagged as such.
- This is a **reverse-engineered** protocol. The internal WebSocket API is
  undocumented by VK; everything in §2 originates from community clients. No live
  Max account was available, so nothing here was confirmed against a running
  server. The Go mapper (`ParseHistory`) is built to the **CONFIRMED** message
  field names and defensively handles the **INFERRED** reply/forward encodings.

Date of research: 2026-07-22.

### Primary sources

| # | Source | What it gave us |
|---|--------|-----------------|
| S1 | `nsdkinx/vkmax` — Python client for VK MAX (OneMe). https://github.com/nsdkinx/vkmax | Envelope, opcodes, auth flow (opcodes 17/18/19), send/reply/forward shapes. |
| S2 | `vkmax/docs/opcodes.md` https://github.com/nsdkinx/vkmax/blob/main/docs/opcodes.md | Opcode table (19 auth, 32, 49 history, 50, 57, 64, 66, 67, 75, 77, 178, 22). |
| S3 | `vkmax/docs/protocol.md` (in repo) | WebSocket endpoint `wss://ws-api.oneme.ru/websocket`; START_AUTH + CHECK_CODE. |
| S4 | `Sharkow1743/MaxAPI` — Python userbot wrapper. https://github.com/Sharkow1743/MaxAPI (`MaxBridge/max_api.py`, `max_api_doc.md`) | Opcode map incl. 1 heartbeat / 6 handshake / 19 auth / 49 `get_history`; history request payload; `cmd` response codes. |
| S5 | `renosaza/max-mcp` — MCP server, has `dump_channel`. https://github.com/renosaza/max-mcp (`src/max_mcp/normalize.py`) | Normalized message field names (`id`, `chat_id`, `sender`, `text`, `time`, `type`, `prev_message_id`, `attaches`); reply detection. |
| S6 | `MaxApiTeam/PyMax` (`maxapi-python`) — the wrapper max-mcp runs on. https://github.com/MaxApiTeam/PyMax (`src/pymax/types/domain/message.py`, `api/messages/service.py`, `api/messages/payloads.py`) | Authoritative `Message` model (camelCase wire aliases), `fetch_history` (opcode 49, payload key `messages`), REPLY/FORWARD `link` shapes, attachment `_type` discriminator + field names. |
| S7 | Official Bot API — https://dev.max.ru/docs and https://dev.max.ru/docs-api | REST base host, token issuance via @MasterBot, `Authorization` header. |

---

## 1. Official Bot API (REST) — CONFIRMED (docs)

The public, supported surface. Bots only — cannot read arbitrary channels or act
as a user (that is what §2 is for).

- **Base host:** `https://botapi.max.ru` (a.k.a. platform-api host). [S7, and
  multiple SDKs] — CONFIRMED as the documented REST host.
- **Auth:** register on `dev.max.ru`, obtain a bot token from **@MasterBot** in
  the app (`/create`, pick an `*_bot` nickname). Requests authenticate with the
  token in the `Authorization` header (the old query-param method is dropped).
  [S7] — CONFIRMED.
- **Capabilities:** send/receive messages, manage chats, upload files, receive
  updates (long-poll or webhook). [S7] — CONFIRMED.
- **Relevance to Thready:** insufficient on its own for "dump a channel's
  history as a user"; the Bot API sees only chats the bot is a member of and
  updates delivered to it. History back-fill of arbitrary channels needs §2.

This adapter targets §2 (the OneMe user session) because ThreadReader needs full
thread history. The Bot API remains a future option for the *send/notify* side.

---

## 2. Internal "OneMe" WebSocket API — reverse-engineered

### 2.1 Transport & envelope — CONFIRMED

- **Endpoint:** `wss://ws-api.oneme.ru/websocket` — CONFIRMED [S1
  `vkmax/vkmax/client.py`: `WS_HOST = "wss://ws-api.oneme.ru/websocket"`; S3].
- **Request envelope** — CONFIRMED [S4 `max_api.py` `send_command_async`; S2]:

  ```json
  { "ver": 11, "cmd": 0, "seq": <int>, "opcode": <int>, "payload": { ... } }
  ```

  `ver` = protocol version (11 observed). `cmd` = 0 for a request. `seq` =
  monotonic sequence id, echoed in the matching response.

- **Response / event `cmd` codes** — CONFIRMED [S4 `_process_message`]:
  - `cmd: 1` → response to a request (matched by `seq`).
  - `cmd: 0` → server-initiated event (e.g. incoming message push).
  - `cmd: 3` → API error.

### 2.2 Opcodes (subset relevant to read-history) — CONFIRMED

| Opcode | Name | Purpose |
|-------:|------|---------|
| 1 | heartbeat | keep-alive, send `{ "interactive": false }` every ~5–10 s [S4] |
| 6 | handshake | first frame: `{ "userAgent": { "deviceType": "WEB" }, "deviceId": "…" }` [S1,S4] |
| 17 | START_AUTH | request SMS code for a phone [S1,S3] |
| 18 | CHECK_CODE | verify SMS code → session token [S1,S3] |
| 19 | login/authenticate | token login + initial sync [S1,S2,S4] |
| 32 | get contact details | resolve users [S2,S4] |
| 48 | resolve chat by id | `{ "chatIds": [...] }` [S1 `channels.py`] |
| 49 | **get history** | fetch chat messages [S1,S2,S4,S6] |
| 50 | mark as read | [S2,S4] |
| 57 | subscribe/join by link | `{ "link": "https://max.ru/<name>" }` [S1,S2] |
| 64 | send message | [S1,S2,S4,S6] |
| 66 | delete / 67 edit / 75 leave / 77 members / 178 react / 22 settings | [S2] |
| 83 | get video / 88 get file | resolve media download URLs [S4] |

### 2.3 Auth flow (phone → token) — CONFIRMED

From S1 `vkmax/vkmax/client.py`:

1. **Handshake** (opcode 6) — sent before auth: `{ "userAgent": {...}, "deviceId": <id> }`.
2. **START_AUTH** (opcode 17):
   ```json
   { "phone": "<phone>", "type": "START_AUTH", "language": "ru" }
   ```
   Response carries an SMS token at `payload.token`. [S1 `send_code`]
3. **CHECK_CODE** (opcode 18):
   ```json
   { "token": "<sms_token>", "verifyCode": "<code>", "authTokenType": "CHECK_CODE" }
   ```
   On success, the durable **LOGIN token** is at
   `payload.tokenAttrs.LOGIN.token`; profile at `payload.profile.contact`.
   [S1 `sign_in` docstring + S3]
4. **Token login** (opcode 19) — for later silent re-login:
   ```json
   { "interactive": true, "token": "<login_token>",
     "chatsCount": 40, "chatsSync": 0, "contactsSync": 0,
     "presenceSync": -1, "draftsSync": 0 }
   ```
   [S1 `login_by_token`; S4 `max_api_doc.md` authenticate example]

### 2.4 Read-history / channel-dump flow — CONFIRMED request, CONFIRMED core fields

**Request** — opcode 49 [S4 `max_api.py get_history`, S6 `fetch_history` +
`ChatHistoryPayload`]:

```json
{ "chatId": <int>, "from": <epoch_ms>, "forward": 0,
  "backward": <count>, "getMessages": true }
```

- `from` = anchor timestamp in **epoch milliseconds** (default: now) — CONFIRMED
  [S4 `from_timestamp = int(time.time()*1000)`; S6 `ChatHistoryPayload.from_`].
- `backward` = number of messages to walk backward from `from` (page size).
- `forward` = messages after the anchor (0 for pure back-fill).
- Channel "dump" = repeat opcode-49 paging backward until the list is empty;
  `renosaza/max-mcp`'s `dump_channel` is this loop over `maxapi-python` [S5].

**Response** — `cmd: 1`, `payload.messages` is the message array — CONFIRMED
[S6 `fetch_history` → `parse_payload_list(response, MessagePayloadKey.MESSAGES, …)`,
where `MessagePayloadKey.MESSAGES = "messages"`].

### 2.5 Message object shape — CONFIRMED fields, INFERRED reply/forward on-wire echo

Field names are the **camelCase wire aliases** of PyMax's `Message` model
(`CamelModel` uses `alias_generator=to_camel`) — CONFIRMED [S6
`types/domain/message.py`, `types/domain/base.py`]:

| Wire field | Type | Meaning | Provenance |
|------------|------|---------|-----------|
| `id` | int **or** string | message id | CONFIRMED [S6 field `id`; S1/S4 pass `messageId` as string in other ops → both forms occur] |
| `chatId` | int | owning chat | CONFIRMED [S6 unwrap injects `chatId`] |
| `sender` | int | author user id | CONFIRMED [S6 `sender`; S5 `sender`] |
| `text` | string | message text | CONFIRMED [S6 `text`] |
| `time` | int | **epoch milliseconds** | CONFIRMED [S6 `time`; consistent with request `from` in ms] |
| `type` | string | message type (`TEXT`, `REPLY`, `FORWARD`, …) | CONFIRMED [S6 `type`; S5 checks `type=="REPLY"`] |
| `cid` | int | client id (send-side dedup) | CONFIRMED [S6 `cid`] |
| `attaches` | array | attachments (see 2.6) | CONFIRMED [S6 `attaches`] |
| `elements` | array | rich-text spans | CONFIRMED [S6 `elements`] |
| `prevMessageId` | int/str | previous / replied-to message id | CONFIRMED field [S6 `prevMessageId`; S5 uses it as reply target] |
| `reactionInfo` | object | reactions | CONFIRMED [S6] |
| `options`,`status`,`stats`,`ttl`,`unread`,`mark` | mixed | misc | CONFIRMED [S6] |

**Reply / forward linkage** — this is the one genuinely ambiguous spot:

- On the **send** side (CONFIRMED), a reply is built as
  `message.link = { "type": "REPLY", "messageId": <id> }` and a forward as
  `message.link = { "type": "FORWARD", "messageId": <id>, "chatId": <src> }`
  [S1 `messages.py`; S6 `ReplyLink` / `ForwardLink`].
- On the **received** side there are **two observed encodings**:
  1. an echoed `link` object of the same `{type, messageId, chatId}` shape
     (INFERRED — the natural server echo of the send shape; not captured live here);
  2. a top-level `prevMessageId` (+ `type == "REPLY"`) used by `max-mcp`'s
     normalizer as the reply target — CONFIRMED that the client reads it [S5
     `_reply_to_id`].
- **Mapper decision:** `ParseHistory` handles **both** — it uses
  `link.messageId` when `link.type == "REPLY"`, else falls back to
  `prevMessageId`; `link.type == "FORWARD"` sets `IsForwarded` and is treated as
  a cross-chat reference (it does **not** populate `ParentID`). A FORWARD is not
  a within-thread parent edge.

### 2.6 Attachments — CONFIRMED

Attachments are discriminated by a **`_type`** field [S6
`types/domain/attachments/*` use `Field(alias="_type")`; enum values in
`attachments/enums.py`]: `PHOTO, VIDEO, FILE, STICKER, AUDIO, CONTROL, CONTACT,
CALL, SHARE, INLINE_KEYBOARD, UNKNOWN`.

Per-kind fields the adapter reads — CONFIRMED [S6]:

- `PHOTO`: `photoId`, `photoToken`, `baseUrl`, `width`, `height`.
- `VIDEO`: `videoId` (+ token/url resolved later via opcode 83).
- `FILE`: `fileId`, `name`, `size`, `token` (download URL via opcode 88).
- `SHARE`: link-preview (`url`, `title`, `description`) — not a real file.

**No MIME type and no content hash are transmitted.** The adapter therefore
INFERS a coarse MIME (`image/*`, `video/*`, `audio/*`, or extension-derived for
`FILE`) and leaves `SHA256` empty; the Asset Service computes the real values on
download. Documented as inference, not ground truth.

---

## 3. What the Go mapper implements vs. what stays [BUILD-NEW]

- **REAL + tested (`ParseHistory`, `wire.go`):** opcode-49 `payload.messages`
  array → `[]Post`, using the CONFIRMED field names above and both reply
  encodings. Covered by `parse_test.go` (offline, canned payloads).
- **[BUILD-NEW] stub (`client.go`):** the live socket, handshake, phone/token
  auth, and the opcode-49 send/receive. Returns `ErrNotImplemented`. These need a
  real Max account and an on-wire capture to confirm the INFERRED pieces (chiefly
  the received-message `link` echo and the exact `time` unit per opcode).

### Open items to confirm against a live capture

1. Received-message reply encoding: echoed `link` object vs `prevMessageId`
   (which appears when).
2. Whether `id` in a history response is consistently numeric or string.
3. Exact `time` unit per field/opcode (message `time` is ms; some presence
   fields are seconds — the mapper normalizes ms→s defensively).
4. Any pagination cursor beyond `from`/`backward` for very large channels.
