// PREPARED ONLY. Set this literal to the independently authenticated device ID.
// No upload/start/network mutation is authorized by this file's presence.
let EXPECTED_ID = "CONFIGURE_DEVICE_ID";
let STORAGE_KEY = "net_restore";
let saved = null;
let phase = "idle";
let lastError = null;
let timer = null;
let deadline = 0;
let rpcPending = false;
let generation = 0;

function ipv4(value) {
  if (typeof value !== "string") return false;
  let parts = value.split(".");
  if (parts.length !== 4) return false;
  for (let i = 0; i < 4; i++) {
    let n = Number(parts[i]);
    if (n < 0 || n > 255 || Math.floor(n) !== n || String(n) !== parts[i]) return false;
  }
  return true;
}

function netmask(value) {
  if (!ipv4(value) || value === "0.0.0.0") return false;
  let parts = value.split(".");
  let ended = false;
  let allowed = [0, 128, 192, 224, 240, 248, 252, 254, 255];
  for (let i = 0; i < 4; i++) {
    let n = Number(parts[i]);
    if (allowed.indexOf(n) < 0 || (ended && n !== 0)) return false;
    if (n !== 255) ended = true;
  }
  return true;
}

function snapshotOK(sta) {
  if (sta === null || typeof sta !== "object") return false;
  if (Object.keys(sta).length !== 5 || sta.ipv4mode !== "static") return false;
  if (!ipv4(sta.ip) || !netmask(sta.netmask)) return false;
  if (sta.gw !== null && !ipv4(sta.gw)) return false;
  if (sta.nameserver !== null && !ipv4(sta.nameserver)) return false;
  return true;
}

function snapshot(sta) {
  if (sta === null || typeof sta !== "object") return null;
  return {ipv4mode: sta.ipv4mode, ip: sta.ip, netmask: sta.netmask,
    gw: sta.gw, nameserver: sta.nameserver};
}

function sameSnapshot(a, b) {
  return snapshotOK(a) && snapshotOK(b) && a.ipv4mode === b.ipv4mode &&
    a.ip === b.ip && a.netmask === b.netmask && a.gw === b.gw &&
    a.nameserver === b.nameserver;
}

function identityOK() {
  try {
    return EXPECTED_ID !== "CONFIGURE_DEVICE_ID" &&
      Shelly.getDeviceInfo().id === EXPECTED_ID;
  } catch (e) {
    return false;
  }
}

function recordOK(value) {
  return value !== null && typeof value === "object" &&
    Object.keys(value).length === 4 && value.version === 1 &&
    value.device === EXPECTED_ID && typeof value.pending === "boolean" &&
    snapshotOK(value.sta);
}

function persist(value) {
  try {
    let text = JSON.stringify(value);
    if (!recordOK(value) || text.length > 1024) return false;
    Script.storage.setItem(STORAGE_KEY, text);
    return Script.storage.getItem(STORAGE_KEY) === text;
  } catch (e) {
    return false;
  }
}

function recoveryStatus() {
  return {device: EXPECTED_ID, phase: phase,
    pending: saved !== null && saved.pending, error: lastError};
}

function retryRestore(delay, error) {
  lastError = error;
  phase = "restoring";
  try {
    timer = Timer.set(delay, false, retryFired, generation);
    if (timer === null || timer === undefined) throw "timer";
  } catch (e) {
    phase = "fault";
    lastError = "retry_timer_failed";
  }
}

function retryFired(token) {
  if (token === generation && phase === "restoring") restoreAttempt();
}

function expiryFired(token) {
  if (token === generation && phase === "armed") restoreAttempt();
}

function restoreReply(result, errorCode) {
  rpcPending = false;
  if (errorCode !== 0) {
    retryRestore(5000, "wifi_rpc_failed");
    return;
  }
  if (result !== null && result !== undefined && result.restart_required) {
    phase = "fault";
    lastError = "restart_required";
    return;
  }
  // SetConfig stores first, then reconnects. Verify runtime IP on a later tick.
  retryRestore(1000, null);
}

