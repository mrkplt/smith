#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const docsRoot = path.join(root, "docs");
const markdownPattern = /\[[^\]]+\]\(([^)]+)\)/g;

const kindConfig = {
  planning: {
    base: path.join(docsRoot, "planning"),
    statuses: new Set(["proposed", "approved", "archived"]),
    required: new Set(["id", "title", "status", "doc_type", "prd_mode", "target_branch", "owner"]),
    docTypes: new Set(["feature", "architecture", "workflow", "runbook", "release-note", "adr", "other"]),
    prdModes: new Set(["none", "generate", "update"]),
  },
  prds: {
    base: path.join(docsRoot, "prds"),
    statuses: new Set(["draft", "approved", "archived"]),
    required: new Set(["id", "title", "status", "doc_type", "source_doc", "target_branch", "owner"]),
    docTypes: new Set(["prd"]),
    prdModes: null,
  },
};

const idPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

function parseFrontmatter(filePath) {
  const text = fs.readFileSync(filePath, "utf8");
  const lines = text.split(/\r?\n/);
  if (lines.length === 0 || lines[0].trim() !== "---") {
    return [{}, [`${filePath}: missing frontmatter block`]];
  }

  const data = {};
  let index = 1;
  while (index < lines.length) {
    const line = lines[index];
    if (line.trim() === "---") {
      return [data, []];
    }
    if (!line.trim()) {
      index += 1;
      continue;
    }
    if (line.startsWith("  - ") || line.startsWith("\t- ")) {
      return [{}, [`${filePath}: invalid top-level list item in frontmatter`]];
    }
    if (!line.includes(":")) {
      return [{}, [`${filePath}: invalid frontmatter line '${line}'`]];
    }

    const [rawKey, ...rest] = line.split(":");
    const key = rawKey.trim();
    const value = rest.join(":").trim();
    if (!key) {
      return [{}, [`${filePath}: invalid empty frontmatter key`]];
    }

    if (!value) {
      const items = [];
      index += 1;
      while (index < lines.length) {
        const nested = lines[index];
        if (nested.trim() === "---") {
          data[key] = items;
          return [data, []];
        }
        if (!nested.trim()) {
          index += 1;
          continue;
        }
        if (nested.startsWith("  - ")) {
          items.push(nested.slice(4).trim());
          index += 1;
          continue;
        }
        break;
      }
      data[key] = items;
      continue;
    }

    if (value.startsWith("[") && value.endsWith("]")) {
      const inner = value.slice(1, -1).trim();
      data[key] = inner ? inner.split(",").map((item) => item.trim().replace(/^['"]|['"]$/g, "")) : [];
    } else if (value === "true" || value === "false") {
      data[key] = value === "true";
    } else {
      data[key] = value.replace(/^['"]|['"]$/g, "");
    }
    index += 1;
  }

  return [{}, [`${filePath}: missing closing frontmatter delimiter`]];
}

function listMarkdownFiles(baseDir) {
  if (!fs.existsSync(baseDir)) {
    return [];
  }
  const files = [];
  const stack = [baseDir];
  while (stack.length > 0) {
    const current = stack.pop();
    for (const entry of fs.readdirSync(current, { withFileTypes: true })) {
      const fullPath = path.join(current, entry.name);
      if (entry.isDirectory()) {
        stack.push(fullPath);
        continue;
      }
      if (entry.isFile() && entry.name.endsWith(".md")) {
        files.push(fullPath);
      }
    }
  }
  files.sort();
  return files;
}

function validateFile(kind, filePath) {
  const cfg = kindConfig[kind];
  const [frontmatter, parseErrors] = parseFrontmatter(filePath);
  if (parseErrors.length > 0) {
    return parseErrors;
  }

  const errors = [];
  const rel = path.relative(cfg.base, filePath);
  const parts = rel.split(path.sep);
  if (parts.length < 2) {
    errors.push(`${filePath}: expected file inside status subdirectory`);
    return errors;
  }

  const statusDir = parts[0];
  if (!cfg.statuses.has(statusDir)) {
    errors.push(`${filePath}: unknown lifecycle status directory '${statusDir}'`);
    return errors;
  }

  for (const field of cfg.required) {
    if (!frontmatter[field]) {
      errors.push(`${filePath}: missing required frontmatter field '${field}'`);
    }
  }

  const status = String(frontmatter.status ?? "");
  if (status && status !== statusDir) {
    errors.push(`${filePath}: frontmatter status '${status}' does not match directory '${statusDir}'`);
  }

  const docId = String(frontmatter.id ?? "");
  if (docId && !idPattern.test(docId)) {
    errors.push(`${filePath}: id must be lowercase kebab-case`);
  }

  const title = String(frontmatter.title ?? "");
  if (title && title.length < 8) {
    errors.push(`${filePath}: title should be descriptive (minimum 8 characters)`);
  }

  const docType = String(frontmatter.doc_type ?? "");
  if (docType && !cfg.docTypes.has(docType)) {
    errors.push(`${filePath}: unsupported doc_type '${docType}'`);
  }

  const owner = String(frontmatter.owner ?? "");
  if (Object.hasOwn(frontmatter, "owner") && !owner.trim()) {
    errors.push(`${filePath}: owner must be non-empty`);
  }

  const targetBranch = String(frontmatter.target_branch ?? "");
  if (targetBranch && targetBranch.startsWith("refs/") && targetBranch.includes("/")) {
    errors.push(`${filePath}: target_branch should be a branch name, not a full ref`);
  }

  if (kind === "planning") {
    const prdMode = String(frontmatter.prd_mode ?? "");
    if (prdMode && !cfg.prdModes.has(prdMode)) {
      errors.push(`${filePath}: unsupported prd_mode '${prdMode}'`);
    }
    if (status === "approved" && ["feature", "architecture", "workflow"].includes(docType) && prdMode === "none") {
      errors.push(`${filePath}: approved ${docType} docs must declare prd_mode 'generate' or 'update'`);
    }
    if (status !== "approved" && prdMode === "update") {
      errors.push(`${filePath}: prd_mode 'update' is only valid for approved planning docs`);
    }
  }

  if (kind === "prds") {
    if (docType && docType !== "prd") {
      errors.push(`${filePath}: PRD documents must use doc_type 'prd'`);
    }
    const sourceDoc = String(frontmatter.source_doc ?? "");
    if (sourceDoc && !sourceDoc.startsWith("docs/planning/")) {
      errors.push(`${filePath}: source_doc must point to docs/planning/`);
    }
  }

  return errors;
}

function validateLocalLinks() {
  const errors = [];
  for (const filePath of listMarkdownFiles(docsRoot)) {
    const text = fs.readFileSync(filePath, "utf8");
    for (const match of text.matchAll(markdownPattern)) {
      const value = match[1].trim();
      if (!value || value.includes("://") || value.startsWith("#") || value.startsWith("mailto:")) {
        continue;
      }
      const localPath = value.split("#", 1)[0];
      const resolved = path.resolve(path.dirname(filePath), localPath);
      if (!fs.existsSync(resolved)) {
        errors.push(`${filePath}: broken local link '${value}'`);
      }
    }
  }
  return errors;
}

const files = [];
for (const [kind, cfg] of Object.entries(kindConfig)) {
  for (const filePath of listMarkdownFiles(cfg.base)) {
    files.push([kind, filePath]);
  }
}

const errors = [];
for (const [kind, filePath] of files) {
  errors.push(...validateFile(kind, filePath));
}
errors.push(...validateLocalLinks());

if (errors.length > 0) {
  console.log("Lifecycle documentation validation failed:");
  for (const error of errors) {
    console.log(`  - ${error}`);
  }
  process.exit(1);
}

console.log(`Lifecycle documentation validation passed for ${files.length} file(s).`);
