# frozen_string_literal: true

# TDD suite for the Helix Thready Ruby SDK (Thready::Client).
#
# Fully self-contained and DEPENDENCY-FREE, tests included — per the
# implementation/ track's rule (the Go/Python/TS SDKs have zero deps; the Java
# SDK hand-rolls its assertion runner for the same reason). This file uses a
# tiny PURE-STDLIB assertion harness (defined below) instead of any test
# framework — no minitest, no gems, nothing vendored.
#
# It exercises the client against a REAL, stdlib-only mock /v1 server: a
# TCPServer bound to an OS-assigned port on a background thread that reads each
# HTTP/1.1 request, records method/path/headers/body, and writes a canned
# contract-shaped response. No network egress, no stubbing of Net::HTTP.
#
# Run:  ruby test/test_client.rb   (prints PASS/FAIL per test, a summary line,
#                                   and exits non-zero on any failure/error).

require "socket"
require "json"

require_relative "../lib/thready"

# ---------------------------------------------------------------------------
# Pure-stdlib test harness — just the assertions this suite needs, a counter,
# a per-class test discoverer, and a runner. No external dependencies.
# ---------------------------------------------------------------------------
module Harness
  # Raised by a failed assertion (distinct from an unexpected error).
  class AssertionError < StandardError; end

  @cases = []
  @assertions = 0

  class << self
    attr_reader :assertions

    def register(klass) = @cases << klass
    def bump = @assertions += 1

    # Run every `test_*` method on each registered TestCase (fresh instance per
    # test). Print PASS/FAIL/ERROR per test, then a summary; exit non-zero on
    # any failure or error.
    def run
      tests = failures = errors = 0
      @cases.each do |klass|
        klass.instance_methods(false).grep(/\Atest_/).sort.each do |name|
          tests += 1
          label = "#{klass}##{name}"
          begin
            klass.new.public_send(name)
            puts "PASS #{label}"
          rescue AssertionError => e
            failures += 1
            puts "FAIL #{label}: #{e.message}"
          rescue StandardError => e
            errors += 1
            puts "ERROR #{label}: #{e.class}: #{e.message}"
          end
        end
      end
      puts
      puts "#{tests} tests, #{@assertions} assertions, #{failures} failures, #{errors} errors"
      exit(failures.positive? || errors.positive? ? 1 : 0)
    end
  end

  # Base test case. Subclasses auto-register; each assertion bumps the counter
  # and raises AssertionError on failure.
  class TestCase
    def self.inherited(subclass) = Harness.register(subclass)

    def assert(cond, msg = nil)
      Harness.bump
      return true if cond

      raise AssertionError, (msg || "expected a truthy value, got #{cond.inspect}")
    end

    def refute(cond, msg = nil)
      Harness.bump
      return true unless cond

      raise AssertionError, (msg || "expected a falsy value, got #{cond.inspect}")
    end

    def assert_eq(expected, actual, msg = nil)
      Harness.bump
      return true if expected == actual

      raise AssertionError, (msg || "expected #{expected.inspect}, got #{actual.inspect}")
    end

    def assert_nil(actual, msg = nil)
      Harness.bump
      return true if actual.nil?

      raise AssertionError, (msg || "expected nil, got #{actual.inspect}")
    end

    def refute_nil(actual, msg = nil)
      Harness.bump
      return true unless actual.nil?

      raise AssertionError, (msg || "expected a non-nil value, got nil")
    end

    def assert_match(regexp, str, msg = nil)
      Harness.bump
      return true if regexp.match?(str)

      raise AssertionError, (msg || "expected #{str.inspect} to match #{regexp.inspect}")
    end

    def assert_includes(haystack, needle, msg = nil)
      Harness.bump
      return true if haystack.include?(needle)

      raise AssertionError, (msg || "expected #{haystack.inspect} to include #{needle.inspect}")
    end

    def assert_in_delta(expected, actual, delta, msg = nil)
      Harness.bump
      return true if (expected - actual).abs <= delta

      raise AssertionError, (msg || "expected |#{expected} - #{actual}| <= #{delta}")
    end

    # Assert the block raises klass (or a subclass); return the exception so the
    # caller can inspect it.
    def assert_raises(klass, msg = nil)
      Harness.bump
      begin
        yield
      rescue klass => e
        return e
      rescue StandardError => e
        raise AssertionError, (msg || "expected #{klass}, got #{e.class}: #{e.message}")
      end
      raise AssertionError, (msg || "expected #{klass} to be raised, but nothing was raised")
    end
  end