function restoreAttempt() {
  timer = null;
  if (saved === null || !saved.pending || rpcPending) return;
  phase = "restoring"; // Confirmation cannot cancel an in-flight restore.
  if (!identityOK()) {
    phase = "fault";
    lastError = "device_id_mismatch";
    return;
  }
  try {
    let wifi = Shelly.getComponentConfig("wifi");
    let status = Shelly.getComponentStatus("wifi");
    if (wifi === null || status === null) {
      retryRestore(5000, "wifi_state_unavailable");
      return;
    }
    if (sameSnapshot(snapshot(wifi.sta), saved.sta)) {
      if (status.sta_ip === saved.sta.ip) {
        phase = "restored";
        lastError = null;
      } else {
        retryRestore(1000, "waiting_for_old_ip");
      }
      return;
    }
    rpcPending = true;
    Shelly.call("Wifi.SetConfig", {config: {sta: snapshot(saved.sta)}}, restoreReply);
  } catch (e) {
    rpcPending = false;
    retryRestore(5000, "wifi_state_failed");
  }
}

function armRestore(milliseconds) {
  if (!identityOK() || phase === "fault" || (saved !== null && saved.pending)) return false;
  if (typeof milliseconds !== "number" || milliseconds < 30000 ||
      milliseconds > 600000 || Math.floor(milliseconds) !== milliseconds) return false;
  let next = null;
  try {
    let wifi = Shelly.getComponentConfig("wifi");
    if (wifi === null || wifi.sta === undefined || !wifi.sta.enable) return false;
    next = {version: 1, device: EXPECTED_ID, pending: true, sta: snapshot(wifi.sta)};
    if (!recordOK(next)) return false;
  } catch (e) {
    lastError = "snapshot_failed";
    return false;
  }
  if (!persist(next)) {
    lastError = "storage_failed";
    return false;
  }
  saved = next;
  generation = generation + 1;
  phase = "armed";
  lastError = null;
  try {
    deadline = Shelly.getUptimeMs() + milliseconds;
    timer = Timer.set(milliseconds, false, expiryFired, generation);
    if (timer === null || timer === undefined) throw "timer";
  } catch (e) {
    phase = "fault";
    lastError = "arm_timer_failed";
    return false;
  }
  return true;
}

function confirmRestore() {
  if (!identityOK() || phase !== "armed" || saved === null || !saved.pending) return false;
  // Reject a late confirmation even if the expired timer callback is queued.
  try {
    if (Shelly.getUptimeMs() >= deadline) return false;
  } catch (e) {
    lastError = "uptime_failed";
    return false;
  }
  let next = {version: 1, device: EXPECTED_ID, pending: false, sta: snapshot(saved.sta)};
  if (!persist(next)) {
    lastError = "storage_failed";
    return false;
  }
  saved = next;
  generation = generation + 1;
  phase = "confirmed";
  lastError = null;
  // A queued callback also observes pending=false, even if clear is too late.
  try {
    if (timer !== null) Timer.clear(timer);
  } catch (e) {
    lastError = "timer_clear_failed";
  }
  timer = null;
  return true;
}

function acknowledgeRestored() {
  if (!identityOK() || phase !== "restored" || saved === null ||
      !saved.pending || rpcPending) return false;
  try {
    let wifi = Shelly.getComponentConfig("wifi");
    let status = Shelly.getComponentStatus("wifi");
    if (wifi === null || status === null || status.sta_ip !== saved.sta.ip ||
        !sameSnapshot(snapshot(wifi.sta), saved.sta)) return false;
  } catch (e) {
    lastError = "wifi_state_failed";
    return false;
  }
  let next = {version: 1, device: EXPECTED_ID, pending: false, sta: snapshot(saved.sta)};
  if (!persist(next)) {
    lastError = "storage_failed";
    return false;
  }
  saved = next;
  generation = generation + 1;
  phase = "acknowledged";
  lastError = null;
  return true;
}

function recoveryBoot() {
  if (!identityOK()) {
    phase = "fault";
    lastError = "device_id_mismatch";
    return;
  }
  try {
    let text = Script.storage.getItem(STORAGE_KEY);
    if (text === null) return;
    if (typeof text !== "string" || text.length > 1024) throw "record";
    let value = JSON.parse(text);
    if (!recordOK(value)) throw "record";
    saved = value;
  } catch (e) {
    phase = "fault";
    lastError = "invalid_record";
    return;
  }
  if (saved.pending) restoreAttempt();
  else phase = "confirmed";
}

recoveryBoot();
