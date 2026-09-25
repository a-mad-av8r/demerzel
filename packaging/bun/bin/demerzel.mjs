#!/usr/bin/env node
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { createReadStream, createWriteStream } from "node:fs";
import {
  chmod,
  lstat,
  mkdir,
  mkdtemp,
  readFile,
  rename,
  rm,
} from "node:fs/promises";
import { homedir, tmpdir } from "node:os";
import { isAbsolute, join } from "node:path";
import { Readable } from "node:stream";
import { pipeline } from "node:stream/promises";
import { fileURLToPath } from "node:url";

const packageRoot = fileURLToPath(new URL("..", import.meta.url));
const packageInfo = JSON.parse(
  await readFile(join(packageRoot, "package.json"), "utf8"),
);
const repository = "a-mad-av8r/demerzel";
const version = packageInfo.version;
const tag = `v${version}`;
const releaseBase = `https://github.com/${repository}/releases/download/${tag}`;
const identity = `^https://github\\.com/${repository}/\\.github/workflows/release\\.yml@refs/tags/${escapeRegExp(tag)}$`;

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function printHelp() {
  process.stdout.write(`Demerzel Bun launcher ${version}

Usage:
  demerzel [gateway arguments]
  demerzel --help
  demerzel --version

This launcher verifies and runs the version-matched Demerzel Go gateway release.
Install cosign before starting the gateway. The binary and manifest are fetched
from the public GitHub release and checked against its Sigstore identity.
`);
}

function platformAsset() {
  const operatingSystem = { darwin: "macos", linux: "linux" }[process.platform];
  const architecture = { arm64: "arm64", x64: "amd64" }[process.arch];
  if (!operatingSystem || !architecture) {
    throw new Error(
      `Native Bun installation does not support ${process.platform}/${process.arch}; supported targets are macOS and Linux on amd64 or arm64.`,
    );
  }
  return `demerzel-${operatingSystem}-${architecture}`;
}

async function downloadAsset(name, destination) {
  const response = await fetch(`${releaseBase}/${name}`, {
    redirect: "follow",
    signal: AbortSignal.timeout(120_000),
  });
  if (!response.ok) {
    if (response.status === 404) {
      throw new Error(
        `The signed Demerzel release ${tag} or its ${name} asset has not been published.`,
      );
    }
    throw new Error(
      `Could not download ${name} from the Demerzel release (HTTP ${response.status}).`,
    );
  }
  if (!response.url.startsWith("https://")) {
    throw new Error(`Refusing a non-HTTPS release asset URL for ${name}.`);
  }
  if (!response.body)
    throw new Error(
      `The Demerzel release returned an empty response for ${name}.`,
    );
  await pipeline(
    Readable.fromWeb(response.body),
    createWriteStream(destination, { flags: "wx", mode: 0o600 }),
  );
  return destination;
}

function verifyManifest(manifestPath, bundlePath) {
  const result = spawnSync(
    "cosign",
    [
      "verify-blob",
      "--bundle",
      bundlePath,
      "--certificate-identity-regexp",
      identity,
      "--certificate-oidc-issuer",
      "https://token.actions.githubusercontent.com",
      manifestPath,
    ],
    { stdio: "inherit" },
  );
  if (result.error?.code === "ENOENT") {
    throw new Error(
      "cosign is required to verify the signed release. Install cosign and try again.",
    );
  }
  if (result.error) throw result.error;
  if (result.status !== 0)
    throw new Error("cosign did not verify the Demerzel release manifest.");
}

async function sha256File(path) {
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return hash.digest("hex");
}

async function ensureCacheDirectory() {
  const cacheBase = process.env.XDG_CACHE_HOME || join(homedir(), ".cache");
  if (!isAbsolute(cacheBase))
    throw new Error("XDG_CACHE_HOME must be an absolute path.");
  const cacheDirectory = join(cacheBase, "demerzel", "bun", version);
  await mkdir(cacheDirectory, { recursive: true, mode: 0o700 });
  const info = await lstat(cacheDirectory);
  if (!info.isDirectory() || info.isSymbolicLink()) {
    throw new Error(
      "The Demerzel cache path must be a real directory, not a symlink.",
    );
  }
  await chmod(cacheDirectory, 0o700);
  return cacheDirectory;
}

async function verifiedBinary(asset) {
  const stagingDirectory = await mkdtemp(join(tmpdir(), "demerzel-bun-"));
  await chmod(stagingDirectory, 0o700);
  try {
    const manifestPath = await downloadAsset(
      "manifest.json",
      join(stagingDirectory, "manifest.json"),
    );
    const bundlePath = await downloadAsset(
      "manifest.sigstore.json",
      join(stagingDirectory, "manifest.sigstore.json"),
    );
    verifyManifest(manifestPath, bundlePath);

    const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
    if (
      manifest.schemaVersion !== 1 ||
      manifest.repository !== repository ||
      manifest.tag !== tag ||
      manifest.version !== version
    ) {
      throw new Error(
        "The signed manifest does not match this Demerzel package version.",
      );
    }
    const expectedDigest = manifest.assets?.find(
      (entry) => entry.name === asset,
    )?.sha256;
    if (!/^[a-f0-9]{64}$/.test(expectedDigest ?? "")) {
      throw new Error(
        `The signed manifest does not contain a valid digest for ${asset}.`,
      );
    }

    const cacheDirectory = await ensureCacheDirectory();
    const binaryPath = join(cacheDirectory, asset);
    try {
      const cachedInfo = await lstat(binaryPath);
      if (!cachedInfo.isFile() || cachedInfo.isSymbolicLink()) {
        throw new Error("The cached Demerzel binary is not a regular file.");
      }
      if ((await sha256File(binaryPath)) === expectedDigest) return binaryPath;
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }

    const stagedBinary = await downloadAsset(
      asset,
      join(stagingDirectory, asset),
    );
    if ((await sha256File(stagedBinary)) !== expectedDigest) {
      throw new Error(
        `The downloaded ${asset} digest does not match the signed manifest.`,
      );
    }
    await chmod(stagedBinary, 0o700);
    await rename(stagedBinary, binaryPath);
    await chmod(binaryPath, 0o700);
    return binaryPath;
  } finally {
    await rm(stagingDirectory, { recursive: true, force: true });
  }
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length === 1 && (args[0] === "--help" || args[0] === "-h")) {
    printHelp();
    return;
  }
  if (args.length === 1 && (args[0] === "--version" || args[0] === "-V")) {
    process.stdout.write(`@amadmalik/demerzel ${version}\n`);
    return;
  }

  if (
    !/^2\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(
      version,
    )
  ) {
    throw new Error(
      `Invalid package version for the current release workflow: ${version}`,
    );
  }
  const binary = await verifiedBinary(platformAsset());
  const result = spawnSync(binary, args, {
    stdio: "inherit",
    env: process.env,
  });
  if (result.error) throw result.error;
  if (result.signal) {
    process.kill(process.pid, result.signal);
    return;
  }
  process.exitCode = result.status ?? 1;
}

main().catch((error) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`demerzel: ${message}`);
  process.exitCode = 1;
});
