const assert = require('node:assert/strict');
const { createRequire } = require('node:module');
const mammothRequire = createRequire(require.resolve('mammoth'));
const argparseRequire = createRequire(mammothRequire.resolve('argparse'));
const { sprintf } = argparseRequire('sprintf-js');
// Fail before excluding the upstream advisory from audit if the local patch is absent.
// GHSA-hp3w-g68c-fv3c has no upstream patched release; the pinned pnpm patch
// clamps numeric precision to ECMAScript limits without aborting consumers.
assert.equal(sprintf('%.2f', 1.25), '1.25');
assert.equal(sprintf('%.3g', 1.25), '1.25');
for (const conversion of ['e', 'f', 'g']) {
  for (const precision of ['101', '9999999999999999999999999999999999999999999']) {
    const result = sprintf(`%.${precision}${conversion}`, 1.25);
    assert.equal(typeof result, 'string');
    assert.ok(result.length <= 110);
  }
}
assert.equal(sprintf('%.0g', 1.25), '1');
console.log('Verified local sprintf-js security patch and ordinary formatting.');

// Registry audits cannot identify a local patch. Exclude only the tested advisory
// in this command, leaving ordinary pnpm audit and its configuration untouched.
const { spawnSync } = require('node:child_process');
const audit = process.platform === 'win32'
  ? spawnSync('cmd.exe', ['/d', '/s', '/c', 'pnpm audit --json'], { encoding: 'utf8', shell: false })
  : spawnSync('pnpm', ['audit', '--json'], { encoding: 'utf8', shell: false });
if (audit.error || audit.status === null) throw audit.error || Error('Audit did not finish');
let report;
try { report = JSON.parse(audit.stdout); }
catch { console.error(audit.stdout, audit.stderr); process.exit(1); }
if (report.error) { console.error(report.error); process.exit(1); }
const advisories = Object.values(report.advisories || {});
const remaining = advisories.filter(advisory => advisory.github_advisory_id !== 'GHSA-hp3w-g68c-fv3c');
if (remaining.length || (audit.status !== 0 && advisories.length === 0)) {
  console.error(JSON.stringify(report, null, 2));
  process.exit(1);
}
console.log(`Frontend audit: no unpatched advisories; ${advisories.length - remaining.length} locally patched advisory verified.`);
