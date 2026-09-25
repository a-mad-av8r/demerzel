require "digest"
require "json"
require "pathname"

class Demerzel < Formula
  desc "Local-first LLM gateway"
  homepage "https://github.com/a-mad-av8r/demerzel"
  license "MIT"
  version "2.0.0"

  # Homebrew stages the tap formula locally; installation fetches signed release assets.
  url "file://#{File.expand_path(__FILE__)}"
  sha256 Digest::SHA256.file(__FILE__).hexdigest

  depends_on "age"
  depends_on "cosign" => :build

  def install
    asset = Hardware::CPU.arm? ? "demerzel-macos-arm64" : "demerzel-macos-amd64"
    release_base = "https://github.com/a-mad-av8r/demerzel/releases/download/v#{version}"
    artifacts = buildpath/"demerzel-release"
    artifacts.mkpath
    ["manifest.json", "manifest.sigstore.json", asset].each do |name|
      system "curl",
        "--fail", "--location", "--silent", "--show-error",
        "--proto", "=https", "--tlsv1.2",
        "--output", (artifacts/name).to_s,
        "#{release_base}/#{name}"
    end

    expected_tag = "v#{version}"
    expected_identity = "^https://github\\.com/a-mad-av8r/demerzel/\\.github/workflows/release\\.yml@refs/tags/#{Regexp.escape(expected_tag)}$"
    system "cosign", "verify-blob",
      "--bundle", (artifacts/"manifest.sigstore.json").to_s,
      "--certificate-identity-regexp", expected_identity,
      "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
      (artifacts/"manifest.json").to_s

    manifest = JSON.parse(File.read(artifacts/"manifest.json"))
    odie "signed manifest version does not match this formula" unless manifest["schemaVersion"] == 1 && manifest["repository"] == "a-mad-av8r/demerzel" && manifest["tag"] == expected_tag && manifest["version"] == version
    expected_digest = manifest.fetch("assets").find { |entry| entry["name"] == asset }&.fetch("sha256")
    odie "signed manifest does not list the selected macOS binary" unless expected_digest&.match?(/\A[a-f0-9]{64}\z/)
    binary = artifacts/asset
    actual_digest = Digest::SHA256.file(binary).hexdigest
    odie "selected binary digest does not match the signed manifest" unless actual_digest == expected_digest

    libexec.install binary => "demerzel"
    bin.mkpath
    (bin/"demerzel").write <<~SH
      #!/bin/sh
      set -eu
      pin="${HOME}/.config/demerzel/data-dir"
      if [ ! -f "$pin" ] || [ -L "$pin" ]; then
        printf 'restore the owner-only Demerzel data-root pin before starting\\n' >&2
        exit 1
      fi
      IFS= read -r data_dir < "$pin"
      case "$data_dir" in
        /*) ;;
        *) printf 'invalid pinned DATA_DIR\\n' >&2; exit 1 ;;
      esac
      if [ -n "${DATA_DIR:-}" ] && [ "$DATA_DIR" != "$data_dir" ]; then
        printf 'DATA_DIR differs from the pinned Homebrew installation\\n' >&2
        exit 1
      fi
      export DATA_DIR="$data_dir"
      exec "#{libexec}/demerzel" "$@"
    SH
    chmod 0755, bin/"demerzel"
  end

  def post_install
    config_dir = Pathname.new(Dir.home)/".config"/"demerzel"
    data_dir_config = config_dir/"data-dir"
    identity = config_dir/"identity.txt"
    requested_data_dir = ENV["HOMEBREW_DEMERZEL_DATA_DIR"]
    config_dir.mkpath
    odie "#{config_dir} must not be a symlink" if config_dir.symlink?
    config_dir.chmod(0700)
    odie "#{data_dir_config} must not be a symlink" if data_dir_config.symlink?

    if data_dir_config.file?
      configured_path = File.read(data_dir_config).strip
      odie "configured DATA_DIR is missing; restore it instead of changing roots" if configured_path.empty?
      data_dir = Pathname.new(configured_path)
      odie "configured DATA_DIR must be absolute" unless data_dir.absolute?
      if requested_data_dir && Pathname.new(requested_data_dir).cleanpath != data_dir.cleanpath
        odie "DATA_DIR is already pinned; a root change requires the supervised restore/import runbook"
      end
      odie "configured DATA_DIR is missing or symlinked; restore the original path" unless data_dir.directory? && !data_dir.symlink?
      data_dir = data_dir.realpath
    else
      data_dir = if requested_data_dir
        requested = Pathname.new(requested_data_dir)
        odie "HOMEBREW_DEMERZEL_DATA_DIR must be absolute" unless requested.absolute?
        requested
      else
        Pathname.new(Dir.home)/".demerzel"
      end
      odie "#{data_dir} must not be a symlink" if data_dir.symlink?
      data_dir.mkpath
      data_dir = data_dir.realpath
      File.open(data_dir_config, File::WRONLY | File::CREAT | File::EXCL, 0600) do |file|
        file.write("#{data_dir}\n")
      end
    end
    data_dir_config.chmod(0600)
    identity_path = identity.expand_path.to_s
    odie "age identity must be outside #{data_dir}" if identity_path.start_with?("#{data_dir}/")
    odie "#{identity} must not be a symlink" if identity.symlink?

    unless identity.exist?
      existing_state = !data_dir.children.empty?
      odie "#{data_dir} contains state but its external age identity is missing; restore or migrate custody before service startup" if existing_state

      system "age-keygen", "-o", identity.to_s
    end
    odie "age identity must be a regular file outside #{data_dir}" unless identity.file? && !identity.symlink?
    identity.chmod(0600)
    recipient = Utils.safe_popen_read("age-keygen", "-y", identity.to_s).strip
    odie "configured age identity did not produce a valid recipient" unless recipient.match?(/\Aage1[0-9a-z]+\z/)
    data_dir.chmod(0700)
    (data_dir/"logs").mkpath
    (data_dir/"logs").chmod(0700)
  end

  service do
    config_dir = File.join(Dir.home, ".config", "demerzel")
    identity = File.join(config_dir, "identity.txt")
    pin = File.join(config_dir, "data-dir")
    data_dir = File.file?(pin) ? File.read(pin).strip : File.join(Dir.home, ".demerzel")
    recipient = File.file?(identity) ? Utils.safe_popen_read("age-keygen", "-y", identity).strip : ""
    run [opt_bin/"demerzel"]
    run_at_load true
    keep_alive true
    working_dir opt_prefix
    environment_variables(
      "HOST" => "127.0.0.1",
      "DATA_DIR" => data_dir,
      "DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE" => identity,
      "DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT" => recipient,
    )
    log_path File.join(data_dir, "logs", "launchd.log")
    error_log_path File.join(data_dir, "logs", "launchd-error.log")
  end
end
