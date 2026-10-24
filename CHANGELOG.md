# CHANGELOG

## v0.1.0（生产级规划 · 批1/批2 最小闭环）

- m1：外部精度基准 harness（vendored ngspice-47 + sha256 + 语料清单 + `--selfcheck`）。
- m2：`.model` 卡解析（NMOS/PMOS/DIODE，未知参数报错）+ 公共 `element.ModelCard`。
- m3：DC 扫描（`RunDCSweep`）与 `.meas`（AVG/MAX/MIN/PP/FIND）。
- m4：1e5 规模基线协议；**1e5 OOM**（未满足 C5，待内存/分块优化）。
- m6：SPICE 子集前端（`ParseSpiceNetlist`）与收敛矩阵（3 例失败 0）。
- m5：CI 脚本 / Makefile / 门禁清单 / 版本记录（本文件）。
