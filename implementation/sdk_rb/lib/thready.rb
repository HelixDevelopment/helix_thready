# frozen_string_literal: true

# Thready is the stdlib-only Ruby SDK client for the Helix Thready REST `/v1`
# control API (schema: docs/public/research/mvp/api/openapi.yaml; realized by the
# implementation/rest_gateway module, and mirrored from implementation/sdk_go).
#
# It is self-contained: it uses ONLY the Ruby standard library (net/http, json,
# uri, securerandom, ipaddr) and imports no sibling implementation modules, so it
# can be vendored on its own.
#
# A Thready::Client injects auth (a JWT bearer access token OR an X-API-Key),
# encodes and decodes JSON, maps every non-2xx response to a typed
# Thready::ApiError, retries idempotent GETs on transient 503/429 with capped
# exponential backoff, and stamps a fresh Idempotency-Key onto unsafe POSTs.
#
# SECURITY: it refuses to attach a credential to a plaintext-http request bound
# for a non-loopback host (raising Thready::InsecureTransportError BEFORE any
# bytes leave the process) unless allow_insecure_http was explicitly opted into.

require "net/http"
require "json"
require "uri"
require "securerandom"
require "ipaddr"

module Thready
  VERSION = "0.1.0"

  # Default tuning for a freshly constructed Client.
  DEFAULT_TIMEOUT      = 30
  DEFAULT_MAX_RETRIES  = 3
  DEFAULT_BACKOFF_BASE = 0.025 # seconds
  DEFAULT_BACKOFF_MAX  = 2.0   # seconds

  # ---------------------------------------------------------------------------
  # Errors
  # ---------------------------------------------------------------------------

  # Base class for every error raised by the SDK.
  class Error < StandardError; end

  # Raised instead of attaching a credential (an "Authorization: Bearer …" or
  # "X-API-Key: …" header) to a request that would travel over plaintext http to
  # a NON-loopback host. Sending a bearer token or API key in the clear to a
  # remote origin would expose it to any on-path observer, so the SDK refuses by
  # default. https (any host) and http to a loopback host (127.0.0.0/8, ::1,
  # localhost) are always allowed; pass allow_insecure_http: true to opt out on a
  # trusted network. It is raised BEFORE any request is sent.
  class InsecureTransportError < Error; end

  # ApiError is the typed error surfaced for every non-2xx response. It is
  # decoded from the gateway's canonical failure envelope:
  #
  #   {"error":{"code","message","status","request_id"[,"trace_id","details"]}}
  #
  # +code+ is a stable machine string (mirrors the Connect/gRPC canonical codes);
  # +status+ is the mirrored HTTP status; +request_id+ correlates support logs.
  class ApiError < Error
    attr_reader :code, :status, :request_id, :details

    def initialize(code:, message:, status:, request_id: nil, details: nil)
      @code = code
      @status = status
      @request_id = request_id
      @details = details
      suffix = request_id && !request_id.empty? ? " [request_id=#{request_id}]" : ""
      super("thready: #{code} (#{status}): #{message}#{suffix}")
    end

    # Retryable reports whether the code is one the SDK may transparently retry.
    def retryable?
      %w[rate_limited unavailable deadline_exceeded].include?(code)
    end
  end

  # Canonical code <- HTTP status fallback (used only when a non-2xx body is NOT
  # the canonical envelope; the gateway itself always sends the envelope).
  STATUS_TO_CODE = {
    400 => "invalid_argument",
    401 => "unauthenticated",
    403 => "permission_denied",
    404 => "not_found",
    409 => "conflict",
    412 => "failed_precondition",
    422 => "unprocessable",
    429 => "rate_limited",
    500 => "internal",
    503 => "unavailable",
    504 => "deadline_exceeded",
  }.freeze

  # ---------------------------------------------------------------------------
  # Client
  # ---------------------------------------------------------------------------

  # Client is a typed client for the Thready `/v1` API. Exactly one of
  # access_token or api_key is normally set; if both are present the bearer
  # access_token wins. A successful #login updates the in-flight access_token so
  # later calls authenticate automatically.
  class Client
    attr_reader :base_url, :access_token, :api_key, :timeout, :allow_insecure_http

    # base_url is the gateway origin, with or without a trailing slash, e.g.
    # "https://thready.hxd3v.com" or "http://127.0.0.1:8080". Required.
    def initialize(base_url:, access_token: nil, api_key: nil,
                   timeout: DEFAULT_TIMEOUT, allow_insecure_http: false,
                   max_retries: DEFAULT_MAX_RETRIES,
                   backoff_base: DEFAULT_BACKOFF_BASE,
                   backoff_max: DEFAULT_BACKOFF_MAX)
      base = base_url.to_s.strip.sub(%r{/+\z}, "")
      raise ArgumentError, "thready: base_url is required" if base.empty?

      @base_url = base
      @access_token = access_token
      @api_key = api_key
      @timeout = timeout
      @allow_insecure_http = allow_insecure_http
      @max_retries = max_retries
      @backoff_base = backoff_base
      @backoff_max = backoff_max
    end

    # -- Methods over the /v1 surface -----------------------------------------

    # login exchanges credentials (plus TOTP for admin tiers) for a token pair
    # and stores the returned access token on the Client so subsequent calls
    # authenticate automatically. POST /v1/auth/login.
    def login(email:, password:, totp: nil)
      body = { email: email, password: password }
      body[:totp] = totp unless totp.nil?
      pair = request_json(:post, "/v1/auth/login", body: body)
      @access_token = pair[:access_token] if pair.is_a?(Hash) && pair[:access_token]
      pair
    end

    # list_channels lists the channels registered for the caller's tenant.
    # GET /v1/channels. Returns the decoded `data` array.
    def list_channels
      env = request_json(:get, "/v1/channels")
      env.is_a?(Hash) ? (env[:data] || []) : []
    end

    # create_channel registers a channel/group to read. It is an unsafe POST, so
    # it carries an auto-generated Idempotency-Key (override via idempotency_key:).
    # POST /v1/channels.
    def create_channel(name:, platform: nil, external_ref: nil, idempotency_key: nil)
      body = { name: name }
      body[:platform] = platform unless platform.nil?
      body[:external_ref] = external_ref unless external_ref.nil?
      request_json(:post, "/v1/channels", body: body,
                   idempotency_key: idempotency_key || SecureRandom.uuid)
    end

    # get_post fetches a single post by id. GET /v1/posts/{id}.
    def get_post(id)
      request_json(:get, "/v1/posts/#{escape(id)}")
    end

    # reprocess forces a fresh processing run for a post and returns the queued
    # job (202 Accepted). Unsafe POST -> carries an auto Idempotency-Key.
    # POST /v1/posts/{id}/reprocess.
    def reprocess(id, idempotency_key: nil)
      request_json(:post, "/v1/posts/#{escape(id)}/reprocess", body: {},
                   idempotency_key: idempotency_key || SecureRandom.uuid)
    end

    # search runs a semantic / keyword / hybrid search over posts and generated
    # materials. POST /v1/search.
    def search(query:, mode: nil, top_k: nil, sources: nil, rerank: nil)
      body = { query: query }
      body[:mode] = mode unless mode.nil?
      body[:sources] = sources unless sources.nil?
      body[:top_k] = top_k unless top_k.nil?
      body[:rerank] = rerank unless rerank.nil?
      request_json(:post, "/v1/search", body: body)
    end

    # list_skills lists the Skill-Graph knowledge units. GET /v1/skills.
    # Returns the decoded `data` array.
    def list_skills
      env = request_json(:get, "/v1/skills")
      env.is_a?(Hash) ? (env[:data] || []) : []
    end

    private

    # request_json performs a request and decodes the 2xx body into a Ruby
    # Hash/Array (symbolized keys), or raises a typed ApiError on non-2xx.
    def request_json(method, path, query: nil, body: nil, idempotency_key: nil)
      response = request(method, path, query: query, body: body, idempotency_key: idempotency_key)
      decode_response(response)
    end

    # request builds and performs the HTTP request, retrying idempotent GETs on
    # transient 503/429 (and transport errors) with capped exponential backoff.
    # It returns the raw Net::HTTPResponse; error mapping happens in the caller.
    def request(method, path, query: nil, body: nil, idempotency_key: nil)
      uri = build_uri(path, query)
      payload = body.nil? ? nil : JSON.generate(body)
      attempts = method == :get ? @max_retries + 1 : 1
      last_error = nil

      attempts.times do |attempt|
        sleep(backoff_delay(attempt)) if attempt.positive?

        begin
          response = perform(method, uri, payload, idempotency_key)
        rescue InsecureTransportError
          raise # never retried, never swallowed — no bytes left the process
        rescue StandardError => e
          last_error = e
          next if method == :get && attempt < attempts - 1

          raise
        end

        status = response.code.to_i
        if method == :get && attempt < attempts - 1 && [503, 429].include?(status)
          last_error = parse_api_error(response)
          next
        end

        return response
      end

      raise last_error if last_error
    end

    # perform builds the request (applying auth, which may raise
    # InsecureTransportError BEFORE any connection is opened) and executes it.
    def perform(method, uri, payload, idempotency_key)
      req = build_request(method, uri, payload, idempotency_key)
      Net::HTTP.start(uri.hostname, uri.port,
                      use_ssl: uri.scheme == "https",
                      open_timeout: @timeout,
                      read_timeout: @timeout) do |http|
        http.request(req)
      end
    end

    def build_request(method, uri, payload, idempotency_key)
      klass = method == :get ? Net::HTTP::Get : Net::HTTP::Post
      req = klass.new(uri)
      req["Accept"] = "application/json"
      if payload
        req["Content-Type"] = "application/json"
        req.body = payload
      end
      req["Idempotency-Key"] = idempotency_key if idempotency_key
      apply_auth(req, uri)
      req
    end

    # apply_auth injects the credential: a bearer JWT when present, otherwise an
    # X-API-Key (bearer wins). When a credential IS present it first enforces the
    # transport policy, raising InsecureTransportError (and attaching no header)
    # for plaintext http to a non-loopback host.
    def apply_auth(req, uri)
      token = @access_token
      has_credential = present?(token) || present?(@api_key)
      raise InsecureTransportError, insecure_message if has_credential && !transport_allowed?(uri)

      if present?(token)
        req["Authorization"] = "Bearer #{token}"
      elsif present?(@api_key)
        req["X-API-Key"] = @api_key
      end
    end

    # transport_allowed? reports whether it is safe to attach a credential to a
    # request bound for uri. https (or any non-http scheme) is always fine;
    # plaintext http is allowed only to a loopback host, or unconditionally when
    # allow_insecure_http was opted into.
    def transport_allowed?(uri)
      return true if @allow_insecure_http
      return true unless uri.scheme == "http"

      loopback_host?(uri.hostname)
    end

    # loopback_host? reports whether host refers to the local machine: the
    # literal "localhost", or any loopback IP (127.0.0.0/8, ::1).
    def loopback_host?(host)
      return false if host.nil? || host.empty?
      return true if host == "localhost"

      begin
        IPAddr.new(host).loopback?
      rescue IPAddr::Error
        false
      end
    end

    def decode_response(response)
      status = response.code.to_i
      if status >= 200 && status < 300
        return nil if status == 204

        body = response.body.to_s
        return nil if body.strip.empty?

        return JSON.parse(body, symbolize_names: true)
      end

      raise parse_api_error(response)
    end

    # parse_api_error reads a non-2xx body and maps it to a typed ApiError. It
    # prefers the canonical {"error":{code,message,status,request_id,…}} envelope,
    # backfilling status/request_id from the HTTP status line and headers, and
    # degrades gracefully to a status-derived error for a non-envelope body.
    def parse_api_error(response)
      status = response.code.to_i
      body = response.body.to_s
      parsed = begin
        JSON.parse(body, symbolize_names: true)
      rescue JSON::ParserError
        nil
      end

      if parsed.is_a?(Hash) && parsed[:error].is_a?(Hash)
        err = parsed[:error]
        return ApiError.new(
          code: err[:code] || STATUS_TO_CODE.fetch(status, "internal"),
          message: err[:message] || default_message(body, status),
          status: err[:status] || status,
          request_id: err[:request_id] || err[:trace_id] || response["x-request-id"],
          details: err[:details],
        )
      end

      ApiError.new(
        code: STATUS_TO_CODE.fetch(status, "internal"),
        message: default_message(body, status),
        status: status,
        request_id: response["x-request-id"],
      )
    end

    def default_message(body, status)
      msg = body.to_s.strip
      msg.empty? ? "HTTP #{status}" : msg
    end

    def build_uri(path, query)
      uri = URI.parse(@base_url + path)
      uri.query = URI.encode_www_form(query) if query && !query.empty?
      uri
    end

    def escape(segment)
      # UUID path segments are already URL-safe; escape defensively anyway.
      segment.to_s.gsub(%r{[^a-zA-Z0-9\-._~]}) { |c| format("%%%02X", c.ord) }
    end

    def backoff_delay(attempt)
      delay = @backoff_base * (2**(attempt - 1))
      delay > @backoff_max ? @backoff_max : delay
    end

    def present?(value)
      !value.nil? && !value.to_s.empty?
    end

    def insecure_message
      "thready: refusing to send credentials over plaintext http to a " \
      "non-loopback host; use https or pass allow_insecure_http: true"
    end
  end
end
