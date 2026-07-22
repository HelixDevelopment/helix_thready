//! Helix Thready — typed, **std-only** Rust SDK client for the REST `/v1`
//! control API.
//!
//! Schema of record: `docs/public/research/mvp/api/openapi.yaml`; realized by
//! the `implementation/rest_gateway` module. The typed surface mirrors the
//! sibling Go SDK (`implementation/sdk_go`).
//!
//! # std-only, on purpose
//!
//! This crate uses **only** the Rust standard library — no `cargo`, no crates
//! (no `reqwest`/`serde`/`tokio`/`uuid`). Because `std` ships neither an HTTP
//! client nor a JSON codec, both are hand-rolled here:
//!
//! * [`http`] — a minimal **blocking HTTP/1.1** client over
//!   [`std::net::TcpStream`]: it serializes the request bytes (method, path,
//!   `Host`, headers, body), reads the response, and parses the status line +
//!   headers + body (by `Content-Length`).
//! * [`json`] — a [`json::Value`] enum with a recursive-descent parser and a
//!   compact encoder sufficient for the `/v1` contract shapes.
//!
//! ## Transport: http works, https needs a crate
//!
//! `std` has **no TLS**, so the only transport this crate can actually *speak*
//! is plaintext `http`. Talking to an `https` origin needs an external TLS
//! crate (e.g. `rustls`/`native-tls`), which is out of scope for a std-only
//! build. Crucially, the **insecure-transport guard still treats an `https`
//! base URL as safe** (never `InsecureTransport`): the guard's job is to refuse
//! leaking a credential over *plaintext* http to a remote host, and `https` is
//! exactly the case it must *not* refuse — so [`ThreadyClient::transport_allowed`]
//! returns `true` for any `https` URL.
//!
//! # What the client does
//!
//! * Bearer-wins auth injection (`Authorization: Bearer …` beats `X-API-Key`).
//! * Typed [`Error::Api`] mapped from the canonical
//!   `{"error":{"code","message","request_id"}}` envelope.
//! * Transparent retry of **idempotent GETs** on `503`/`429` with capped
//!   exponential backoff.
//! * An `Idempotency-Key` stamped onto **unsafe POSTs** (`create_channel`,
//!   `reprocess`).
//! * An insecure-transport guard that returns [`Error::InsecureTransport`]
//!   **before connecting** for plaintext-http to a non-loopback host.

#![allow(dead_code)]

use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Mutex;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

// ===========================================================================
// json — a minimal Value + recursive-descent parser + compact encoder.
// ===========================================================================

/// A minimal, hand-rolled JSON codec (no `serde`).
pub mod json {
    /// A JSON value. Objects preserve insertion order (a `Vec` of pairs) so the
    /// encoder emits deterministic output.
    #[derive(Debug, Clone, PartialEq)]
    pub enum Value {
        Null,
        Bool(bool),
        Num(f64),
        Str(String),
        Array(Vec<Value>),
        Object(Vec<(String, Value)>),
    }

    impl Value {
        /// Look up a key on an object value.
        pub fn get(&self, key: &str) -> Option<&Value> {
            match self {
                Value::Object(m) => m.iter().find(|(k, _)| k == key).map(|(_, v)| v),
                _ => None,
            }
        }
        pub fn as_str(&self) -> Option<&str> {
            match self {
                Value::Str(s) => Some(s),
                _ => None,
            }
        }
        pub fn as_f64(&self) -> Option<f64> {
            match self {
                Value::Num(n) => Some(*n),
                _ => None,
            }
        }
        pub fn as_bool(&self) -> Option<bool> {
            match self {
                Value::Bool(b) => Some(*b),
                _ => None,
            }
        }
        pub fn as_array(&self) -> Option<&Vec<Value>> {
            match self {
                Value::Array(a) => Some(a),
                _ => None,
            }
        }
    }

    /// Encode a [`Value`] to a compact JSON string.
    pub fn encode(v: &Value) -> String {
        let mut s = String::new();
        enc(v, &mut s);
        s
    }

    fn enc(v: &Value, s: &mut String) {
        match v {
            Value::Null => s.push_str("null"),
            Value::Bool(true) => s.push_str("true"),
            Value::Bool(false) => s.push_str("false"),
            Value::Num(n) => {
                if n.is_finite() && n.fract() == 0.0 && n.abs() < 9.007_199_254_740_992e15 {
                    s.push_str(&(*n as i64).to_string());
                } else {
                    s.push_str(&n.to_string());
                }
            }
            Value::Str(x) => enc_str(x, s),
            Value::Array(a) => {
                s.push('[');
                for (i, e) in a.iter().enumerate() {
                    if i > 0 {
                        s.push(',');
                    }
                    enc(e, s);
                }
                s.push(']');
            }
            Value::Object(m) => {
                s.push('{');
                for (i, (k, val)) in m.iter().enumerate() {
                    if i > 0 {
                        s.push(',');
                    }
                    enc_str(k, s);
                    s.push(':');
                    enc(val, s);
                }
                s.push('}');
            }
        }
    }

    fn enc_str(x: &str, s: &mut String) {
        s.push('"');
        for c in x.chars() {
            match c {
                '"' => s.push_str("\\\""),
                '\\' => s.push_str("\\\\"),
                '\n' => s.push_str("\\n"),
                '\r' => s.push_str("\\r"),
                '\t' => s.push_str("\\t"),
                '\u{08}' => s.push_str("\\b"),
                '\u{0C}' => s.push_str("\\f"),
                c if (c as u32) < 0x20 => s.push_str(&format!("\\u{:04x}", c as u32)),
                c => s.push(c),
            }
        }
        s.push('"');
    }

    /// Parse a JSON document into a [`Value`]. Returns a human-readable error
    /// string on malformed input.
    pub fn parse(input: &str) -> Result<Value, String> {
        let mut p = P {
            c: input.chars().collect(),
            i: 0,
        };
        p.ws();
        let v = p.value()?;
        p.ws();
        Ok(v)
    }

    struct P {
        c: Vec<char>,
        i: usize,
    }

    impl P {
        fn ws(&mut self) {
            while self.i < self.c.len() && self.c[self.i].is_ascii_whitespace() {
                self.i += 1;
            }
        }
        fn peek(&self) -> Option<char> {
            self.c.get(self.i).copied()
        }
        fn lit(&mut self, word: &str) -> bool {
            let w: Vec<char> = word.chars().collect();
            if self.i + w.len() <= self.c.len() && self.c[self.i..self.i + w.len()] == w[..] {
                self.i += w.len();
                true
            } else {
                false
            }
        }

        fn value(&mut self) -> Result<Value, String> {
            self.ws();
            match self.peek() {
                None => Err("unexpected end of input".into()),
                Some('{') => self.object(),
                Some('[') => self.array(),
                Some('"') => Ok(Value::Str(self.string()?)),
                Some('t') | Some('f') => self.boolean(),
                Some('n') => self.null(),
                Some(c) if c == '-' || c.is_ascii_digit() => self.number(),
                Some(c) => Err(format!("unexpected character '{}'", c)),
            }
        }

