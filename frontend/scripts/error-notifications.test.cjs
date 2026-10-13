const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { stripTypeScriptTypes } = require('node:module');

function harness() {
 const window = new EventTarget();
 const errors = []; const unauthorized = [];
 window.addEventListener('trynas:error-notification', event => errors.push(event.detail));
 window.addEventListener('trynas:unauthorized', event => unauthorized.push(event));
 let fetchImpl; let uploadStatus = 413;
 class XHR {
  upload = {};
  open() {}
  send() { this.status = uploadStatus; this.responseText = 'quota exceeded'; this.onload(); }
  abort() { this.onabort(); }
 }
 const context = {window, Event, CustomEvent, Error, FormData, XMLHttpRequest:XHR, process:{env:{NODE_ENV:'development'}}, fetch:(...args)=>fetchImpl(...args)};
 function load(file, requireFn) {
  const module = {exports:{}};
  const source = fs.readFileSync(path.join(__dirname,'../modules/shared',file), 'utf8');
  const code = stripTypeScriptTypes(source, {mode:'transform'})
    .replace(/^import .*;$/gm, 'const { reportError } = require("./error-notifications");')
    .replace(/^export /gm, '')
    + (file === 'api.ts' ? '\nmodule.exports = {api, uploadFiles, ApiError};' : '\nmodule.exports = {reportError, errorNotificationText};');
  vm.runInNewContext(code, {...context,module,exports:module.exports,require:requireFn});
  return module.exports;
 }
 const notifications=load('error-notifications.ts');
 const api=load('api.ts',()=>notifications);
 return {api,errors,unauthorized,setFetch:value=>{fetchImpl=value},setUploadStatus:value=>{uploadStatus=value}};
}

test('API failures notify while preserving rejection; normal signed-out probes and cancellation stay quiet',async()=>{
 const h=harness();
 h.setFetch(async()=>({ok:false,status:403,text:async()=>'folder permission denied\n'}));
 await assert.rejects(h.api.api('/api/files'), {status:403});
 assert.deepEqual(h.errors,['folder permission denied']);
 h.errors.length=0;
 h.setFetch(async()=>{throw new Error('Cannot reach server')});
 await assert.rejects(h.api.api('/api/files'));
 assert.deepEqual(h.errors,['Cannot reach server']);
 h.errors.length=0;
 h.setFetch(async()=>{const error=new Error('aborted');error.name='AbortError';throw error});
 await assert.rejects(h.api.api('/api/files'));
 assert.deepEqual(h.errors,[]);
 h.setFetch(async()=>({ok:false,status:401,text:async()=>'unauthorized'}));
 await assert.rejects(h.api.api('/api/me',{quietUnauthorized:true}));
 assert.equal(h.errors.length,0);assert.equal(h.unauthorized.length,0);
 await assert.rejects(h.api.api('/api/files'));
 assert.deepEqual(h.errors,['Your session expired. Please sign in again.']);
 assert.equal(h.unauthorized.length,1);
});

test('XHR upload errors notify, cancellations do not, and expired upload sessions sign out',async()=>{
 const h=harness();
 await assert.rejects(h.api.uploadFiles(new FormData(),()=>{}),{status:413});
 assert.deepEqual(h.errors,['quota exceeded']);
 h.errors.length=0;
 await assert.rejects(h.api.uploadFiles(new FormData(),()=>{},AbortSignal.abort()));
 assert.deepEqual(h.errors,[]);
 h.setUploadStatus(401);
 await assert.rejects(h.api.uploadFiles(new FormData(),()=>{}),{status:401});
 assert.deepEqual(h.errors,['Your session expired. Please sign in again.']);
 assert.equal(h.unauthorized.length,1);
});
