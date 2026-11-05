#!/usr/bin/env node
// Runs the actual prepared script in a fake Shelly runtime; no device access.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');

const original = fs.readFileSync(path.join(__dirname, 'shelly-network-recovery.js'), 'utf8');
const id = 'shelly-fixture-a';
const source = original.replace('let EXPECTED_ID = "CONFIGURE_DEVICE_ID";', 'let EXPECTED_ID = "' + id + '";');
const oldSta = {ipv4mode: 'static', ip: '192.0.2.28', netmask: '255.255.255.0', gw: '192.0.2.1', nameserver: '192.0.2.1'};
const newSta = {...oldSta, ip: '198.51.100.28', gw: '198.51.100.1', nameserver: '198.51.100.1'};
const clone = v => JSON.parse(JSON.stringify(v));
let cases = 0;

function runtime(options = {}) {
  const storage = options.storage || new Map();
  const state = {
    now: 0, timers: new Map(), serial: 0, calls: [], pending: [], storage,
    storageFail: false, timerFail: false, clearFail: false,
    device: options.device || id,
    wifi: {sta: {...clone(options.sta || oldSta), enable: true, ssid: 'PRIVATE_SENTINEL', pass: 'PRIVATE_SENTINEL'}, sta1: {enable: false}},
    status: {sta_ip: (options.sta || oldSta).ip},
  };
  const context = vm.createContext({
    Script: {storage: {
      getItem: key => storage.has(key) ? storage.get(key) : null,
      setItem: (key, value) => {if (state.storageFail) throw Error('PRIVATE_SENTINEL'); storage.set(key, value);},
    }},
    Timer: {
      set: (ms, repeat, fn, userdata) => {
        assert.equal(repeat, false);
        if (state.timerFail) throw Error('PRIVATE_SENTINEL');
        const n = ++state.serial;
        state.timers.set(n, {at: state.now + ms, fn: () => fn(userdata)});
        assert(state.timers.size <= 2, 'unexpected timer accumulation');
        return n;
      },
      clear: n => {if (state.clearFail) throw Error('PRIVATE_SENTINEL'); return state.timers.delete(n);},
    },
    Shelly: {
      getDeviceInfo: () => ({id: state.device}),
      getUptimeMs: () => state.now,
      getComponentConfig: component => {assert.equal(component, 'wifi'); return clone(state.wifi);},
      getComponentStatus: component => {assert.equal(component, 'wifi'); return clone(state.status);},
      call: (method, params, cb) => {
        assert.equal(method, 'Wifi.SetConfig', 'forbidden RPC');
        assert.deepEqual(Object.keys(params), ['config']);
        assert.deepEqual(Object.keys(params.config), ['sta']);
        assert.deepEqual(Object.keys(params.config.sta).sort(), Object.keys(oldSta).sort());
        assert(!JSON.stringify(params).includes('PRIVATE_SENTINEL'));
        state.calls.push(clone(params));
        state.pending.push({params: clone(params), cb});
        assert.equal(state.pending.length, 1, 'overlapping restore RPCs');
      },
    },
  });
  vm.runInContext(options.unconfigured ? original : source, context);
  const api = {
    state,
    arm: ms => context.armRestore(ms),
    confirm: () => context.confirmRestore(),
    acknowledge: () => context.acknowledgeRestored(),
    status: () => clone(context.recoveryStatus()),
    change: () => {state.wifi.sta = {...state.wifi.sta, ...clone(newSta)}; state.status.sta_ip = newSta.ip;},
    advance: ms => {
      const end = state.now + ms;
      while (true) {
        const due = [...state.timers].filter(([, t]) => t.at <= end).sort((a, b) => a[1].at - b[1].at)[0];
        if (!due) break;
        state.now = due[1].at; state.timers.delete(due[0]); due[1].fn();
      }
      state.now = end;
    },
    reply: (error = 0, result = {}, apply = true) => {
      const op = state.pending.shift(); assert(op, 'missing RPC');
      if (!error && apply) {
        state.wifi.sta = {...state.wifi.sta, ...op.params.config.sta};
        state.status.sta_ip = op.params.config.sta.ip;
      }
      op.cb(result, error, 'PRIVATE_SENTINEL');
    },
  };
  return api;
}

function test(name, fn) {
  fn(); cases++; console.log('passed: ' + name);
}