        fn object(&mut self) -> Result<Value, String> {
            self.i += 1; // consume '{'
            let mut m: Vec<(String, Value)> = Vec::new();
            self.ws();
            if self.peek() == Some('}') {
                self.i += 1;
                return Ok(Value::Object(m));
            }
            loop {
                self.ws();
                if self.peek() != Some('"') {
                    return Err("expected string key in object".into());
                }
                let key = self.string()?;
                self.ws();
                if self.peek() != Some(':') {
                    return Err("expected ':' in object".into());
                }
                self.i += 1;
                let val = self.value()?;
                m.push((key, val));
                self.ws();
                match self.peek() {
                    Some(',') => {
                        self.i += 1;
                    }
                    Some('}') => {
                        self.i += 1;
                        break;
                    }
                    _ => return Err("expected ',' or '}' in object".into()),
                }
            }
            Ok(Value::Object(m))
        }

        fn array(&mut self) -> Result<Value, String> {
            self.i += 1; // consume '['
            let mut a: Vec<Value> = Vec::new();
            self.ws();
            if self.peek() == Some(']') {
                self.i += 1;
                return Ok(Value::Array(a));
            }
            loop {
                let val = self.value()?;
                a.push(val);
                self.ws();
                match self.peek() {
                    Some(',') => {
                        self.i += 1;
                    }
                    Some(']') => {
                        self.i += 1;
                        break;
                    }
                    _ => return Err("expected ',' or ']' in array".into()),
                }
            }
            Ok(Value::Array(a))
        }

        fn string(&mut self) -> Result<String, String> {
            self.i += 1; // consume opening quote
            let mut out = String::new();
            loop {
                let ch = self.peek().ok_or("unterminated string")?;
                self.i += 1;
                match ch {
                    '"' => break,
                    '\\' => {
                        let e = self.peek().ok_or("bad escape at end of input")?;
                        self.i += 1;
                        match e {
                            '"' => out.push('"'),
                            '\\' => out.push('\\'),
                            '/' => out.push('/'),
                            'n' => out.push('\n'),
                            'r' => out.push('\r'),
                            't' => out.push('\t'),
                            'b' => out.push('\u{08}'),
                            'f' => out.push('\u{0C}'),
                            'u' => {
                                let cp = self.hex4()?;
                                if (0xD800..=0xDBFF).contains(&cp) {
                                    // High surrogate: try to combine with a low.
                                    if self.peek() == Some('\\') {
                                        self.i += 1;
                                        if self.peek() == Some('u') {
                                            self.i += 1;
                                            let lo = self.hex4()?;
                                            if (0xDC00..=0xDFFF).contains(&lo) {
                                                let c = 0x10000
                                                    + ((cp - 0xD800) << 10)
                                                    + (lo - 0xDC00);
                                                if let Some(ch) = char::from_u32(c) {
                                                    out.push(ch);
                                                }
                                            }
                                        }
                                    }
                                } else if let Some(ch) = char::from_u32(cp) {
                                    out.push(ch);
                                }
                            }
                            _ => return Err(format!("invalid escape '\\{}'", e)),
                        }
                    }
                    c => out.push(c),
                }
            }
            Ok(out)
        }

        fn hex4(&mut self) -> Result<u32, String> {
            if self.i + 4 > self.c.len() {
                return Err("truncated \\u escape".into());
            }
            let s: String = self.c[self.i..self.i + 4].iter().collect();
            self.i += 4;
            u32::from_str_radix(&s, 16).map_err(|_| format!("bad \\u escape '{}'", s))
        }

        fn number(&mut self) -> Result<Value, String> {
            let start = self.i;
            while let Some(c) = self.peek() {
                if c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' || c.is_ascii_digit() {
                    self.i += 1;
                } else {
                    break;
                }
            }
            let s: String = self.c[start..self.i].iter().collect();
            s.parse::<f64>()
                .map(Value::Num)
                .map_err(|_| format!("bad number '{}'", s))
        }

        fn boolean(&mut self) -> Result<Value, String> {
            if self.lit("true") {
                Ok(Value::Bool(true))
            } else if self.lit("false") {
                Ok(Value::Bool(false))
            } else {
                Err("invalid literal".into())
            }
        }

        fn null(&mut self) -> Result<Value, String> {
            if self.lit("null") {
                Ok(Value::Null)
            } else {
                Err("invalid literal".into())
            }
        }
    }
}

// ===========================================================================
// http — a minimal blocking HTTP/1.1 client over std::net::TcpStream.
// ===========================================================================

/// A minimal, hand-rolled blocking HTTP/1.1 client (no `reqwest`).
pub mod http {
    use super::Error;
    use std::io::{BufRead, BufReader, Write};
    use std::net::TcpStream;

    /// A parsed absolute URL (`scheme://host[:port]/path?query`).
    #[derive(Debug, Clone, PartialEq)]
    pub struct Url {
        pub scheme: String,
        pub host: String,
        pub port: u16,
        /// Path plus query, e.g. `/v1/skills?x=1`. Always begins with `/`.
        pub path: String,
    }

    impl Url {
        /// Parse an absolute URL. Understands `http`/`https` default ports and
        /// bracketed IPv6 hosts.
        pub fn parse(s: &str) -> Result<Url, Error> {
            let (scheme, rest) = s
                .split_once("://")
                .ok_or_else(|| Error::Config(format!("invalid url (no scheme): {}", s)))?;
            let scheme = scheme.to_ascii_lowercase();

            let split = rest.find(['/', '?']).unwrap_or(rest.len());
            let authority = &rest[..split];
            let mut path = rest[split..].to_string();
            if path.is_empty() {
                path = "/".to_string();
            }

            let default_port: u16 = if scheme == "https" { 443 } else { 80 };
            let (host, port) = parse_authority(authority, default_port)?;
            if host.is_empty() {
                return Err(Error::Config(format!("invalid url (no host): {}", s)));
            }
            Ok(Url {
                scheme,
                host,
                port,
                path,
            })
        }

        /// The value for the `Host:` header (adds `:port` only when non-default).
        fn host_header(&self) -> String {
            let default = if self.scheme == "https" { 443 } else { 80 };
            let hostpart = if self.host.contains(':') {
                format!("[{}]", self.host)
            } else {
                self.host.clone()
            };
            if self.port == default {
                hostpart
            } else {
                format!("{}:{}", hostpart, self.port)
            }
        }
    }

    fn parse_authority(authority: &str, default_port: u16) -> Result<(String, u16), Error> {
        if let Some(rest) = authority.strip_prefix('[') {
            // IPv6 literal: [host] or [host]:port
            let end = rest
                .find(']')
                .ok_or_else(|| Error::Config("invalid ipv6 authority".into()))?;
            let host = rest[..end].to_string();
            let after = &rest[end + 1..];
            let port = if let Some(p) = after.strip_prefix(':') {
                p.parse::<u16>()
                    .map_err(|_| Error::Config(format!("invalid port '{}'", p)))?
            } else {
                default_port
            };
            return Ok((host, port));
        }
        match authority.rsplit_once(':') {
            Some((h, p)) => {
                let port = p
                    .parse::<u16>()
                    .map_err(|_| Error::Config(format!("invalid port '{}'", p)))?;
                Ok((h.to_string(), port))
            }
            None => Ok((authority.to_string(), default_port)),
        }
    }

    /// An HTTP response: status code, headers, and the raw body bytes.
    #[derive(Debug, Clone)]
    pub struct Response {
        pub status: u16,
        pub headers: Vec<(String, String)>,
        pub body: Vec<u8>,
    }

    impl Response {
        pub fn header(&self, name: &str) -> Option<&str> {
            self.headers
                .iter()
                .find(|(k, _)| k.eq_ignore_ascii_case(name))
                .map(|(_, v)| v.as_str())
        }
    }

