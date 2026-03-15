#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const docsRoot = path.join(root, "docs");
const configPath = path.join(root, "zensical.toml");
const publicExcludes = new Set([
  "docs/planning",
  "docs/prds",
  "docs/docs-to-prd-lifecycle.md",
]);

function shouldExclude(sourcePath) {
  const rel = path.relative(root, sourcePath).split(path.sep).join("/");
  for (const item of publicExcludes) {
    if (rel === item || rel.startsWith(`${item}/`)) {
      return true;
    }
  }
  return false;
}

function copyPublicDocs(sourceDir, targetDir) {
  for (const entry of fs.readdirSync(sourceDir, { withFileTypes: true })) {
    const sourcePath = path.join(sourceDir, entry.name);
    if (shouldExclude(sourcePath)) {
      continue;
    }
    const targetPath = path.join(targetDir, entry.name);
    if (entry.isDirectory()) {
      fs.mkdirSync(targetPath, { recursive: true });
      copyPublicDocs(sourcePath, targetPath);
      continue;
    }
    fs.mkdirSync(path.dirname(targetPath), { recursive: true });
    fs.copyFileSync(sourcePath, targetPath);
  }
}

const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), "smith-public-docs-"));

try {
  const tempDocs = path.join(tempRoot, "docs");
  fs.mkdirSync(tempDocs, { recursive: true });
  copyPublicDocs(docsRoot, tempDocs);
  fs.copyFileSync(configPath, path.join(tempRoot, "zensical.toml"));

  execFileSync(path.join(root, "scripts", "docs", "docs-container.sh"), [tempRoot, "build"], {
    cwd: root,
    stdio: "inherit",
  });

  const siteDir = path.join(tempRoot, "site");
  const targetSite = path.join(root, "site");
  fs.rmSync(targetSite, { recursive: true, force: true });
  fs.cpSync(siteDir, targetSite, { recursive: true });
} finally {
  fs.rmSync(tempRoot, { recursive: true, force: true });
}

console.log("Public docs site build completed.");
