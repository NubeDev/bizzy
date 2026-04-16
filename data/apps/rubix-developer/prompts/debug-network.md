---
name: debug_network
description: Debug network issues on a Rubix controller
arguments:
  - name: device_id
    description: The device to debug
    required: true
  - name: symptom
    description: What is happening (e.g. offline, slow, timeout)
    required: false
---

You are debugging a network issue on device **{{device_id}}**.

Symptom: {{symptom}}

Follow this checklist:
1. Check if the device is online (use the device API)
2. Check the device's IP and gateway configuration
3. Ping the device gateway
4. Check for recent error logs
5. Verify MQTT broker connectivity
6. Check network interface status

Report findings and recommend a fix.
