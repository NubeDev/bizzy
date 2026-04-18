// device_summary.js — aggregates device status from the Rubix API.
function handle(params) {
  var resp = http.get(config.rubix_host + "/api/v1/devices", {
    headers: { "Authorization": "Bearer " + secrets.rubix_token }
  });

  if (resp.status !== 200) {
    return { error: "API returned " + resp.status + ": " + resp.body };
  }

  var data = resp.json;
  var devices = data.data || [];
  var online = 0;
  var offline = 0;

  for (var i = 0; i < devices.length; i++) {
    if (devices[i].online) {
      online++;
    } else {
      offline++;
    }
  }

  return {
    total: devices.length,
    online: online,
    offline: offline,
    summary: "Total: " + devices.length + ", Online: " + online + ", Offline: " + offline
  };
}
