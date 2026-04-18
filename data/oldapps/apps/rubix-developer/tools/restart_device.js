// restart_device.js — restart a device via the Rubix API.
function handle(params) {
  if (!params.id) {
    return { error: "device id is required" };
  }

  var baseUrl = config.rubix_host + "/api/v1/devices/" + params.id;
  var headers = { "Authorization": "Bearer " + secrets.rubix_token };

  // Check device exists first.
  var check = http.get(baseUrl, { headers: headers });
  if (check.status === 404) {
    return { error: "device " + params.id + " not found" };
  }

  // Set to offline (simulate restart).
  var resp = http.patch(baseUrl, { online: false }, { headers: headers });
  if (resp.status !== 200) {
    return { error: "failed to take device offline: " + resp.body };
  }

  // Set back to online.
  resp = http.patch(baseUrl, { online: true }, { headers: headers });
  if (resp.status !== 200) {
    return { error: "failed to bring device back online: " + resp.body };
  }

  log.info("device " + params.id + " restarted successfully");
  return { message: "device " + params.id + " restarted", status: "online" };
}
