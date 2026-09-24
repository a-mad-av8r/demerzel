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
    data_dir = Pathname.new(Dir.home)/".demerzel"
    config_dir = Pathname.new(Dir.home)/".config"/"demerzel"
    identity = config_dir/"identity.txt"
    odie "#{data_dir} must not be a symlink" if data_dir.symlink?
    odie "#{config_dir} must not be a symlink" if config_dir.symlink?
    config_dir.mkpath
    config_dir.chmod(0700)

    unless identity.exist?
      existing_state = data_dir.directory? && !data_dir.children.empty?
      odie "#{data_dir} contains state but its external age identity is missing; restore the original identity instead of creating a replacement" if existing_state

      data_dir.mkpath
      data_dir.chmod(0700)
      system "age-keygen", "-o", identity.to_s
    end
    odie "age identity must be a regular file outside #{data_dir}" unless identity.file? && !identity.symlink?
    identity.chmod(0600)
    recipient = Utils.safe_popen_read("age-keygen", "-y", identity.to_s).strip
    odie "configured age identity did not produce a valid recipient" unless recipient.match?(/\Aage1[0-9a-z]+\z/)
    data_dir.mkpath
    data_dir.chmod(0700)
    (data_dir/"logs").mkpath
    (data_dir/"logs").chmod(0700)
  end

  service do
    identity = File.join(Dir.home, ".config", "demerzel", "identity.txt")
    recipient = Utils.safe_popen_read("age-keygen", "-y", identity).strip
    run [opt_bin/"demerzel"]
    run_at_load true
    keep_alive true
    working_dir opt_prefix
    environment_variables(
      "HOST" => "127.0.0.1",
      "DATA_DIR" => File.join(Dir.home, ".demerzel"),
      "DEMERZEL_ENCRYPTION_KEY_AGE_IDENTITY_FILE" => identity,
      "DEMERZEL_ENCRYPTION_KEY_AGE_RECIPIENT" => recipient,
    )
    log_path File.join(Dir.home, ".demerzel", "logs", "launchd.log")
    error_log_path File.join(Dir.home, ".demerzel", "logs", "launchd-error.log")
  end
end