    /// Serialize and send one request over a fresh `Connection: close` socket,
    /// then read and parse the whole response.
    pub fn send(
        url: &Url,
        method: &str,
        headers: &[(String, String)],
        body: Option<&[u8]>,
    ) -> Result<Response, Error> {
        let addr = format!("{}:{}", url.host, url.port);
        let mut stream = TcpStream::connect(&addr)
            .map_err(|e| Error::Transport(format!("connect {}: {}", addr, e)))?;

        let mut req: Vec<u8> = Vec::new();
        write!(req, "{} {} HTTP/1.1\r\n", method, url.path).unwrap();
        write!(req, "Host: {}\r\n", url.host_header()).unwrap();
        for (k, v) in headers {
            write!(req, "{}: {}\r\n", k, v).unwrap();
        }
        if let Some(b) = body {
            write!(req, "Content-Length: {}\r\n", b.len()).unwrap();
        }
        req.extend_from_slice(b"Connection: close\r\n\r\n");
        if let Some(b) = body {
            req.extend_from_slice(b);
        }

        stream
            .write_all(&req)
            .map_err(|e| Error::Transport(format!("write: {}", e)))?;
        stream.flush().ok();

        let mut reader = BufReader::new(&mut stream);
        let (status_line, resp_headers, resp_body) =
            read_message(&mut reader, true).map_err(|e| Error::Transport(format!("read: {}", e)))?;
        let status = parse_status(&status_line)?;
        Ok(Response {
            status,
            headers: resp_headers,
            body: resp_body,
        })
    }

    fn parse_status(line: &str) -> Result<u16, Error> {
        line.split_whitespace()
            .nth(1)
            .and_then(|s| s.parse::<u16>().ok())
            .ok_or_else(|| Error::Transport(format!("bad status line: {:?}", line)))
    }

    /// Read a single HTTP message (request or response) from a buffered reader:
    /// the first line, the headers, and the body.
    ///
    /// The body is taken from `Content-Length` when present. When absent,
    /// `read_body_to_eof` decides: `true` (a response over a `close` socket)
    /// reads to EOF; `false` (a request whose sender keeps the socket open for
    /// the reply) treats it as an empty body — avoiding a read deadlock.
    pub fn read_message<R: BufRead>(
        r: &mut R,
        read_body_to_eof: bool,
    ) -> std::io::Result<(String, Vec<(String, String)>, Vec<u8>)> {
        let mut first = String::new();
        r.read_line(&mut first)?;
        let first = first.trim_end().to_string();

        let mut headers: Vec<(String, String)> = Vec::new();
        loop {
            let mut line = String::new();
            let n = r.read_line(&mut line)?;
            if n == 0 {
                break; // EOF
            }
            let t = line.trim_end_matches(['\r', '\n']);
            if t.is_empty() {
                break; // end of headers
            }
            if let Some((k, v)) = t.split_once(':') {
                headers.push((k.trim().to_string(), v.trim().to_string()));
            }
        }

        let content_length = headers
            .iter()
            .find(|(k, _)| k.eq_ignore_ascii_case("content-length"))
            .and_then(|(_, v)| v.trim().parse::<usize>().ok());

        let body = match content_length {
            Some(n) => {
                let mut b = vec![0u8; n];
                r.read_exact(&mut b)?;
                b
            }
            None if read_body_to_eof => {
                let mut b = Vec::new();
                r.read_to_end(&mut b)?;
                b
            }
            None => Vec::new(),
        };
        Ok((first, headers, body))
    }
}

// ===========================================================================
// error — typed error surface.
// ===========================================================================

/// Every failure the SDK can surface.
#[derive(Debug)]
pub enum Error {
    /// A non-2xx response, decoded from the canonical
    /// `{"error":{"code","message","request_id"}}` envelope (status/request_id
    /// backfilled from the HTTP status line / `X-Request-Id` header when the
    /// envelope omits them).
    Api {
        code: String,
        message: String,
        status: u16,
        request_id: String,
    },
    /// Refused to attach a credential to plaintext http bound for a non-loopback
    /// host. Nothing was sent. Use `https`, target a loopback host, or set
    /// `allow_insecure_http`.
    InsecureTransport,
    /// A transport-level failure (connect/read/write).
    Transport(String),
    /// A 2xx body that could not be decoded.
    Decode(String),
    /// Invalid client configuration (e.g. empty/invalid base URL).
    Config(String),
}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Error::Api {
                code,
                message,
                status,
                request_id,
            } => {
                if request_id.is_empty() {
                    write!(f, "thready: {} ({}): {}", code, status, message)
                } else {
                    write!(
                        f,
                        "thready: {} ({}): {} [request_id={}]",
                        code, status, message, request_id
                    )
                }
            }
            Error::InsecureTransport => write!(
                f,
                "thready: refusing to send credentials over plaintext http to a \
                 non-loopback host; use https or set allow_insecure_http"
            ),
            Error::Transport(m) => write!(f, "thready: transport: {}", m),
            Error::Decode(m) => write!(f, "thready: decode: {}", m),
            Error::Config(m) => write!(f, "thready: config: {}", m),
        }
    }
}

impl std::error::Error for Error {}

/// Map an HTTP status to a canonical code for the fallback path (a non-envelope
/// error body). Mirrors the gateway/Go-SDK taxonomy.
fn code_for_status(status: u16) -> String {
    let s = match status {
        400 => "invalid_argument",
        401 => "unauthenticated",
        403 => "permission_denied",
        404 => "not_found",
        409 => "conflict",
        412 => "failed_precondition",
        422 => "unprocessable",
        429 => "rate_limited",
        503 => "unavailable",
        504 => "deadline_exceeded",
        _ => "internal",
    };
    s.to_string()
}

fn status_text(status: u16) -> String {
    let s = match status {
        200 => "OK",
        202 => "Accepted",
        204 => "No Content",
        400 => "Bad Request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        409 => "Conflict",
        429 => "Too Many Requests",
        500 => "Internal Server Error",
        503 => "Service Unavailable",
        _ => "Error",
    };
    s.to_string()
}

// ===========================================================================
// types — typed request/response DTOs mirroring the /v1 wire shapes.
// ===========================================================================

use json::Value;

fn str_of(v: &Value, k: &str) -> String {
    v.get(k).and_then(|x| x.as_str()).unwrap_or("").to_string()
}
fn i64_of(v: &Value, k: &str) -> i64 {
    v.get(k).and_then(|x| x.as_f64()).map(|f| f as i64).unwrap_or(0)
}
fn f64_of(v: &Value, k: &str) -> f64 {
    v.get(k).and_then(|x| x.as_f64()).unwrap_or(0.0)
}
fn bool_of(v: &Value, k: &str) -> bool {
    v.get(k).and_then(|x| x.as_bool()).unwrap_or(false)
}
fn optstr_of(v: &Value, k: &str) -> Option<String> {
    v.get(k).and_then(|x| x.as_str()).map(|s| s.to_string())
}
fn strvec_of(v: &Value, k: &str) -> Vec<String> {
    v.get(k)
        .and_then(|x| x.as_array())
        .map(|a| {
            a.iter()
                .filter_map(|e| e.as_str().map(|s| s.to_string()))
                .collect()
        })
        .unwrap_or_default()
}

/// Credential body for `POST /v1/auth/login`. `totp` is required for admin tiers.
#[derive(Debug, Clone)]
pub struct LoginRequest {
    pub email: String,
    pub password: String,
    pub totp: Option<String>,
}

