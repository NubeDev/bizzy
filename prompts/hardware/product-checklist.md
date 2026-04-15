# Hardware Product Checklist

Use this checklist to ensure all aspects of a hardware product specification are complete.

---

## 📋 Product Definition

- [ ] Product name defined (e.g., IO22, IO16)
- [ ] Product family selected (ACBM / ACBL)
- [ ] Target use cases identified
- [ ] Deployment mode determined (side plugin / remote I/O / both)

---

## 🔧 Technical Specifications

### Hardware Platform
- [ ] Processor selected (STM32 / ESP32)
- [ ] Power requirements defined
  - [ ] Voltage range specified
  - [ ] Power consumption estimated
  - [ ] Power source identified (host-powered / external / PoE)

### Connectivity
- [ ] Communication interface defined (RS485 / Ethernet / WiFi / LoRa)
- [ ] Isolation requirements specified (ISO / non-ISO)
- [ ] Protocol support confirmed (BACnet / Modbus / both)
- [ ] Wireless options (if applicable):
  - [ ] WiFi requirements
  - [ ] LoRa requirements
  - [ ] Antenna type

---

## 🔌 I/O Configuration

### Digital I/O
- [ ] Digital Input (DI) count: _____
  - [ ] Voltage range specified
  - [ ] Input type (dry contact / voltage / both)
- [ ] Digital Output (DO) count: _____
  - [ ] Output type (solid-state / relay)
  - [ ] Current rating per channel
- [ ] Relay Output count (if applicable): _____
  - [ ] Voltage rating
  - [ ] Current rating
  - [ ] Contact type (SPDT / SPST)

### Analog I/O
- [ ] Universal Input (UI) count: _____
  - [ ] Supported input types (0-10V / 4-20mA / 10K thermistor / PT1000)
- [ ] Analog Output (AO) count: _____
  - [ ] Output type (0-10V / 4-20mA)
  - [ ] Resolution/accuracy

### Special I/O
- [ ] Pulse/counter inputs
- [ ] PWM outputs
- [ ] 1-Wire interfaces
- [ ] Other specialized I/O

---

## 📡 Protocols and Software

- [ ] Primary protocol defined
- [ ] Secondary protocols (if any)
- [ ] Firmware requirements identified
- [ ] Configuration method specified
- [ ] Integration with host controller (if applicable)

---

## 📝 Documentation

- [ ] Summary section written
- [ ] Use cases documented
- [ ] Hardware platform section complete
- [ ] Connectivity section complete
- [ ] Power section complete
- [ ] Field interfaces section complete
- [ ] I/O specifications detailed
- [ ] Protocols section complete
- [ ] Notes/special considerations added

---

## 🎨 Deliverables

- [ ] Markdown spec created: `dist/hardware/[product-name].md`
- [ ] PDF generated: `dist/hardware/[product-name].pdf`
- [ ] HTML preview generated (optional): `dist/hardware/[product-name].html`
- [ ] Spec reviewed for accuracy
- [ ] No AI bloat or unnecessary technical details
- [ ] All TBD items identified

---

## ✅ Validation

- [ ] I/O counts match requirements
- [ ] Deployment mode constraints satisfied
  - [ ] Side plugin: non-ISO RS485, BACnet only, no LoRa
  - [ ] Remote: ISO RS485, BACnet or Modbus, optional LoRa
- [ ] Processor capabilities align with requirements
  - [ ] STM32: no wireless
  - [ ] ESP32: wireless available (remote mode only)
- [ ] Power specifications match deployment mode
- [ ] Protocol support matches deployment mode
- [ ] All customer requirements addressed

---

## 🚀 Completion

- [ ] Spec saved to correct location (`dist/hardware/`)
- [ ] PDF generation successful
- [ ] File naming convention followed (kebab-case)
- [ ] Ready for review/approval

---

## Quick Reference: Constraints

| Deployment | RS485 | Protocols | LoRa | Power Source |
|------------|-------|-----------|------|--------------|
| Side plugin | Non-ISO | BACnet only | ❌ No | ACBM host |
| Remote I/O | ISO | BACnet or Modbus | ✅ Yes (ESP32) | External |

| Processor | Wireless | Best For |
|-----------|----------|----------|
| STM32 | ❌ No | RS485 only, cost-sensitive |
| ESP32 | ✅ Yes (remote) | Wireless, remote deployments |
