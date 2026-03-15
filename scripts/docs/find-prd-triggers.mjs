#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");

function git(...args) {
  return execFileSync("git", args, {
    cwd: root,
    encoding: "utf8",
  });
}

function parseFrontmatter(blob) {
  const lines = blob.split(/\r?\n/);
  if (lines.length === 0 || lines[0].trim() !== "---") {
    return {};
  }
  const data = {};
  for (let index = 1; index < lines.length; index += 1) {
    const line = lines[index];
    if (line.trim() === "---") {
      return data;
    }
    if (!line.includes(":")) {
      continue;
    }
    const [rawKey, ...rest] = line.split(":");
    data[rawKey.trim()] = rest.join(":").trim().replace(/^['"]|['"]$/g, "");
  }
  return {};
}

function show(ref, filePath) {
  try {
    return git("show", `${ref}:${filePath}`);
  } catch {
    return "";
  }
}

function listChanges(base, head) {
  const output = git("diff", "--name-status", "--find-renames", base, head, "--", "docs/planning");
  const changes = [];
  for (const line of output.split(/\r?\n/)) {
    if (!line.trim()) {
      continue;
    }
    const parts = line.split("\t");
    const status = parts[0];
    if (status.startsWith("R")) {
      changes.push([status, parts[1], parts[2]]);
    } else if (/^[AMC]/.test(status)) {
      changes.push([status, null, parts[1]]);
    }
  }
  return changes;
}

function triggerReason(statusCode, oldPath, newPath, base, head) {
  const newMeta = parseFrontmatter(show(head, newPath));
  if (Object.keys(newMeta).length === 0) {
    return null;
  }
  if (!newPath.includes("/approved/")) {
    return null;
  }
  if (newMeta.status !== "approved") {
    return null;
  }
  if (!["generate", "update"].includes(newMeta.prd_mode)) {
    return null;
  }

  if (statusCode.startsWith("R") && oldPath && oldPath.includes("/proposed/") && newPath.includes("/approved/")) {
    return "promotion";
  }
  if (statusCode.startsWith("A")) {
    return "new-approved-doc";
  }

  const oldMeta = parseFrontmatter(show(base, oldPath ?? newPath));
  if (oldMeta.status !== "approved") {
    return "status-change";
  }
  if (newMeta.prd_mode === "update") {
    return "approved-doc-updated";
  }
  return null;
}

if (process.argv.length !== 4) {
  console.error("usage: find-prd-triggers.mjs <base-ref> <head-ref>");
  process.exit(2);
}

const [, , base, head] = process.argv;
const events = [];

for (const [statusCode, oldPath, newPath] of listChanges(base, head)) {
  const reason = triggerReason(statusCode, oldPath, newPath, base, head);
  if (!reason) {
    continue;
  }
  const meta = parseFrontmatter(show(head, newPath));
  events.push({
    reason,
    doc_path: newPath,
    doc_id: meta.id ?? "",
    title: meta.title ?? "",
    prd_mode: meta.prd_mode ?? "",
    target_branch: meta.target_branch ?? "",
    linked_prd: meta.linked_prd ?? "",
    source_path: oldPath ?? newPath,
  });
}

console.log(JSON.stringify(events, null, 2));