impl LoginRequest {
    pub fn new(email: &str, password: &str) -> Self {
        LoginRequest {
            email: email.to_string(),
            password: password.to_string(),
            totp: None,
        }
    }
    fn to_value(&self) -> Value {
        let mut o = vec![
            ("email".to_string(), Value::Str(self.email.clone())),
            ("password".to_string(), Value::Str(self.password.clone())),
        ];
        if let Some(t) = &self.totp {
            o.push(("totp".to_string(), Value::Str(t.clone())));
        }
        Value::Object(o)
    }
}

/// Login/refresh success body.
#[derive(Debug, Clone, PartialEq)]
pub struct TokenPair {
    pub access_token: String,
    pub refresh_token: String,
    pub token_type: String,
    pub expires_in: i64,
    pub refresh_expires_in: i64,
}

impl TokenPair {
    fn from_value(v: &Value) -> Self {
        TokenPair {
            access_token: str_of(v, "access_token"),
            refresh_token: str_of(v, "refresh_token"),
            token_type: str_of(v, "token_type"),
            expires_in: i64_of(v, "expires_in"),
            refresh_expires_in: i64_of(v, "refresh_expires_in"),
        }
    }
}

/// A registered messenger channel/group.
#[derive(Debug, Clone, PartialEq)]
pub struct Channel {
    pub id: String,
    pub account_id: String,
    pub name: String,
    pub platform: String,
    pub external_ref: String,
    pub created_at: String,
}

impl Channel {
    fn from_value(v: &Value) -> Self {
        Channel {
            id: str_of(v, "id"),
            account_id: str_of(v, "account_id"),
            name: str_of(v, "name"),
            platform: str_of(v, "platform"),
            external_ref: str_of(v, "external_ref"),
            created_at: str_of(v, "created_at"),
        }
    }
}

/// Create body for `POST /v1/channels`.
#[derive(Debug, Clone)]
pub struct CreateChannelRequest {
    pub name: String,
    pub platform: String,
    pub external_ref: String,
}

impl CreateChannelRequest {
    fn to_value(&self) -> Value {
        Value::Object(vec![
            ("name".to_string(), Value::Str(self.name.clone())),
            ("platform".to_string(), Value::Str(self.platform.clone())),
            (
                "external_ref".to_string(),
                Value::Str(self.external_ref.clone()),
            ),
        ])
    }
}

/// A channel post with its processing status.
#[derive(Debug, Clone, PartialEq)]
pub struct Post {
    pub id: String,
    pub channel_id: String,
    pub account_id: String,
    pub body: String,
    pub hashtags: Vec<String>,
    pub categories: Vec<String>,
    pub status: String,
    pub created_at: String,
}

impl Post {
    fn from_value(v: &Value) -> Self {
        Post {
            id: str_of(v, "id"),
            channel_id: str_of(v, "channel_id"),
            account_id: str_of(v, "account_id"),
            body: str_of(v, "body"),
            hashtags: strvec_of(v, "hashtags"),
            categories: strvec_of(v, "categories"),
            status: str_of(v, "status"),
            created_at: str_of(v, "created_at"),
        }
    }
}

/// The async (re)processing job returned (`202 Accepted`) by `reprocess`.
#[derive(Debug, Clone, PartialEq)]
pub struct Job {
    pub job_id: String,
    pub post_id: String,
    pub status: String,
    pub precedence: Vec<String>,
    pub queued_at: String,
}

impl Job {
    fn from_value(v: &Value) -> Self {
        Job {
            job_id: str_of(v, "job_id"),
            post_id: str_of(v, "post_id"),
            status: str_of(v, "status"),
            precedence: strvec_of(v, "precedence"),
            queued_at: str_of(v, "queued_at"),
        }
    }
}

/// Body for `POST /v1/search`. `mode` is `semantic|keyword|hybrid`; `sources`
/// selects the corpora (`posts|generated|assets`).
#[derive(Debug, Clone)]
pub struct SearchRequest {
    pub query: String,
    pub mode: Option<String>,
    pub sources: Vec<String>,
    pub top_k: Option<i64>,
    pub rerank: bool,
}

impl SearchRequest {
    pub fn new(query: &str) -> Self {
        SearchRequest {
            query: query.to_string(),
            mode: None,
            sources: Vec::new(),
            top_k: None,
            rerank: false,
        }
    }
    fn to_value(&self) -> Value {
        let mut o = vec![("query".to_string(), Value::Str(self.query.clone()))];
        if let Some(m) = &self.mode {
            o.push(("mode".to_string(), Value::Str(m.clone())));
        }
        if !self.sources.is_empty() {
            o.push((
                "sources".to_string(),
                Value::Array(self.sources.iter().map(|s| Value::Str(s.clone())).collect()),
            ));
        }
        if let Some(k) = self.top_k {
            o.push(("top_k".to_string(), Value::Num(k as f64)));
        }
        o.push(("rerank".to_string(), Value::Bool(self.rerank)));
        Value::Object(o)
    }
}

/// A single ranked search result.
#[derive(Debug, Clone, PartialEq)]
pub struct SearchHit {
    pub source_id: String,
    pub kind: String,
    pub score: f64,
    pub span: Option<String>,
    pub snippet: String,
}

impl SearchHit {
    fn from_value(v: &Value) -> Self {
        SearchHit {
            source_id: str_of(v, "source_id"),
            kind: str_of(v, "kind"),
            score: f64_of(v, "score"),
            span: optstr_of(v, "span"),
            snippet: str_of(v, "snippet"),
        }
    }
}

/// The ranked result set plus provenance.
#[derive(Debug, Clone, PartialEq)]
pub struct SearchResults {
    pub results: Vec<SearchHit>,
    pub took_ms: i64,
    pub embedder: String,
}

impl SearchResults {
    fn from_value(v: &Value) -> Self {
        let results = v
            .get("results")
            .and_then(|x| x.as_array())
            .map(|a| a.iter().map(SearchHit::from_value).collect())
            .unwrap_or_default();
        SearchResults {
            results,
            took_ms: i64_of(v, "took_ms"),
            embedder: str_of(v, "embedder"),
        }
    }
}

/// A knowledge unit in the Skill-Graph DAG.
#[derive(Debug, Clone, PartialEq)]
pub struct Skill {
    pub id: String,
    pub name: String,
    pub kind: String,
    pub sort_order: i64,
}

impl Skill {
    fn from_value(v: &Value) -> Self {
        Skill {
            id: str_of(v, "id"),
            name: str_of(v, "name"),
            kind: str_of(v, "kind"),
            sort_order: i64_of(v, "sort_order"),
        }
    }
}

// ===========================================================================
// client — the typed ThreadyClient.
// ===========================================================================

const DEFAULT_MAX_RETRIES: u32 = 3;
const BACKOFF_BASE_MS: u64 = 25;
const BACKOFF_MAX_MS: u64 = 2000;

/// A monotonically-incrementing counter combined with a nanosecond clock read
/// to mint a **unique** `Idempotency-Key` per unsafe POST. (This is a unique
/// key, **not** a UUID — `std` ships no UUID and no crates are allowed.)
static IDEM_COUNTER: AtomicU64 = AtomicU64::new(0);

fn new_idempotency_key() -> String {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_nanos())
        .unwrap_or(0);
    let c = IDEM_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("idem-{:x}-{:x}", nanos, c)
}

/// Percent-encode a single path segment (RFC 3986 unreserved kept verbatim).
fn path_escape(seg: &str) -> String {
    let mut out = String::with_capacity(seg.len());
    for b in seg.bytes() {
        match b {
            b'A'..=b'Z' | b'a'..=b'z' | b'0'..=b'9' | b'-' | b'_' | b'.' | b'~' => {
                out.push(b as char)
            }
            _ => out.push_str(&format!("%{:02X}", b)),
        }
    }
    out
}