test('expiry restores only saved IPv4 and preserves credentials', () => {
  const r = runtime(); assert(r.arm(30000));
  const saved = JSON.parse(r.state.storage.get('net_restore'));
  assert(saved.pending); assert.deepEqual(saved.sta, oldSta);
  assert(!JSON.stringify(saved).includes('PRIVATE_SENTINEL'));
  r.change(); r.advance(30000); assert.equal(r.state.calls.length, 1);
  assert(!r.confirm(), 'cannot confirm in flight');
  r.reply(); r.advance(1000); assert.equal(r.status().phase, 'restored');
  assert(r.status().pending, 'retain pending marker until explicit retirement');
  assert.equal(r.state.wifi.sta.pass, 'PRIVATE_SENTINEL');
  assert.equal(r.state.wifi.sta.ssid, 'PRIVATE_SENTINEL');
});

test('boot pending immediately restores with no clock/network dependency', () => {
  const first = runtime(); assert(first.arm(120000)); first.change();
  const boot = runtime({storage: first.state.storage, sta: newSta});
  assert.equal(boot.state.calls.length, 1);
  boot.reply(); boot.advance(1000); assert.equal(boot.status().phase, 'restored');
  const again = runtime({storage: boot.state.storage});
  assert.equal(again.status().phase, 'restored'); assert.equal(again.state.calls.length, 0);
});

test('confirmation persists before timer cancellation and survives reboot', () => {
  const r = runtime(); assert(r.arm(30000)); r.change(); assert(r.confirm());
  assert.equal(JSON.parse(r.state.storage.get('net_restore')).pending, false);
  r.advance(60000); assert.equal(r.state.calls.length, 0);
  const boot = runtime({storage: r.state.storage, sta: newSta});
  assert.equal(boot.status().phase, 'confirmed'); assert.equal(boot.state.calls.length, 0);
});

test('queued expiry rejects late confirmation; queued callback after confirm is harmless', () => {
  const late = runtime(); assert(late.arm(30000)); late.change(); late.state.now = 30000;
  assert(!late.confirm()); late.advance(0); assert.equal(late.state.calls.length, 1);
  const good = runtime(); assert(good.arm(30000)); good.change();
  const queued = [...good.state.timers.values()][0].fn;
  assert(good.confirm()); queued(); assert.equal(good.state.calls.length, 0);
});

test('failed RPC retries without clearing pending or leaking firmware error', () => {
  const r = runtime(); assert(r.arm(30000)); r.change(); r.advance(30000);
  r.reply(500); assert.equal(r.status().error, 'wifi_rpc_failed');
  assert(!JSON.stringify(r.status()).includes('PRIVATE_SENTINEL'));
  assert(!r.confirm()); r.advance(5000); assert.equal(r.state.calls.length, 2);
  r.reply(); r.advance(1000); assert.equal(r.status().phase, 'restored');
});

test('runtime IP must follow saved config before declaring restored', () => {
  const r = runtime(); assert(r.arm(30000)); r.change(); r.advance(30000); r.reply(0, {}, false);
  r.state.wifi.sta = {...r.state.wifi.sta, ...clone(oldSta)};
  r.advance(1000); assert.equal(r.status().error, 'waiting_for_old_ip');
  assert.equal(r.state.calls.length, 1);
  r.state.status.sta_ip = oldSta.ip; r.advance(1000); assert.equal(r.status().phase, 'restored');
});

test('wrong identity, malformed records, extra fields and bad masks fail without writes', () => {
  for (const options of [{device: 'other'}, {unconfigured: true}]) {
    const r = runtime(options); assert(!r.arm(30000)); assert.equal(r.status().phase, 'fault');
  }
  const base = {version: 1, device: id, pending: true, sta: oldSta};
  const malformed = ['bad json', 'null', '[]', JSON.stringify({...base, device: 'other'}),
    JSON.stringify({...base, pending: 'true'}), JSON.stringify({...base, secret: 'PRIVATE_SENTINEL'}),
    JSON.stringify({...base, sta: {...oldSta, pass: 'PRIVATE_SENTINEL'}}),
    JSON.stringify({...base, sta: {...oldSta, ip: 'NaN.1.2.3'}}),
    JSON.stringify({...base, sta: {...oldSta, netmask: '255.0.255.0'}}), 'x'.repeat(1025)];
  for (const text of malformed) {
    const r = runtime({storage: new Map([['net_restore', text]])});
    assert.equal(r.status().phase, 'fault'); assert.equal(r.state.calls.length, 0); assert(!r.arm(30000));
  }
});