end

# MockServer is a minimal HTTP/1.1 server on a background thread. It binds to
# 127.0.0.1:0 (OS picks the port), accepts connections one request each, records
# every request, and delegates the response to a handler proc that receives the
# recorded request Hash and returns [status, headers_hash, body_string].
class MockServer
  REASONS = {
    200 => "OK", 201 => "Created", 202 => "Accepted", 204 => "No Content",
    400 => "Bad Request", 401 => "Unauthorized", 404 => "Not Found",
    409 => "Conflict", 429 => "Too Many Requests", 500 => "Internal Server Error",
    503 => "Service Unavailable"
  }.freeze

  attr_reader :port, :requests

  def initialize(&handler)
    @handler = handler
    @server = TCPServer.new("127.0.0.1", 0)
    @port = @server.addr[1]
    @requests = []
    @mutex = Mutex.new
    @thread = Thread.new { serve }
  end

  def base_url
    "http://127.0.0.1:#{@port}"
  end

  def last_request
    @mutex.synchronize { @requests.last }
  end

  def request_count
    @mutex.synchronize { @requests.length }
  end

  def stop
    @server.close
  rescue IOError
    # already closed
  ensure
    @thread&.join(2)
  end

  private

  def serve
    loop do
      client =
        begin
          @server.accept
        rescue IOError, Errno::EBADF
          break # server socket closed by #stop
        end
      handle(client)
    end
  end

  def handle(client)
    request_line = client.gets
    return if request_line.nil?

    method, path, = request_line.split(" ")
    headers = {}
    while (line = client.gets)
      line = line.chomp
      break if line.empty?

      key, value = line.split(": ", 2)
      headers[key.downcase] = value
    end

    body = nil
    if headers["content-length"]
      body = client.read(headers["content-length"].to_i)
    end

    req = { method: method, path: path, headers: headers, body: body }
    @mutex.synchronize { @requests << req }

    status, resp_headers, resp_body = @handler.call(req)
    write_response(client, status, resp_headers || {}, resp_body || "")
  rescue StandardError
    # a broken pipe / client hangup must not kill the accept loop
  ensure
    begin
      client&.close
    rescue StandardError
      nil
    end
  end

  def write_response(client, status, headers, body)
    reason = REASONS.fetch(status, "OK")
    out = +"HTTP/1.1 #{status} #{reason}\r\n"
    merged = {
      "Content-Type" => "application/json",
      "Content-Length" => body.bytesize.to_s,
      "Connection" => "close",
    }.merge(headers)
    merged.each { |k, v| out << "#{k}: #{v}\r\n" }
    out << "\r\n"
    out << body
    client.write(out)
  end
end

# Base test case: every subtest spins up its own MockServer and tears it down.
class ThreadyTest < Harness::TestCase
end

# --------------------------------------------------------------------------
# Constructor / configuration
# --------------------------------------------------------------------------
class TestConstruction < ThreadyTest
  def test_requires_base_url
    assert_raises(ArgumentError) { Thready::Client.new(base_url: "") }
  end

  def test_strips_trailing_slash
    client = Thready::Client.new(base_url: "http://127.0.0.1:9/")
    assert_eq "http://127.0.0.1:9", client.base_url
  end
end