/// A typed, std-only client for the Thready `/v1` API.
///
/// Construct it with [`ThreadyClient::new`]. Exactly one of `access_token` /
/// `api_key` is normally set; if both are present the bearer token wins. A
/// successful [`login`](ThreadyClient::login) updates the in-flight access token
/// so later calls authenticate automatically.
pub struct ThreadyClient {
    base_url: String,
    access_token: Mutex<String>,
    api_key: String,
    allow_insecure_http: bool,
    max_retries: u32,
    backoff_base_ms: u64,
    backoff_max_ms: u64,
}

impl ThreadyClient {
    /// Build a client. `base_url` is the gateway origin (trailing slash trimmed),
    /// e.g. `http://127.0.0.1:8080` or `https://thready.hxd3v.com`. Returns
    /// [`Error::Config`] if `base_url` is empty.
    pub fn new(
        base_url: &str,
        access_token: &str,
        api_key: &str,
        allow_insecure_http: bool,
    ) -> Result<Self, Error> {
        let base = base_url.trim().trim_end_matches('/').to_string();
        if base.is_empty() {
            return Err(Error::Config("base_url is required".into()));
        }
        Ok(ThreadyClient {
            base_url: base,
            access_token: Mutex::new(access_token.to_string()),
            api_key: api_key.to_string(),
            allow_insecure_http,
            max_retries: DEFAULT_MAX_RETRIES,
            backoff_base_ms: BACKOFF_BASE_MS,
            backoff_max_ms: BACKOFF_MAX_MS,
        })
    }

    /// The token the client currently authenticates with.
    pub fn access_token(&self) -> String {
        self.access_token.lock().unwrap().clone()
    }

    // ---- auth + transport policy ----------------------------------------

    /// Inject the credential (bearer JWT wins over `X-API-Key`). Enforces the
    /// transport policy *first*: when a credential is present and the request is
    /// plaintext http to a non-loopback host, it attaches **no header** and
    /// returns [`Error::InsecureTransport`] — before any bytes are sent.
    fn apply_auth(
        &self,
        headers: &mut Vec<(String, String)>,
        url: &http::Url,
    ) -> Result<(), Error> {
        let tok = self.access_token();
        let has_credential = !tok.is_empty() || !self.api_key.is_empty();
        if has_credential && !self.transport_allowed_url(url) {
            return Err(Error::InsecureTransport);
        }
        if !tok.is_empty() {
            headers.push(("Authorization".to_string(), format!("Bearer {}", tok)));
        } else if !self.api_key.is_empty() {
            headers.push(("X-API-Key".to_string(), self.api_key.clone()));
        }
        Ok(())
    }

    /// Whether it is safe to attach a credential to a request bound for
    /// `full_url`. `https` (any host) is always safe; plaintext `http` is safe
    /// only to a loopback host — or unconditionally when `allow_insecure_http`.
    ///
    /// Note the deliberate asymmetry the task calls out: an **`https` base URL
    /// is never refused**, even though this std-only build cannot actually speak
    /// TLS. The guard refuses *plaintext leakage*, not `https`.
    pub fn transport_allowed(&self, full_url: &str) -> bool {
        match http::Url::parse(full_url) {
            Ok(u) => self.transport_allowed_url(&u),
            Err(_) => true,
        }
    }

    fn transport_allowed_url(&self, u: &http::Url) -> bool {
        if self.allow_insecure_http {
            return true;
        }
        if u.scheme != "http" {
            return true; // https (and any non-plaintext scheme) is safe
        }
        is_loopback_host(&u.host)
    }

    fn backoff(&self, attempt: u32) {
        let shifted = self
            .backoff_base_ms
            .checked_shl(attempt.saturating_sub(1))
            .unwrap_or(self.backoff_max_ms);
        let d = shifted.min(self.backoff_max_ms).max(1);
        std::thread::sleep(Duration::from_millis(d));
    }

    // ---- core request path ----------------------------------------------

    /// Perform a request: encode the optional body, inject headers + auth,
    /// (for idempotent GETs) retry `503`/`429` with capped backoff, and decode
    /// the response into a [`json::Value`] (or a typed [`Error`]).
    fn do_request(
        &self,
        method: &str,
        path: &str,
        body: Option<Value>,
        idempotency_key: Option<String>,
    ) -> Result<Value, Error> {
        let full = format!("{}{}", self.base_url, path);
        let url = http::Url::parse(&full)?;
        let body_bytes: Option<Vec<u8>> = body.as_ref().map(|v| json::encode(v).into_bytes());

        let is_get = method.eq_ignore_ascii_case("GET");
        let attempts = if is_get { self.max_retries + 1 } else { 1 };

        let mut last_err: Option<Error> = None;
        for attempt in 0..attempts {
            if attempt > 0 {
                self.backoff(attempt);
            }

            let mut headers: Vec<(String, String)> = Vec::new();
            headers.push(("Accept".to_string(), "application/json".to_string()));
            if body_bytes.is_some() {
                headers.push((
                    "Content-Type".to_string(),
                    "application/json".to_string(),
                ));
            }
            if let Some(k) = &idempotency_key {
                headers.push(("Idempotency-Key".to_string(), k.clone()));
            }
            // Enforce the credential-transport policy BEFORE any send: a refusal
            // here means no header was attached and no request left the process.
            self.apply_auth(&mut headers, &url)?;

            match http::send(&url, method, &headers, body_bytes.as_deref()) {
                Ok(resp) => {
                    if is_get
                        && attempt < attempts - 1
                        && (resp.status == 503 || resp.status == 429)
                    {
                        last_err = Some(parse_api_error(&resp));
                        continue; // retry idempotent GET on transient upstream
                    }
                    return decode_response(resp);
                }
                Err(Error::InsecureTransport) => return Err(Error::InsecureTransport),
                Err(e) => {
                    last_err = Some(e);
                    if is_get && attempt < attempts - 1 {
                        continue; // transient transport error on an idempotent GET
                    }
                    return Err(last_err.unwrap());
                }
            }
        }
        Err(last_err.unwrap_or_else(|| Error::Transport("request failed".into())))
    }

    // ---- typed methods ---------------------------------------------------

    /// `POST /v1/auth/login`. Stores the returned access token on the client so
    /// subsequent calls authenticate automatically.
    pub fn login(&self, req: &LoginRequest) -> Result<TokenPair, Error> {
        let v = self.do_request("POST", "/v1/auth/login", Some(req.to_value()), None)?;
        let tp = TokenPair::from_value(&v);
        if !tp.access_token.is_empty() {
            *self.access_token.lock().unwrap() = tp.access_token.clone();
        }
        Ok(tp)
    }

    /// `GET /v1/channels`.
    pub fn list_channels(&self) -> Result<Vec<Channel>, Error> {
        let v = self.do_request("GET", "/v1/channels", None, None)?;
        Ok(data_array(&v, Channel::from_value))
    }

    /// `POST /v1/channels` (unsafe → carries an `Idempotency-Key`).
    pub fn create_channel(&self, req: &CreateChannelRequest) -> Result<Channel, Error> {
        let key = new_idempotency_key();
        let v = self.do_request("POST", "/v1/channels", Some(req.to_value()), Some(key))?;
        Ok(Channel::from_value(&v))
    }

