# 74LS 系列器件电气参数表（TTL）

本表是 `element/ttl/spec.go` 中 `SpecOf` 表的**同源人读版本**：每行一个型号，与 Go 侧 `deviceSpecs` 一一对应。
数值取自公开手册的**典型值**（typical，TA = 25 °C，VCC = 5 V）；只记数值与出处，不复制手册正文与表格图片。

## 列与单位

| 列 | 单位 | 说明 |
|---|---|---|
| TpdLH / TpdHL | ns（表内）；Go 侧存秒 | 手册 Switching Characteristics 表的 TYP 值 |
| IIH / IIL | 单元格内带 SI 前缀（`u` = 1e-6 A，`m` = 1e-3 A） | 手册 Electrical Characteristics 表的限值 |
| IOH / IOL | 同上 | 手册 Recommended Operating Conditions 的额定值 |
| VOH / VOL | V | 手册 Electrical Characteristics 表的 TYP 值 |
| Source | — | 手册名 + 文档号 + 表号/页码，或 URL 锚点；未取证写 `未取证` |

**取值口径**：缺值一律写 `（未取证）`，绝不填推测值；Go 侧对应字段为 `0`。
多档测试条件时取"与该器件 IOL 额定值同档"的那一行（例如 74LS00/08/32/74 取 IOL = 8 mA 档的 VOL TYP）。

| 型号 | TpdLH(ns) | TpdHL(ns) | IIH | IIL | IOH | IOL | VOH | VOL | Source |
|---|---|---|---|---|---|---|---|---|---|
| 74LS00 | 9 | 10 | 20u | -0.4m | -0.4m | 8m | 3.4 | 0.35 | TI SN74LS00, SDLS025D, §6.3/§6.6/§6.9 (RL=2k, CL=15pF) |
| 74LS04 | 9 | 10 | （未取证） | （未取证） | -0.4m | 8m | 3.4 | 0.25 | TI SN74LS04, SDLS029C, "54LS/74LS" 各表 (RL=2k, CL=15pF) |
| 74LS08 | 8 | 10 | 20u | -0.4m | -0.4m | 8m | 3.4 | 0.35 | TI SN74LS08, SDLS033, p.4 各表 (RL=2k, CL=15pF) |
| 74LS32 | 14 | （未取证） | 20u | -0.4m | -0.4m | 8m | 2.7 | 0.35 | TI SN54LS32/SN74LS32, p.4 各表 (RL=2k, CL=15pF) |
| 74LS74 | 13 | 25 | 20u | -0.4m | -0.4m | 8m | 3.4 | 0.35 | TI SN74LS74A, SDLS119, p.5 各表 (CLR/PRE/CLK→Q, RL=2k, CL=15pF) |
| 74LS86 | （未取证） | （未取证） | （未取证） | （未取证） | -0.4m | 8m | 3.4 | 0.25 | TI SN74LS86A, SDLS124, p.4 各表 |
| 74LS138 | （未取证） | （未取证） | （未取证） | （未取证） | -0.4m | 8m | 3.4 | （未取证） | TI SN54LS138/SN74LS138, p.4 各表 |
| 74LS164 | （未取证） | （未取证） | 20u | -0.4m | -0.4m | 8m | 3.5 | 0.35 | TI SN54LS164/SN74LS164, p.5 各表 |
| 74LS245 | 8 | 8 | 20u | -0.2m | -15m | 24m | 3.4 | （未取证） | TI SN74LS245, SDLS146B, §7.3/§7.5/§7.7 (CL=45pF, RL=667Ω) |
| 74LS373 | 12 | 12 | （未取证） | （未取证） | -2.6m | 24m | （未取证） | （未取证） | TI SN74LS373, SDLS165B, "54LS/74LS" 表 (Data→Q, RL=667Ω, CL=45pF) |
| 74LS574 | （未取证） | （未取证） | 20u | -0.4m | -2.6m | 24m | 3.3 | 0.35 | Fairchild DM74LS574, Farnell 18717.pdf, Recommended/DC/Switching 表 |

## 取值备注（逐型号）

- 74LS00：VOH 3.4 V 为 VCC=MIN、IOH=-0.4 mA 时的 TYP；VOL 0.35 V 为 IOL=8 mA 档 TYP。
- 74LS04：VOL 取手册 IOL=8 mA 档 TYP 0.25 V（该手册 4 mA 档 TYP 亦为 0.25 V）；IIH/IIL 在 400 dpi OCR 中未读出，标未取证。
- 74LS08 / 74LS32 / 74LS74：VOL 取 IOL=8 mA 档 TYP；74LS32 的 tPHL 行 OCR 未读出。
- 74LS74：IIL 取 D/CLK 输入档 -0.4 mA（该手册 CLR/PRE 输入档为 -0.8 mA）；tPHL TYP 25 ns 为 CLR/PRE/CLK→Q 行。
- 74LS86：tpd 与 IIH/IIL 行 OCR 未读出；VOH 3.4 V 取 SN74LS86A 列 TYP，VOL 0.25 V 取 IOL=4 mA 档 TYP。
- 74LS138 / 74LS164：tpd 行 OCR 读出多组数值但列归属无法判定，按要求标未取证；IOL 8 mA、IOH -0.4 mA 取 SN74LS 列；74LS164 的 VOL 0.35 V 为 IOL=8 mA 档 TYP。
- 74LS245：VOH 3.4 V 为 VCC=MIN、IOH=-3 mA 时的 TYP（该手册另给 IOH=MAX 档 VOH(MIN)=2.0 V）；VOL 手册只给 MAX（12 mA 档 0.4 V、24 mA 档 0.5 V），无 TYP，故标未取证。
- 74LS373：tpd 取 "Data→Any Q" 行 12/12 ns（RL=667Ω, CL=45pF）；该器件另有 C/CLK→Q 行（20/30 ns 一组），列归属存疑故未采用；VOH/VOL/IIH/IIL 未取证。
- 74LS574：VOH 3.3 V / VOL 0.35 V（IOL=Max 档）/ IIH 20 µA / IIL -400 µA 均取该 Fairchild 手册 DC 表 TYP 与限值；
  tpd 该手册 Switching Characteristics 表只有 MAX 28 ns（无 TYP 列），为保持"典型值"口径一致，Tpd 标未取证。

## 未取证清单

74LS04 的 IIH/IIL；74LS32 的 TpdHL；74LS86 的 TpdLH/TpdHL/IIH/IIL；74LS138 的 TpdLH/TpdHL/IIH/IIL/VOL；
74LS245 的 VOL；74LS373 的 IIH/IIL/VOH/VOL；74LS574 的 TpdLH/TpdHL（手册只给 MAX 28 ns）。

原因：TI 对多数老 LS 器件只发布**无文本层**的扫描件（`pdftotext` 读不出表格），本任务用 `pdftoppm + tesseract` OCR 取证；
OCR 读不出的行、以及只有 MAX 而无 TYP 的行，按规则标未取证，不猜测、不回退默认值。
