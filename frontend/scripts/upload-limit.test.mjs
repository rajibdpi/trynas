import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { proxyUploadLimitBytes } from '../upload-limit.mjs';

test('proxy reads saved configuration, explicit overrides and custom paths', () => {
 const root=mkdtempSync(join(tmpdir(),'trynas-proxy-limit-'));
 try {
  assert.equal(proxyUploadLimitBytes(root,{}),10*1024**3);
  writeFileSync(join(root,'.env'),'ADMIN_PASSWORD="secret"\nPROXY_UPLOAD_LIMIT_MB="128"\n');
  assert.equal(proxyUploadLimitBytes(root,{}),128*1024**2);
  assert.equal(proxyUploadLimitBytes(root,{PROXY_UPLOAD_LIMIT_MB:'256'}),256*1024**2);
  writeFileSync(join(root,'custom.env'),"PROXY_UPLOAD_LIMIT_MB='512'\n");
  assert.equal(proxyUploadLimitBytes(root,{TRYNAS_CONFIG_FILE:'custom.env'}),512*1024**2);
  for (const value of ['0','-1','10241','1.5','abc','1e3']) {
   assert.throws(()=>proxyUploadLimitBytes(root,{PROXY_UPLOAD_LIMIT_MB:value}));
   writeFileSync(join(root,'.env'),`PROXY_UPLOAD_LIMIT_MB="${value}"\n`);
   assert.throws(()=>proxyUploadLimitBytes(root,{}));
  }
 } finally {rmSync(root,{recursive:true,force:true});}
});