    /// `GET /v1/posts/{post_id}`.
    pub fn get_post(&self, post_id: &str) -> Result<Post, Error> {
        let path = format!("/v1/posts/{}", path_escape(post_id));
        let v = self.do_request("GET", &path, None, None)?;
        Ok(Post::from_value(&v))
    }

    /// `POST /v1/posts/{post_id}/reprocess` (unsafe → carries an
    /// `Idempotency-Key`). Returns the queued [`Job`] (`202 Accepted`).
    pub fn reprocess(&self, post_id: &str) -> Result<Job, Error> {
        let key = new_idempotency_key();
        let path = format!("/v1/posts/{}/reprocess", path_escape(post_id));
        let v = self.do_request("POST", &path, None, Some(key))?;
        Ok(Job::from_value(&v))
    }

    /// `POST /v1/search`.
    pub fn search(&self, req: &SearchRequest) -> Result<SearchResults, Error> {
        let v = self.do_request("POST", "/v1/search", Some(req.to_value()), None)?;
        Ok(SearchResults::from_value(&v))
    }

    /// `GET /v1/skills`.
    pub fn list_skills(&self) -> Result<Vec<Skill>, Error> {
        let v = self.do_request("GET", "/v1/skills", None, None)?;
        Ok(data_array(&v, Skill::from_value))
    }
}

/// Decode the standard `{"data":[...]}` collection wrapper into a typed vec.
fn data_array<T>(v: &Value, f: fn(&Value) -> T) -> Vec<T> {
    v.get("data")
        .and_then(|d| d.as_array())
        .map(|a| a.iter().map(f).collect())
        .unwrap_or_default()
}

/// Whether `host` refers to the local machine: literal `localhost`, or any
/// loopback IP (`127.0.0.0/8`, `::1`).
fn is_loopback_host(host: &str) -> bool {
    if host.eq_ignore_ascii_case("localhost") {
        return true;
    }
    if let Ok(ip) = host.parse::<std::net::IpAddr>() {
        return ip.is_loopback();
    }
    false
}

/// A 2xx body decodes into a [`json::Value`] (204/empty → `Null`); anything else
/// becomes a typed [`Error::Api`].
fn decode_response(resp: http::Response) -> Result<Value, Error> {
    if (200..300).contains(&resp.status) {
        if resp.status == 204 || resp.body.is_empty() {
            return Ok(Value::Null);
        }
        let text = String::from_utf8_lossy(&resp.body);
        json::parse(&text).map_err(Error::Decode)
    } else {
        Err(parse_api_error(&resp))
    }
}

/// Map a non-2xx response to a typed [`Error::Api`]. Prefers the canonical
/// `{"error":{code,message,status,request_id}}` envelope, backfilling missing
/// status/request_id from the status line / `X-Request-Id` header; degrades to a
/// status-derived error for a non-envelope body.
fn parse_api_error(resp: &http::Response) -> Error {
    let request_id_hdr = resp.header("x-request-id").unwrap_or("").to_string();
    let text = String::from_utf8_lossy(&resp.body);

    if let Ok(v) = json::parse(&text) {
        if let Some(err) = v.get("error") {
            let code = err.get("code").and_then(|x| x.as_str()).unwrap_or("");
            if !code.is_empty() {
                let message = err
                    .get("message")
                    .and_then(|x| x.as_str())
                    .unwrap_or("")
                    .to_string();
                let status = err
                    .get("status")
                    .and_then(|x| x.as_f64())
                    .map(|f| f as u16)
                    .filter(|s| *s != 0)
                    .unwrap_or(resp.status);
                let request_id = err
                    .get("request_id")
                    .and_then(|x| x.as_str())
                    .map(|s| s.to_string())
                    .filter(|s| !s.is_empty())
                    .unwrap_or(request_id_hdr);
                return Error::Api {
                    code: code.to_string(),
                    message,
                    status,
                    request_id,
                };
            }
        }
    }

    let msg = text.trim();
    Error::Api {
        code: code_for_status(resp.status),
        message: if msg.is_empty() {
            status_text(resp.status)
        } else {
            msg.to_string()
        },
        status: resp.status,
        request_id: request_id_hdr,
    }
}

