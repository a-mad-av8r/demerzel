require "digest"
require "json"
require "pathname"

class Demerzel < Formula
  desc "Private local-first LLM gateway"
  homepage "https://github.com/a-mad-av8r/demerzel"
  license "MIT"
  repo = "a-mad-av8r/demerzel"
  local_artifacts = ENV["HOMEBREW_DEMERZEL_ARTIFACT_DIR"]
  local_manifest = if local_artifacts && File.file?(File.join(local_artifacts, "manifest.json"))
    JSON.parse(File.read(File.join(local_artifacts, "manifest.json")))
  end
  version local_manifest ? local_manifest.fetch("version") : "2.0.0"

  if local_artifacts
    asset = Hardware::CPU.arm? ? "demerzel-macos-arm64" : "demerzel-macos-amd64"
    url "file://#{File.expand_path(File.join(local_artifacts, asset))}"
  elsif Hardware::CPU.arm?
    url "https://github.com/#{repo}/releases/download/v#{version}/demerzel-macos-arm64"
  else
    url "https://github.com/#{repo}/releases/download/v#{version}/demerzel-macos-amd64"
  end
  sha256 :no_check

  depends_on "age"
  depends_on "cosign" => :build
  depends_on "gh" => :build

  def install
    asset = Hardware::CPU.arm? ? "demerzel-macos-arm64" : "demerzel-macos-amd64"
    artifacts = if (path = ENV["HOMEBREW_DEMERZEL_ARTIFACT_DIR"])
      Pathname.new(path).realpath
    else
      odie "authenticate with gh auth login --hostname github.com before installing this private formula" unless system "gh", "auth", "status", "--hostname", "github.com"

      directory = buildpath/"demerzel-release"
      directory.mkpath
      system "gh", "release", "download", "v#{version}", "--repo", "a-mad-av8r/demerzel", "--pattern", "manifest.json", "--pattern", "manifest.sigstore.json", "--pattern", asset, "--dir", directory.to_s
      directory
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

    bin.install binary => "demerzel"
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
    data_dir = File.read(File.join(config_dir, "data-dir")).strip
    recipient = Utils.safe_popen_read("age-keygen", "-y", identity).strip
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