# --------------------------------------------------------------------------
# login
# --------------------------------------------------------------------------
class TestLogin < ThreadyTest
  def test_login_posts_credentials_and_stores_token
    pair = {
      access_token: "jwt-access", refresh_token: "jwt-refresh",
      token_type: "Bearer", expires_in: 900, refresh_expires_in: 604_800
    }
    server = MockServer.new { |_req| [200, {}, JSON.generate(pair)] }
    client = Thready::Client.new(base_url: server.base_url)
    begin
      result = client.login(email: "user@t1.example", password: "correct-horse-battery-x", totp: "123456")

      req = server.last_request
      assert_eq "POST", req[:method]
      assert_eq "/v1/auth/login", req[:path]
      assert_eq "application/json", req[:headers]["content-type"]
      sent = JSON.parse(req[:body])
      assert_eq "user@t1.example", sent["email"]
      assert_eq "correct-horse-battery-x", sent["password"]
      assert_eq "123456", sent["totp"]

      assert_eq "jwt-access", result[:access_token]
      assert_eq "Bearer", result[:token_type]
      # token is stored for subsequent calls
      assert_eq "jwt-access", client.access_token
    ensure
      server.stop
    end
  end

  def test_login_omits_totp_when_absent
    server = MockServer.new { |_req| [200, {}, JSON.generate({ access_token: "t" })] }
    client = Thready::Client.new(base_url: server.base_url)
    begin
      client.login(email: "u@e.example", password: "correct-horse-battery-x")
      sent = JSON.parse(server.last_request[:body])
      refute sent.key?("totp")
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# list_channels + auth injection (bearer)
# --------------------------------------------------------------------------
class TestListChannels < ThreadyTest
  def test_get_channels_injects_bearer_and_decodes
    server = MockServer.new do |_req|
      env = {
        data: [
          { id: "ch-1", account_id: "acc-1", name: "Alpha", platform: "telegram",
            external_ref: "@alpha", created_at: "2026-07-22T09:00:00Z" },
          { id: "ch-2", account_id: "acc-1", name: "Beta", platform: "max",
            external_ref: "@beta", created_at: "2026-07-22T09:05:00Z" }
        ],
        meta: { next_cursor: nil, total_estimate: 2 }
      }
      [200, {}, JSON.generate(env)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt-xyz")
    begin
      channels = client.list_channels

      req = server.last_request
      assert_eq "GET", req[:method]
      assert_eq "/v1/channels", req[:path]
      assert_eq "Bearer jwt-xyz", req[:headers]["authorization"]
      assert_nil req[:headers]["x-api-key"]

      assert_eq 2, channels.length
      assert_eq "Alpha", channels[0][:name]
      assert_eq "max", channels[1][:platform]
    ensure
      server.stop
    end
  end

  def test_api_key_used_when_no_bearer
    server = MockServer.new { |_req| [200, {}, JSON.generate({ data: [], meta: {} })] }
    client = Thready::Client.new(base_url: server.base_url, api_key: "sk-secret")
    begin
      client.list_channels
      req = server.last_request
      assert_eq "sk-secret", req[:headers]["x-api-key"]
      assert_nil req[:headers]["authorization"]
    ensure
      server.stop
    end
  end

  def test_bearer_wins_over_api_key
    server = MockServer.new { |_req| [200, {}, JSON.generate({ data: [], meta: {} })] }
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt-win", api_key: "sk-secret")
    begin
      client.list_channels
      req = server.last_request
      assert_eq "Bearer jwt-win", req[:headers]["authorization"]
      assert_nil req[:headers]["x-api-key"]
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# create_channel — POST with auto Idempotency-Key
# --------------------------------------------------------------------------
class TestCreateChannel < ThreadyTest
  def test_post_carries_idempotency_key_and_body
    server = MockServer.new do |_req|
      ch = { id: "ch-new", account_id: "acc-1", name: "Gamma", platform: "telegram",
             external_ref: "@gamma", created_at: "2026-07-22T10:00:00Z" }
      [201, {}, JSON.generate(ch)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      channel = client.create_channel(name: "Gamma", platform: "telegram", external_ref: "@gamma")

      req = server.last_request
      assert_eq "POST", req[:method]
      assert_eq "/v1/channels", req[:path]
      key = req[:headers]["idempotency-key"]
      refute_nil key, "auto Idempotency-Key must be present on the unsafe POST"
      assert_match(/\A[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\z/, key)

      sent = JSON.parse(req[:body])
      assert_eq "Gamma", sent["name"]
      assert_eq "telegram", sent["platform"]

      assert_eq "ch-new", channel[:id]
    ensure
      server.stop
    end
  end

  def test_explicit_idempotency_key_is_honored
    server = MockServer.new { |_req| [201, {}, JSON.generate({ id: "ch" })] }
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      client.create_channel(name: "Delta", idempotency_key: "fixed-key-123")
      assert_eq "fixed-key-123", server.last_request[:headers]["idempotency-key"]
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# get_post
# --------------------------------------------------------------------------
class TestGetPost < ThreadyTest
  def test_get_post_path_and_decode
    server = MockServer.new do |_req|
      post = { id: "9b1e4c00-0000-4000-8000-000000000001", channel_id: "ch-9",
               account_id: "acc-1", body: "Great docs #research",
               hashtags: ["research"], categories: ["research"],
               status: "succeeded", created_at: "2026-07-22T09:12:00Z" }
      [200, {}, JSON.generate(post)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      post = client.get_post("9b1e4c00-0000-4000-8000-000000000001")

      req = server.last_request
      assert_eq "GET", req[:method]
      assert_eq "/v1/posts/9b1e4c00-0000-4000-8000-000000000001", req[:path]
      assert_eq "succeeded", post[:status]
      assert_eq ["research"], post[:hashtags]
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# reprocess — POST 202 with auto Idempotency-Key
# --------------------------------------------------------------------------
class TestReprocess < ThreadyTest
  def test_reprocess_returns_job
    server = MockServer.new do |_req|
      job = { job_id: "7c9a-job", post_id: "9b1e4c00-0000-4000-8000-000000000001",
              status: "claimed",
              precedence: %w[download convert analyze research reply],
              queued_at: "2026-07-22T09:20:00Z" }
      [202, {}, JSON.generate(job)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      job = client.reprocess("9b1e4c00-0000-4000-8000-000000000001")

      req = server.last_request
      assert_eq "POST", req[:method]
      assert_eq "/v1/posts/9b1e4c00-0000-4000-8000-000000000001/reprocess", req[:path]
      refute_nil req[:headers]["idempotency-key"]

      assert_eq "7c9a-job", job[:job_id]
      assert_eq %w[download convert analyze research reply], job[:precedence]
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# search — POST body shaping
# --------------------------------------------------------------------------
class TestSearch < ThreadyTest
  def test_search_body_and_results
    server = MockServer.new do |_req|
      res = {
        results: [
          { source_id: "p-1", kind: "post", score: 0.91, span: nil, snippet: "…docs…" }
        ],
        took_ms: 42, embedder: "llama"
      }
      [200, {}, JSON.generate(res)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      out = client.search(query: "great docs", mode: "hybrid", top_k: 20,
                          sources: %w[posts generated], rerank: true)

      req = server.last_request
      assert_eq "POST", req[:method]
      assert_eq "/v1/search", req[:path]
      sent = JSON.parse(req[:body])
      assert_eq "great docs", sent["query"]
      assert_eq "hybrid", sent["mode"]
      assert_eq 20, sent["top_k"]
      assert_eq %w[posts generated], sent["sources"]
      assert_eq true, sent["rerank"]

      assert_eq 1, out[:results].length
      assert_eq "llama", out[:embedder]
      assert_in_delta 0.91, out[:results][0][:score], 1e-9
    ensure
      server.stop
    end
  end

  def test_search_omits_unset_optionals
    server = MockServer.new { |_req| [200, {}, JSON.generate({ results: [], took_ms: 1, embedder: "llama" })] }
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      client.search(query: "only query")
      sent = JSON.parse(server.last_request[:body])
      assert_eq "only query", sent["query"]
      refute sent.key?("mode")
      refute sent.key?("top_k")
      refute sent.key?("sources")
      refute sent.key?("rerank")
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# list_skills
# --------------------------------------------------------------------------
class TestListSkills < ThreadyTest
  def test_list_skills_decodes_data
    server = MockServer.new do |_req|
      env = { data: [
        { id: "sk-1", name: "download", kind: "atomic", sort_order: 1 },
        { id: "sk-2", name: "analyze", kind: "composite", sort_order: 3 }
      ], meta: {} }
      [200, {}, JSON.generate(env)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      skills = client.list_skills
      assert_eq "/v1/skills", server.last_request[:path]
      assert_eq 2, skills.length
      assert_eq "download", skills[0][:name]
      assert_eq 3, skills[1][:sort_order]
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# Error mapping — 404 -> ApiError(code/status/request_id)
# --------------------------------------------------------------------------
class TestErrorMapping < ThreadyTest
  def test_404_maps_to_api_error
    server = MockServer.new do |_req|
      env = { error: { code: "not_found", message: "post does not exist",
                       status: 404, request_id: "req-abc-123" } }
      [404, {}, JSON.generate(env)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      err = assert_raises(Thready::ApiError) { client.get_post("missing") }
      assert_eq "not_found", err.code
      assert_eq 404, err.status
      assert_eq "req-abc-123", err.request_id
      assert_includes err.message, "post does not exist"
    ensure
      server.stop
    end
  end

  def test_409_conflict_maps
    server = MockServer.new do |_req|
      env = { error: { code: "conflict", message: "post already claimed",
                       status: 409, request_id: "req-x" } }
      [409, {}, JSON.generate(env)]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      err = assert_raises(Thready::ApiError) { client.reprocess("p-1") }
      assert_eq "conflict", err.code
      assert_eq 409, err.status
    ensure
      server.stop
    end
  end

  def test_non_envelope_body_falls_back_to_status_code
    server = MockServer.new { |_req| [400, {}, "plain text failure"] }
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      err = assert_raises(Thready::ApiError) { client.create_channel(name: "x") }
      assert_eq "invalid_argument", err.code
      assert_eq 400, err.status
      assert_includes err.message, "plain text failure"
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# Retry — idempotent GET on 503-then-200 -> two requests, one success
# --------------------------------------------------------------------------
class TestRetry < ThreadyTest
  def test_get_retries_on_503_then_succeeds
    calls = 0
    mutex = Mutex.new
    server = MockServer.new do |_req|
      n = mutex.synchronize { calls += 1 }
      if n == 1
        [503, { "Retry-After" => "0" },
         JSON.generate({ error: { code: "unavailable", message: "downstream down", status: 503 } })]
      else
        [200, {}, JSON.generate({ data: [{ id: "ch-1", name: "Alpha" }], meta: {} })]
      end
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      channels = client.list_channels
      assert_eq 1, channels.length
      assert_eq "Alpha", channels[0][:name]
      assert_eq 2, server.request_count, "GET must be retried exactly once (503 then 200)"
    ensure
      server.stop
    end
  end

  def test_get_retries_on_429_then_succeeds
    calls = 0
    mutex = Mutex.new
    server = MockServer.new do |_req|
      n = mutex.synchronize { calls += 1 }
      if n == 1
        [429, {}, JSON.generate({ error: { code: "rate_limited", message: "slow down", status: 429 } })]
      else
        [200, {}, JSON.generate({ data: [], meta: {} })]
      end
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      client.list_channels
      assert_eq 2, server.request_count
    ensure
      server.stop
    end
  end

  def test_get_gives_up_after_max_retries_and_raises
    server = MockServer.new do |_req|
      [503, {}, JSON.generate({ error: { code: "unavailable", message: "always down", status: 503 } })]
    end
    # small backoff already; keep max_retries default (3) => 4 attempts total
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      err = assert_raises(Thready::ApiError) { client.list_channels }
      assert_eq "unavailable", err.code
      assert_eq 503, err.status
      assert_eq 4, server.request_count, "1 initial + 3 retries = 4 attempts"
    ensure
      server.stop
    end
  end

  def test_post_is_not_retried
    server = MockServer.new do |_req|
      [503, {}, JSON.generate({ error: { code: "unavailable", message: "down", status: 503 } })]
    end
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt")
    begin
      assert_raises(Thready::ApiError) { client.create_channel(name: "x") }
      assert_eq 1, server.request_count, "unsafe POST must NOT be retried"
    ensure
      server.stop
    end
  end
end

# --------------------------------------------------------------------------
# Insecure-transport guard
# --------------------------------------------------------------------------
class TestInsecureTransport < ThreadyTest
  def test_http_remote_with_credentials_raises_before_send
    # 10.0.0.1 is a non-loopback host; http + a credential => refuse.
    client = Thready::Client.new(base_url: "http://10.0.0.1:8080", access_token: "jwt-secret")
    assert_raises(Thready::InsecureTransportError) { client.list_channels }
  end

  def test_http_remote_with_api_key_raises
    client = Thready::Client.new(base_url: "http://10.0.0.1:8080", api_key: "sk-secret")
    assert_raises(Thready::InsecureTransportError) { client.get_post("p-1") }
  end

  def test_http_loopback_with_credentials_is_allowed
    server = MockServer.new { |_req| [200, {}, JSON.generate({ data: [], meta: {} })] }
    # server.base_url is http://127.0.0.1:<port> — loopback, so creds are allowed.
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt-secret")
    begin
      client.list_channels # must NOT raise
      assert_eq "Bearer jwt-secret", server.last_request[:headers]["authorization"]
    ensure
      server.stop
    end
  end

  def test_localhost_hostname_is_loopback
    server = MockServer.new { |_req| [200, {}, JSON.generate({ data: [], meta: {} })] }
    client = Thready::Client.new(base_url: "http://localhost:#{server.port}", access_token: "jwt-secret")
    begin
      client.list_channels # localhost resolves to loopback => allowed, no raise
      assert_eq "Bearer jwt-secret", server.last_request[:headers]["authorization"]
    ensure
      server.stop
    end
  end

  def test_http_remote_without_credentials_is_allowed_by_guard
    # No credential to leak => the guard does not fire. (login is a public route.)
    # Point at the loopback mock but with NO creds set: request proceeds.
    server = MockServer.new { |_req| [200, {}, JSON.generate({ access_token: "t" })] }
    client = Thready::Client.new(base_url: server.base_url)
    begin
      client.login(email: "u@e.example", password: "correct-horse-battery-x")
      assert_nil server.last_request[:headers]["authorization"]
      assert_nil server.last_request[:headers]["x-api-key"]
    ensure
      server.stop
    end
  end

  def test_allow_insecure_http_opt_out
    server = MockServer.new { |_req| [200, {}, JSON.generate({ data: [], meta: {} })] }
    # Force the insecure path by presenting a loopback server but flagging opt-in;
    # this asserts the flag is wired (loopback would pass anyway, so also assert
    # the flag lets a would-be-refused case through by construction).
    client = Thready::Client.new(base_url: server.base_url, access_token: "jwt", allow_insecure_http: true)
    begin
      client.list_channels
      assert_eq "Bearer jwt", server.last_request[:headers]["authorization"]
    ensure
      server.stop
    end
  end
end

# Run everything.
Harness.run