// ===========================================================================
// tests — TDD against a std::net::TcpListener mock /v1 server on port 0.
// ===========================================================================

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{BufReader, Write};
    use std::net::TcpListener;
    use std::sync::{Arc, Mutex};
    use std::thread;

    /// One request the mock server captured.
    #[derive(Clone)]
    struct Recorded {
        line: String,
        headers: Vec<(String, String)>,
        body: Vec<u8>,
    }

    impl Recorded {
        fn method(&self) -> &str {
            self.line.split_whitespace().next().unwrap_or("")
        }
        fn path(&self) -> &str {
            self.line.split_whitespace().nth(1).unwrap_or("")
        }
        fn header(&self, name: &str) -> Option<&str> {
            self.headers
                .iter()
                .find(|(k, _)| k.eq_ignore_ascii_case(name))
                .map(|(_, v)| v.as_str())
        }
        fn body_json(&self) -> json::Value {
            json::parse(&String::from_utf8_lossy(&self.body)).expect("request body is valid JSON")
        }
    }

    struct Mock {
        port: u16,
        requests: Arc<Mutex<Vec<Recorded>>>,
    }

    impl Mock {
        fn base_url(&self) -> String {
            format!("http://127.0.0.1:{}", self.port)
        }
        fn recorded(&self) -> Vec<Recorded> {
            self.requests.lock().unwrap().clone()
        }
    }

    /// Build a canned HTTP/1.1 response string.
    fn resp(status: u16, reason: &str, body: &str) -> String {
        format!(
            "HTTP/1.1 {} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            status,
            reason,
            body.len(),
            body
        )
    }

    /// Start a mock `/v1` server on `127.0.0.1:0`. Each incoming connection pops
    /// the next canned response (the last one repeats once exhausted), records
    /// the request line/headers/body, and writes the response.
    fn start_mock(responses: Vec<String>) -> Mock {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind loopback:0");
        let port = listener.local_addr().unwrap().port();
        let requests = Arc::new(Mutex::new(Vec::<Recorded>::new()));
        let reqs = requests.clone();

        thread::spawn(move || {
            let mut idx = 0usize;
            for stream in listener.incoming() {
                let mut stream = match stream {
                    Ok(s) => s,
                    Err(_) => break,
                };
                let parsed = {
                    let mut reader = BufReader::new(&mut stream);
                    // read_body_to_eof=false: a request keeps the socket open for
                    // its reply, so bodies come strictly from Content-Length.
                    http::read_message(&mut reader, false)
                };
                if let Ok((line, headers, body)) = parsed {
                    reqs.lock().unwrap().push(Recorded {
                        line,
                        headers,
                        body,
                    });
                }
                let out = if idx < responses.len() {
                    responses[idx].clone()
                } else {
                    responses.last().cloned().unwrap_or_default()
                };
                idx += 1;
                let _ = stream.write_all(out.as_bytes());
                let _ = stream.flush();
                // stream drops here -> Connection: close honored.
            }
        });

        Mock { port, requests }
    }

    // ---- method/path/headers + typed decode -----------------------------

    #[test]
    fn test_login_sends_credentials_stores_token() {
        let body = r#"{"access_token":"jwt-abc","refresh_token":"r-xyz","token_type":"Bearer","expires_in":3600,"refresh_expires_in":7200}"#;
        let mock = start_mock(vec![resp(200, "OK", body)]);
        let client = ThreadyClient::new(&mock.base_url(), "", "", false).unwrap();

        let mut req = LoginRequest::new("user@thready.test", "pw-123");
        req.totp = Some("123456".to_string());
        let tp = client.login(&req).expect("login ok");

        assert_eq!(tp.access_token, "jwt-abc");
        assert_eq!(tp.expires_in, 3600);
        // Token stored on the client for later calls.
        assert_eq!(client.access_token(), "jwt-abc");

        let rec = &mock.recorded()[0];
        assert_eq!(rec.method(), "POST");
        assert_eq!(rec.path(), "/v1/auth/login");
        assert_eq!(rec.header("Accept"), Some("application/json"));
        assert_eq!(rec.header("Content-Type"), Some("application/json"));
        let jb = rec.body_json();
        assert_eq!(jb.get("email").unwrap().as_str(), Some("user@thready.test"));
        assert_eq!(jb.get("password").unwrap().as_str(), Some("pw-123"));
        assert_eq!(jb.get("totp").unwrap().as_str(), Some("123456"));
    }

    #[test]
    fn test_list_channels_injects_bearer_and_decodes() {
        let body = r#"{"data":[{"id":"c1","account_id":"a1","name":"Chan","platform":"telegram","external_ref":"@x","created_at":"2026-01-01T00:00:00Z"}],"meta":{}}"#;
        let mock = start_mock(vec![resp(200, "OK", body)]);
        let client = ThreadyClient::new(&mock.base_url(), "tok-123", "", false).unwrap();

        let channels = client.list_channels().expect("list ok");
        assert_eq!(channels.len(), 1);
        assert_eq!(channels[0].id, "c1");
        assert_eq!(channels[0].platform, "telegram");

        let rec = &mock.recorded()[0];
        assert_eq!(rec.method(), "GET");
        assert_eq!(rec.path(), "/v1/channels");
        assert_eq!(rec.header("Authorization"), Some("Bearer tok-123"));
    }

    #[test]
    fn test_create_channel_sends_idempotency_key_and_body() {
        let body = r#"{"id":"c9","account_id":"a1","name":"New","platform":"max","external_ref":"grp-1","created_at":"2026-02-02T00:00:00Z"}"#;
        let mock = start_mock(vec![resp(201, "Created", body)]);
        // API-key auth (no bearer) exercises the X-API-Key branch over loopback.
        let client = ThreadyClient::new(&mock.base_url(), "", "sk-key", false).unwrap();

        let req = CreateChannelRequest {
            name: "New".into(),
            platform: "max".into(),
            external_ref: "grp-1".into(),
        };
        let ch = client.create_channel(&req).expect("create ok");
        assert_eq!(ch.id, "c9");
        assert_eq!(ch.platform, "max");

        let rec = &mock.recorded()[0];
        assert_eq!(rec.method(), "POST");
        assert_eq!(rec.path(), "/v1/channels");
        // Idempotency-Key present and non-empty on the unsafe POST.
        let idem = rec.header("Idempotency-Key").expect("Idempotency-Key present");
        assert!(!idem.is_empty(), "Idempotency-Key must be non-empty");
        // API-key auth injected (no bearer set).
        assert_eq!(rec.header("X-API-Key"), Some("sk-key"));
        assert_eq!(rec.header("Authorization"), None);
        let jb = rec.body_json();
        assert_eq!(jb.get("name").unwrap().as_str(), Some("New"));
        assert_eq!(jb.get("external_ref").unwrap().as_str(), Some("grp-1"));
    }

    #[test]
    fn test_get_post_decodes_typed_post() {
        let body = r##"{"id":"p1","channel_id":"c1","account_id":"a1","body":"hi","hashtags":["#a","#b"],"categories":["news"],"status":"processed","created_at":"2026-03-03T00:00:00Z"}"##;
        let mock = start_mock(vec![resp(200, "OK", body)]);
        let client = ThreadyClient::new(&mock.base_url(), "tok", "", false).unwrap();

        let post = client.get_post("p1").expect("get_post ok");
        assert_eq!(post.id, "p1");
        assert_eq!(post.hashtags, vec!["#a".to_string(), "#b".to_string()]);
        assert_eq!(post.status, "processed");

        let rec = &mock.recorded()[0];
        assert_eq!(rec.method(), "GET");
        assert_eq!(rec.path(), "/v1/posts/p1");
    }

    #[test]
    fn test_reprocess_returns_job_with_idempotency_key() {
        let body = r#"{"job_id":"j1","post_id":"p1","status":"queued","precedence":["download","analyze","reply"],"queued_at":"2026-04-04T00:00:00Z"}"#;
        let mock = start_mock(vec![resp(202, "Accepted", body)]);
        let client = ThreadyClient::new(&mock.base_url(), "tok", "", false).unwrap();

        let job = client.reprocess("p1").expect("reprocess ok");
        assert_eq!(job.job_id, "j1");
        assert_eq!(job.status, "queued");
        assert_eq!(job.precedence.len(), 3);

        let rec = &mock.recorded()[0];
        assert_eq!(rec.method(), "POST");
        assert_eq!(rec.path(), "/v1/posts/p1/reprocess");
        let idem = rec.header("Idempotency-Key").expect("Idempotency-Key present");
        assert!(!idem.is_empty());
    }

    #[test]
    fn test_search_sends_body_and_decodes_results() {
        let body = r#"{"results":[{"source_id":"p1","kind":"post","score":0.91,"span":null,"snippet":"hello"}],"took_ms":12,"embedder":"e5-large"}"#;
        let mock = start_mock(vec![resp(200, "OK", body)]);
        let client = ThreadyClient::new(&mock.base_url(), "tok", "", false).unwrap();

        let mut req = SearchRequest::new("hello");
        req.mode = Some("hybrid".into());
        req.sources = vec!["posts".into(), "generated".into()];
        req.top_k = Some(5);
        req.rerank = true;

        let res = client.search(&req).expect("search ok");
        assert_eq!(res.results.len(), 1);
        assert_eq!(res.results[0].source_id, "p1");
        assert!((res.results[0].score - 0.91).abs() < 1e-9);
        assert_eq!(res.results[0].span, None);
        assert_eq!(res.embedder, "e5-large");

        let rec = &mock.recorded()[0];
        assert_eq!(rec.method(), "POST");
        assert_eq!(rec.path(), "/v1/search");
        let jb = rec.body_json();
        assert_eq!(jb.get("query").unwrap().as_str(), Some("hello"));
        assert_eq!(jb.get("mode").unwrap().as_str(), Some("hybrid"));
        assert_eq!(jb.get("top_k").unwrap().as_f64(), Some(5.0));
        assert_eq!(jb.get("rerank").unwrap().as_bool(), Some(true));
    }

    #[test]
    fn test_list_skills_decodes_envelope() {
        let body = r#"{"data":[{"id":"s1","name":"download","kind":"action","sort_order":1},{"id":"s2","name":"analyze","kind":"action","sort_order":3}],"meta":{}}"#;
        let mock = start_mock(vec![resp(200, "OK", body)]);
        let client = ThreadyClient::new(&mock.base_url(), "tok", "", false).unwrap();

        let skills = client.list_skills().expect("list_skills ok");
        assert_eq!(skills.len(), 2);
        assert_eq!(skills[0].name, "download");
        assert_eq!(skills[1].sort_order, 3);

        let rec = &mock.recorded()[0];
        assert_eq!(rec.method(), "GET");
        assert_eq!(rec.path(), "/v1/skills");
    }

    #[test]
    fn test_bearer_wins_over_api_key() {
        let mock = start_mock(vec![resp(200, "OK", r#"{"data":[],"meta":{}}"#)]);
        let client = ThreadyClient::new(&mock.base_url(), "the-jwt", "the-apikey", false).unwrap();
        client.list_channels().expect("ok");

        let rec = &mock.recorded()[0];
        assert_eq!(rec.header("Authorization"), Some("Bearer the-jwt"));
        assert_eq!(rec.header("X-API-Key"), None);
    }

    // ---- Api error mapping ----------------------------------------------

    #[test]
    fn test_non2xx_maps_to_typed_api_error() {
        let body = r#"{"error":{"code":"not_found","message":"post not found","request_id":"req-123"}}"#;
        let mock = start_mock(vec![resp(404, "Not Found", body)]);
        let client = ThreadyClient::new(&mock.base_url(), "tok", "", false).unwrap();

        let err = client.get_post("missing").expect_err("should be an error");
        match err {
            Error::Api {
                code,
                message,
                status,
                request_id,
            } => {
                assert_eq!(code, "not_found");
                assert_eq!(message, "post not found");
                assert_eq!(status, 404); // backfilled from the HTTP status line
                assert_eq!(request_id, "req-123");
            }
            other => panic!("expected Error::Api, got {:?}", other),
        }
    }

    // ---- retry ----------------------------------------------------------

    #[test]
    fn test_retry_get_503_then_200_two_connections() {
        let err_body = r#"{"error":{"code":"unavailable","message":"upstream down","request_id":"r1"}}"#;
        let ok_body = r#"{"data":[{"id":"s1","name":"download","kind":"action","sort_order":1}],"meta":{}}"#;
        // First connection => 503, second connection => 200.
        let mock = start_mock(vec![
            resp(503, "Service Unavailable", err_body),
            resp(200, "OK", ok_body),
        ]);
        let client = ThreadyClient::new(&mock.base_url(), "tok", "", false).unwrap();

        let skills = client.list_skills().expect("retry should succeed on 2nd attempt");
        assert_eq!(skills.len(), 1);

        // Exactly two connections: the 503 attempt + the successful retry.
        let count = mock.recorded().len();
        assert_eq!(count, 2, "expected exactly 2 connections, got {}", count);
    }

    #[test]
    fn test_post_not_retried_on_503() {
        let err_body = r#"{"error":{"code":"unavailable","message":"down","request_id":"r"}}"#;
        // Serve 503 on every connection; a POST must not retry.
        let mock = start_mock(vec![resp(503, "Service Unavailable", err_body)]);
        let client = ThreadyClient::new(&mock.base_url(), "tok", "", false).unwrap();

        let req = CreateChannelRequest {
            name: "n".into(),
            platform: "p".into(),
            external_ref: "e".into(),
        };
        let err = client.create_channel(&req).expect_err("should fail");
        assert!(matches!(err, Error::Api { status: 503, .. }));
        assert_eq!(mock.recorded().len(), 1, "POST must not be retried");
    }

    // ---- insecure-transport guard ---------------------------------------

    #[test]
    fn test_transport_allowed_matrix() {
        // Client with a credential and allow_insecure_http = false.
        let client = ThreadyClient::new("http://127.0.0.1:1", "tok", "", false).unwrap();
        assert!(
            !client.transport_allowed("http://198.51.100.9/v1/skills"),
            "plaintext http to a remote host must be refused"
        );
        assert!(
            client.transport_allowed("http://127.0.0.1:8080/v1/skills"),
            "plaintext http to a loopback IP is allowed"
        );
        assert!(
            client.transport_allowed("http://localhost/v1/skills"),
            "plaintext http to localhost is allowed"
        );
        // An https base_url + creds does NOT get refused (the key asymmetry).
        assert!(
            client.transport_allowed("https://remote.example.com/v1/skills"),
            "https is always allowed, even to a remote host"
        );

        // With allow_insecure_http = true, even remote http is allowed.
        let permissive =
            ThreadyClient::new("http://198.51.100.9", "tok", "", true).unwrap();
        assert!(permissive.transport_allowed("http://198.51.100.9/v1/skills"));
    }

    #[test]
    fn test_insecure_http_remote_refused_nothing_sent() {
        // Remote, unroutable http host (TEST-NET-2) + a credential. The guard
        // must fire BEFORE connecting. Getting Error::InsecureTransport (rather
        // than a Transport connect error) is itself proof that no bytes were
        // sent and no socket was opened: InsecureTransport is only returned on
        // the pre-connect guard path.
        let client =
            ThreadyClient::new("http://198.51.100.23/v1", "", "sk-secret", false).unwrap();
        let err = client.list_skills().expect_err("must refuse");
        assert!(
            matches!(err, Error::InsecureTransport),
            "expected InsecureTransport (nothing sent), got {:?}",
            err
        );
    }

    #[test]
    fn test_insecure_http_loopback_allowed_past_guard() {
        // http + 127.0.0.1 + credential: allowed past the guard; the request
        // actually reaches the mock server (which records it).
        let mock = start_mock(vec![resp(200, "OK", r#"{"data":[],"meta":{}}"#)]);
        let client = ThreadyClient::new(&mock.base_url(), "", "sk-secret", false).unwrap();
        client.list_skills().expect("loopback http must be allowed");
        assert_eq!(
            mock.recorded().len(),
            1,
            "the request must reach the server (past the guard)"
        );
        // And the credential was actually attached.
        assert_eq!(mock.recorded()[0].header("X-API-Key"), Some("sk-secret"));
    }

    #[test]
    fn test_https_base_url_not_refused() {
        // An https base_url with credentials must NOT be flagged
        // InsecureTransport. std has no TLS so we cannot actually connect; we
        // assert the guard decision via the transport_allowed helper.
        let client =
            ThreadyClient::new("https://remote.example.com", "tok", "", false).unwrap();
        assert!(
            client.transport_allowed("https://remote.example.com/v1/skills"),
            "https base_url + creds must be allowed (not InsecureTransport)"
        );
    }

    // ---- unit tests for the hand-rolled json + url layers ---------------

    #[test]
    fn test_json_roundtrip_nested() {
        let src = r#"{"a":1,"b":[true,null,"x\ny"],"c":{"d":-3.5,"e":"é"}}"#;
        let v = json::parse(src).expect("parse");
        assert_eq!(v.get("a").unwrap().as_f64(), Some(1.0));
        let b = v.get("b").unwrap().as_array().unwrap();
        assert_eq!(b[2].as_str(), Some("x\ny"));
        assert_eq!(v.get("c").unwrap().get("e").unwrap().as_str(), Some("é"));
        // Re-encode and re-parse: structurally stable.
        let encoded = json::encode(&v);
        let v2 = json::parse(&encoded).expect("re-parse");
        assert_eq!(v, v2);
    }

    #[test]
    fn test_url_parse_variants() {
        let u = http::Url::parse("http://127.0.0.1:8080/v1/skills?x=1").unwrap();
        assert_eq!(u.scheme, "http");
        assert_eq!(u.host, "127.0.0.1");
        assert_eq!(u.port, 8080);
        assert_eq!(u.path, "/v1/skills?x=1");

        let h = http::Url::parse("https://thready.hxd3v.com/v1").unwrap();
        assert_eq!(h.scheme, "https");
        assert_eq!(h.port, 443);

        let v6 = http::Url::parse("http://[::1]:9090/v1/posts/p1").unwrap();
        assert_eq!(v6.host, "::1");
        assert_eq!(v6.port, 9090);
        assert!(is_loopback_host(&v6.host));
    }
}
