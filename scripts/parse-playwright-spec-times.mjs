#!/usr/bin/env node

import { readFileSync } from "node:fs";

const usage = `Usage: scripts/parse-playwright-spec-times.mjs [playwright-json ...]

Aggregates Playwright JSON reporter result durations by spec file.
Reads stdin when no file paths are provided.

Output columns:
  spec_file<TAB>seconds<TAB>tests<TAB>attempts
`;

const args = process.argv.slice(2);
if (args.includes("--help") || args.includes("-h")) {
  process.stdout.write(usage);
  process.exit(0);
}

const inputs = args.length > 0 ? args : ["-"];
const totals = new Map();

function durationMs(value) {
  return typeof value === "number" && Number.isFinite(value) && value > 0
    ? value
    : 0;
}

function addTiming(file, test) {
  if (!file || !test || typeof test !== "object") {
    return;
  }

  let ms = 0;
  let attempts = 0;
  const results = Array.isArray(test.results) ? test.results : [];
  for (const result of results) {
    const resultMs = durationMs(result?.duration);
    if (result?.status === "skipped" && resultMs === 0) {
      continue;
    }
    ms += resultMs;
    attempts += 1;
  }

  if (results.length === 0) {
    ms += durationMs(test.duration);
    attempts = ms > 0 ? 1 : 0;
  }

  if (ms === 0 && attempts === 0) {
    return;
  }

  const current = totals.get(file) || { ms: 0, tests: 0, attempts: 0 };
  current.ms += ms;
  current.tests += 1;
  current.attempts += attempts;
  totals.set(file, current);
}

function walkSuite(suite, inheritedFile) {
  if (!suite || typeof suite !== "object") {
    return;
  }

  const suiteFile = suite.file || inheritedFile;
  const specs = Array.isArray(suite.specs) ? suite.specs : [];
  for (const spec of specs) {
    const file = spec?.file || suiteFile;
    const tests = Array.isArray(spec?.tests) ? spec.tests : [];
    for (const test of tests) {
      addTiming(file, test);
    }
  }

  const childSuites = Array.isArray(suite.suites) ? suite.suites : [];
  for (const child of childSuites) {
    walkSuite(child, suiteFile);
  }
}

for (const input of inputs) {
  const raw = input === "-" ? readFileSync(0, "utf8") : readFileSync(input, "utf8");
  const report = JSON.parse(raw);
  const suites = Array.isArray(report.suites) ? report.suites : [];
  for (const suite of suites) {
    walkSuite(suite, suite.file);
  }
}

const rows = [...totals.entries()].sort(([fileA, totalA], [fileB, totalB]) => {
  if (totalB.ms !== totalA.ms) {
    return totalB.ms - totalA.ms;
  }
  return fileA.localeCompare(fileB);
});

for (const [file, total] of rows) {
  const seconds = (total.ms / 1000).toFixed(3);
  process.stdout.write(`${file}\t${seconds}\t${total.tests}\t${total.attempts}\n`);
}