test('storage, timer, restart-required and confirmation failures remain conservative', () => {
  const storage = runtime(); storage.state.storageFail = true;
  assert(!storage.arm(30000)); assert.equal(storage.state.timers.size, 0);
  const timer = runtime(); timer.state.timerFail = true;
  assert(!timer.arm(30000)); assert(JSON.parse(timer.state.storage.get('net_restore')).pending);
  const confirmed = runtime(); assert(confirmed.arm(30000)); confirmed.change();
  confirmed.state.storageFail = true; assert(!confirmed.confirm());
  assert(JSON.parse(confirmed.state.storage.get('net_restore')).pending);
  confirmed.advance(30000); assert.equal(confirmed.state.calls.length, 1);
  const restart = runtime(); assert(restart.arm(30000)); restart.change(); restart.advance(30000);
  restart.reply(0, {restart_required: true}); assert.equal(restart.status().phase, 'fault'); assert(restart.status().pending);
  const clear = runtime(); assert(clear.arm(30000)); clear.change(); clear.state.clearFail = true;
  assert(clear.confirm()); clear.advance(30000); assert.equal(clear.state.calls.length, 0);
});

test('rearm, invalid timeout, DHCP snapshot and identity change are refused', () => {
  const r = runtime(); for (const ms of [0, 29999, 600001, 30000.5, NaN, '30000']) assert(!r.arm(ms));
  assert(r.arm(30000)); assert(!r.arm(30000)); r.change(); r.state.device = 'other';
  assert(!r.confirm()); r.advance(30000); assert.equal(r.state.calls.length, 0); assert.equal(r.status().phase, 'fault');
  const dhcp = runtime({sta: {...oldSta, ipv4mode: 'dhcp'}}); assert(!dhcp.arm(30000));
});

test('restored acknowledgement closes trials and permits repeated expiry, boot and confirmation', () => {
  const r = runtime(); assert(!r.acknowledge()); assert(r.arm(30000)); assert(!r.acknowledge());
  r.change(); r.advance(30000); assert(!r.acknowledge());
  r.reply(); r.advance(1000); assert.equal(r.status().phase, 'restored');
  assert(r.acknowledge()); assert.equal(r.status().phase, 'acknowledged');
  assert.equal(JSON.parse(r.state.storage.get('net_restore')).pending, false);
  assert(!r.acknowledge()); assert(r.arm(30000)); r.change();
  const boot = runtime({storage: r.state.storage, sta: newSta});
  boot.reply(); boot.advance(1000); assert(boot.acknowledge()); assert(boot.arm(30000));
  boot.change(); assert(boot.confirm()); boot.advance(30000); assert.equal(boot.state.calls.length, 1);
});

test('restored acknowledgement refuses stale runtime, config, identity and persistence failure', () => {
  const r = runtime(); assert(r.arm(30000)); r.change(); r.advance(30000); r.reply(); r.advance(1000);
  r.state.status.sta_ip = newSta.ip; assert(!r.acknowledge());
  r.state.status.sta_ip = oldSta.ip; r.state.wifi.sta.gw = newSta.gw; assert(!r.acknowledge());
  r.state.wifi.sta.gw = oldSta.gw; r.state.device = 'other'; assert(!r.acknowledge());
  r.state.device = id; r.state.storageFail = true; assert(!r.acknowledge());
  assert(JSON.parse(r.state.storage.get('net_restore')).pending);
  r.state.storageFail = false; assert(r.acknowledge());
});

test('silent RPC remains pending without overlapping writes or false success', () => {
  const r = runtime(); assert(r.arm(30000)); r.change(); r.advance(30000);
  r.advance(300000); assert.equal(r.state.calls.length, 1); assert(r.status().pending);
  assert.equal(r.status().phase, 'restoring'); assert(!r.confirm()); assert(!r.acknowledge());
  const boot = runtime({storage: r.state.storage, sta: newSta});
  assert.equal(boot.state.calls.length, 1); boot.reply(); boot.advance(1000);
  assert.equal(boot.status().phase, 'restored');
});

test('stale queued expiry cannot restore a newly armed generation', () => {
  const r = runtime(); assert(r.arm(30000));
  const oldCallback = [...r.state.timers.values()][0].fn;
  r.change(); assert(r.confirm()); assert(r.arm(120000));
  oldCallback(); assert.equal(r.status().phase, 'armed'); assert.equal(r.state.calls.length, 0);
  const failedClear = runtime(); assert(failedClear.arm(30000)); failedClear.change();
  failedClear.state.clearFail = true; assert(failedClear.confirm());
  assert(failedClear.arm(120000)); failedClear.change(); failedClear.advance(30000);
  assert.equal(failedClear.status().phase, 'armed'); assert.equal(failedClear.state.calls.length, 0);
});

assert(!/Shelly\.call\("(?!Wifi\.SetConfig)/.test(source));
assert(!/console\.|print\(|Date\.|Promise|async |=>|\bclass\b/.test(source));
console.log(cases + ' offline recovery scenarios passed; no firmware execution or live recovery claimed');
