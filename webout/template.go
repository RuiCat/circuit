package webout

// htmlPageTemplate 是自包含单网页模板，__DATA__ 会被替换为 JSON 数据。
// 注意：模板内 JavaScript 一律避免使用反引号模板字符串。
const htmlPageTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>circuit 仿真结果</title>
<style>
:root{
  --bg:#f6f8fa; --panel:#ffffff; --border:#e2e6ea; --text:#1f2937; --muted:#6b7280; --accent:#2563eb;
}
*{box-sizing:border-box; margin:0; padding:0;}
body{background:var(--bg); color:var(--text); font:14px/1.5 -apple-system,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;}
.wrap{max-width:1200px; margin:0 auto; padding:20px 16px 48px;}
header{display:flex; flex-wrap:wrap; align-items:baseline; gap:12px; margin-bottom:14px;}
h1{font-size:20px; font-weight:700;}
header .sub{color:var(--muted); font-size:13px;}
.meta{display:flex; flex-wrap:wrap; gap:8px; margin-bottom:16px;}
.chip{background:var(--panel); border:1px solid var(--border); border-radius:999px; padding:4px 12px; font-size:12px; color:var(--muted);}
.chip b{color:var(--text); font-weight:600; margin-left:4px;}
.controls{display:flex; flex-wrap:wrap; gap:8px; margin-bottom:10px; align-items:center;}
button{background:var(--panel); border:1px solid var(--border); border-radius:8px; padding:6px 14px; font-size:13px; cursor:pointer; color:var(--text);}
button:hover{border-color:var(--accent); color:var(--accent);}
.hint{color:var(--muted); font-size:12px; margin-left:auto;}
.legend{display:flex; flex-direction:column; gap:8px; margin:12px 0 14px; padding:12px 14px; background:var(--panel); border:1px solid var(--border); border-radius:12px;}
.lg-group{display:flex; flex-wrap:wrap; align-items:center; gap:8px;}
.lg-group-label{font-size:12px; font-weight:600; color:var(--muted); margin-right:4px; min-width:72px;}
.lg-item{display:flex; align-items:center; gap:6px; border:1px solid var(--border); border-radius:999px; padding:3px 12px; font-size:12px; cursor:pointer; user-select:none; color:var(--muted); background:#fafbfc;}
.lg-item.on{color:var(--text);}
.lg-item.off{opacity:.4; text-decoration:line-through;}
.lg-item .lg-dot{width:10px; height:10px; border-radius:50%; flex:none;}
.charts{display:flex; flex-direction:column; gap:16px;}
.chart-wrap{position:relative; background:var(--panel); border:1px solid var(--border); border-radius:12px; padding:12px; box-shadow:0 1px 2px rgba(16,24,40,.06);}
.chart-title{font-size:13px; font-weight:600; color:var(--muted); margin-bottom:8px;}
canvas.chart{display:block; width:100%; height:420px; cursor:crosshair; touch-action:none;}
.tt{position:absolute; pointer-events:none; background:rgba(15,23,42,.92); color:#f1f5f9; font-size:12px; border-radius:8px; padding:8px 10px; white-space:nowrap; display:none; z-index:10; line-height:1.6;}
.tt .tt-time{color:#93c5fd; font-weight:600;}
.tt .tt-row{display:flex; gap:8px; align-items:center;}
.tt .tt-dot{width:8px; height:8px; border-radius:50%; display:inline-block; flex:none;}
.tableNote{color:var(--muted); font-size:12px; margin-bottom:6px;}
.tablebox{overflow:auto; max-height:480px; border:1px solid var(--border); border-radius:10px; background:var(--panel);}
#datatable{border-collapse:collapse; width:100%; font-size:12px;}
#datatable td,#datatable th{padding:5px 10px; text-align:right; white-space:nowrap; font-variant-numeric:tabular-nums; border-bottom:1px solid var(--border);}
#datatable th{position:sticky; top:0; background:#f1f5f9; z-index:1;}
#datatable td:first-child,#datatable th:first-child{color:var(--muted);}
</style>
</head>
<body>
<div class="wrap">
  <header><h1>电路仿真结果</h1><span class="sub" id="subtitle"></span></header>
  <div class="meta" id="meta"></div>
  <div class="controls">
    <button id="btnReset">复位视图</button>
    <button id="btnAutoY">自动适配 Y</button>
    <button id="btnTable">显示/隐藏数据表</button>
    <button id="btnCsv">下载 CSV</button>
    <span class="hint">滚轮缩放 · 拖拽平移 · 悬停查看数值 · 点击图例开关曲线</span>
  </div>
  <div class="legend" id="legend"></div>
  <div class="charts" id="charts"></div>
  <div id="tableWrap" style="display:none; margin-top:14px;">
    <div class="tableNote" id="tableNote"></div>
    <div class="tablebox"><table id="datatable"></table></div>
  </div>
</div>
<script>
"use strict";
var DATA = __DATA__;
var headers = DATA.headers, time = DATA.time, series = DATA.series, meta = DATA.meta;
var units = DATA.units || [], kinds = DATA.kinds || [];
var nSer = series.length;
var COLORS = ['#e63946','#457b9d','#2a9d8f','#e76f51','#8338ec','#f4a261','#264653','#c1121f','#6a994e','#bc6c25','#5e548e','#023047','#ef8354','#3a86ff'];

document.title = meta.title + ' · 电路仿真结果';
document.getElementById('subtitle').textContent = meta.title;

var metaBox = document.getElementById('meta');
function chip(label, val){
  var d = document.createElement('span');
  d.className = 'chip';
  d.textContent = label;
  var b = document.createElement('b');
  b.textContent = val;
  d.appendChild(b);
  return d;
}
metaBox.appendChild(chip('元件数', meta.elements));
metaBox.appendChild(chip('节点数', meta.nodes));
metaBox.appendChild(chip('仿真步数', meta.steps));
metaBox.appendChild(chip('目标时间', fmtT(meta.targetTime) + ' s'));
metaBox.appendChild(chip('最终时间', fmtT(meta.finalTime) + ' s'));

function fmtT(v){ return fmtNum(v, 6); }
function fmtNum(v, d){
  if (v === null || v === undefined || isNaN(v)) return '-';
  if (!isFinite(v)) return String(v);
  var a = Math.abs(v);
  if (a !== 0 && (a >= 1e5 || a < 1e-4)) return v.toExponential(4);
  return v.toPrecision(d === undefined ? 6 : d);
}

// ---- 序列分组：按 量纲(kind) × 单位(unit) 分组。
// 电气/气动/液压是三个独立系统：电压(V)、压力(Pa)、电流(A)、质量流量(kg/s)、体积流量(m³/s)。
var groups = {}; // key: kind|unit -> {kind, unit, idxs: []}
for (var si = 0; si < nSer; si++){
  var k = kinds[si] || 'voltage';
  var u = units[si] || 'V';
  var key = k + '|' + u;
  if (!groups[key]) groups[key] = {kind: k, unit: u, idxs: []};
  groups[key].idxs.push(si);
}
var groupKeys = Object.keys(groups);
// 组排序：voltage 组在前，current 组在后；同 kind 按单位稳定排序
groupKeys.sort(function(a, b){
  var ka = groups[a].kind, kb = groups[b].kind;
  if (ka !== kb) return ka === 'voltage' ? -1 : 1;
  return a < b ? -1 : a > b ? 1 : 0;
});

// 组标签与 Y 轴标题（区分 电压/压力/电流/流量）
function groupLabel(kind, unit){
  if (kind === 'voltage'){
    if (unit === 'V') return '电压 (V)';
    if (unit === 'Pa') return '压力 (Pa)';
    return '电压/压力 (' + unit + ')';
  }
  if (kind === 'current'){
    if (unit === 'A') return '电流 (A)';
    if (unit === 'kg/s') return '质量流量 (kg/s)';
    if (unit === 'm³/s') return '体积流量 (m³/s)';
    return '流量 (' + unit + ')';
  }
  return kind + ' (' + unit + ')';
}
function yLabelOf(kind, unit){
  if (kind === 'voltage'){
    if (unit === 'V') return '电压 V (V)';
    if (unit === 'Pa') return '压力 p (Pa)';
    return '量 (' + unit + ')';
  }
  if (kind === 'current'){
    if (unit === 'A') return '电流 I (A)';
    if (unit === 'kg/s') return '质量流量 qm (kg/s)';
    if (unit === 'm³/s') return '体积流量 Q (m³/s)';
    return '流量 (' + unit + ')';
  }
  return kind + ' (' + unit + ')';
}

// 颜色按全局序列顺序分配
var colorOf = [];
for (var s = 0; s < nSer; s++) colorOf[s] = COLORS[s % COLORS.length];

// ---- 图例（按组展示，点击切换曲线显隐） ----
var visible = [];
var legendEl = document.getElementById('legend');
groupKeys.forEach(function(key){
  var g = groups[key];
  var groupEl = document.createElement('div');
  groupEl.className = 'lg-group';
  var lab = document.createElement('span');
  lab.className = 'lg-group-label';
  lab.textContent = groupLabel(g.kind, g.unit);
  groupEl.appendChild(lab);
  g.idxs.forEach(function(i){
    visible.push(true);
    var el = document.createElement('span');
    el.className = 'lg-item on';
    var dot = document.createElement('span');
    dot.className = 'lg-dot';
    dot.style.background = colorOf[i];
    var name = document.createElement('span');
    name.textContent = headers[i + 1];
    el.appendChild(dot);
    el.appendChild(name);
    el.onclick = function(){
      visible[i] = !visible[i];
      el.classList.toggle('on', visible[i]);
      el.classList.toggle('off', !visible[i]);
      if (autoY) charts.forEach(function(c){ c.fitY(); });
      charts.forEach(function(c){ c.draw(); });
    };
    groupEl.appendChild(el);
  });
  legendEl.appendChild(groupEl);
});

// ---- 1/2/5 步进的"好看"刻度 ----
function niceTicks(min, max, count){
  if (min === max){ min -= 1; max += 1; }
  var span = max - min;
  var step0 = span / count;
  var mag = Math.pow(10, Math.floor(Math.log(step0) / Math.LN10));
  var norm = step0 / mag;
  var step;
  if (norm < 1.5) step = 1; else if (norm < 3) step = 2; else if (norm < 7) step = 5; else step = 10;
  step *= mag;
  var out = [];
  var start = Math.ceil(min / step) * step;
  for (var v = start; v <= max + step * 1e-9; v += step) out.push(v);
  return out;
}

function idxAt(t){
  if (t <= time[0]) return 0;
  if (t >= time[time.length - 1]) return time.length - 1;
  var lo = 0, hi = time.length - 1;
  while (lo < hi){
    var mid = (lo + hi + 1) >> 1;
    if (time[mid] <= t) lo = mid; else hi = mid - 1;
  }
  return lo;
}

var t0 = time.length ? time[0] : 0;
var t1 = time.length ? time[time.length - 1] : 0;
var margins = {l:76, r:14, t:14, b:40};

// ---- 图表工厂：每个图表独立 Y 轴与交互状态 ----
function makeChart(canvas, tt, idxs, yLabel){
  var ctx = canvas.getContext('2d');
  var view = {x0: t0, x1: t1, y0: 0, y1: 0};
  var hoverIdx = -1, dragging = false, lastPX = 0, lastTX = 0;

  var loAll = Infinity, hiAll = -Infinity;
  for (var i = 0; i < idxs.length; i++){
    var s0 = idxs[i];
    for (var j = 0; j < series[s0].length; j++){
      var v0 = series[s0][j];
      if (v0 < loAll) loAll = v0;
      if (v0 > hiAll) hiAll = v0;
    }
  }
  if (!isFinite(loAll)){ loAll = 0; hiAll = 1; }
  var pad = (hiAll - loAll) * 0.06 || 1;

  function fitY(){
    var lo = Infinity, hi = -Infinity;
    for (var i = 0; i < idxs.length; i++){
      var s = idxs[i];
      if (!visible[s]) continue;
      for (var j = 0; j < series[s].length; j++){
        var v = series[s][j];
        if (v < lo) lo = v;
        if (v > hi) hi = v;
      }
    }
    if (!isFinite(lo)){ lo = loAll; hi = hiAll; }
    view.y0 = lo - pad;
    view.y1 = hi + pad;
    if (view.y0 === view.y1){ view.y0 -= 1; view.y1 += 1; }
  }

  function resetView(){ view.x0 = t0; view.x1 = t1; fitY(); draw(); }

  function draw(){
    var dpr = window.devicePixelRatio || 1;
    var rect = canvas.getBoundingClientRect();
    var W = rect.width, H = rect.height;
    if (canvas.width !== Math.round(W * dpr) || canvas.height !== Math.round(H * dpr)){
      canvas.width = Math.round(W * dpr);
      canvas.height = Math.round(H * dpr);
    }
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, W, H);
    if (time.length < 2 || idxs.length === 0) return;

    var pw = W - margins.l - margins.r;
    var ph = H - margins.t - margins.b;
    var X = function(t){ return margins.l + (t - view.x0) / (view.x1 - view.x0) * pw; };
    var Y = function(v){ return margins.t + (view.y1 - v) / (view.y1 - view.y0) * ph; };

    ctx.font = '11px -apple-system, "Segoe UI", "PingFang SC", sans-serif';
    ctx.lineWidth = 1;

    var yt = niceTicks(view.y0, view.y1, 6);
    ctx.strokeStyle = '#eef1f4';
    ctx.fillStyle = '#6b7280';
    ctx.textAlign = 'right';
    ctx.textBaseline = 'middle';
    for (var i = 0; i < yt.length; i++){
      var gy = Math.round(Y(yt[i])) + 0.5;
      ctx.beginPath();
      ctx.moveTo(margins.l, gy);
      ctx.lineTo(W - margins.r, gy);
      ctx.stroke();
      ctx.fillText(fmtNum(yt[i], 5), margins.l - 8, gy);
    }

    var xt = niceTicks(view.x0, view.x1, 8);
    ctx.textAlign = 'center';
    ctx.textBaseline = 'top';
    for (var j = 0; j < xt.length; j++){
      var gx = Math.round(X(xt[j])) + 0.5;
      ctx.beginPath();
      ctx.moveTo(gx, margins.t);
      ctx.lineTo(gx, H - margins.b);
      ctx.stroke();
      ctx.fillText(fmtT(xt[j]), gx, H - margins.b + 7);
    }

    ctx.fillStyle = '#374151';
    ctx.font = '12px -apple-system, "Segoe UI", "PingFang SC", sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText('时间 t (s)', margins.l + pw / 2, H - 12);
    ctx.save();
    ctx.translate(13, margins.t + ph / 2);
    ctx.rotate(-Math.PI / 2);
    ctx.fillText(yLabel, 0, 0);
    ctx.restore();

    var stride = 1, maxPts = 4000;
    if (time.length > maxPts) stride = Math.ceil(time.length / maxPts);
    for (var i2 = 0; i2 < idxs.length; i2++){
      var s = idxs[i2];
      if (!visible[s]) continue;
      var pts = series[s];
      ctx.strokeStyle = colorOf[s];
      ctx.lineWidth = 1.6;
      ctx.beginPath();
      var started = false;
      for (var k = 0; k < pts.length; k += stride){
        var px = X(time[k]), py = Y(pts[k]);
        if (!started){ ctx.moveTo(px, py); started = true; } else ctx.lineTo(px, py);
      }
      ctx.stroke();
    }

    if (hoverIdx >= 0){
      var hx = Math.round(X(time[hoverIdx])) + 0.5;
      ctx.save();
      ctx.strokeStyle = 'rgba(100,116,139,.55)';
      ctx.setLineDash([4, 4]);
      ctx.beginPath();
      ctx.moveTo(hx, margins.t);
      ctx.lineTo(hx, H - margins.b);
      ctx.stroke();
      ctx.restore();
      for (var i3 = 0; i3 < idxs.length; i3++){
        var s2 = idxs[i3];
        if (!visible[s2]) continue;
        var hv = series[s2][hoverIdx];
        if (hv < view.y0 || hv > view.y1) continue;
        ctx.fillStyle = colorOf[s2];
        ctx.beginPath();
        ctx.arc(hx, Y(hv), 3.5, 0, Math.PI * 2);
        ctx.fill();
      }
    }
  }

  function showTooltip(px, py){
    if (hoverIdx < 0){ tt.style.display = 'none'; return; }
    var html = '<div class="tt-time">t = ' + fmtT(time[hoverIdx]) + ' s</div>';
    for (var i = 0; i < idxs.length; i++){
      var s = idxs[i];
      if (!visible[s]) continue;
      html += '<div class="tt-row"><span class="tt-dot" style="background:' + colorOf[s] + '"></span>'
        + headers[s + 1] + ' = ' + fmtNum(series[s][hoverIdx], 8) + ' ' + (units[s] || '') + '</div>';
    }
    tt.innerHTML = html;
    tt.style.display = 'block';
    var tw = tt.offsetWidth, th = tt.offsetHeight;
    var cw = canvas.getBoundingClientRect().width;
    var ch = canvas.getBoundingClientRect().height;
    var x = px + 14, y = py - 10;
    if (x + tw > cw - 6) x = px - tw - 14;
    if (y + th > ch - 6) y = py - th + 10;
    tt.style.left = x + 'px';
    tt.style.top = y + 'px';
  }

  function hideTooltip(){ tt.style.display = 'none'; }

  canvas.addEventListener('pointerdown', function(e){
    dragging = true;
    lastPX = e.clientX;
    lastTX = view.x0;
    try { canvas.setPointerCapture(e.pointerId); } catch (err) {}
  });

  canvas.addEventListener('pointermove', function(e){
    var rect = canvas.getBoundingClientRect();
    var mx = e.clientX - rect.left;
    var pw = rect.width - margins.l - margins.r;
    if (dragging){
      var dt = (lastPX - e.clientX) / pw * (view.x1 - view.x0);
      var nx0 = lastTX + dt;
      var nx1 = nx0 + (view.x1 - view.x0);
      var range = t1 - t0;
      var lo = t0 - range * 0.05, hi = t1 + range * 0.05;
      if (nx0 < lo){ nx0 = lo; nx1 = nx0 + (view.x1 - view.x0); }
      if (nx1 > hi){ nx1 = hi; nx0 = nx1 - (view.x1 - view.x0); }
      view.x0 = nx0;
      view.x1 = nx1;
      draw();
      return;
    }
    if (time.length < 2){ hideTooltip(); return; }
    var tAt = mx >= margins.l
      ? view.x0 + (mx - margins.l) / pw * (view.x1 - view.x0)
      : view.x0;
    hoverIdx = idxAt(tAt);
    draw();
    showTooltip(mx, e.clientY - rect.top);
  });

  canvas.addEventListener('pointerup', function(){ dragging = false; });
  canvas.addEventListener('pointerleave', function(){
    hoverIdx = -1;
    dragging = false;
    draw();
    hideTooltip();
  });

  canvas.addEventListener('wheel', function(e){
    e.preventDefault();
    var rect = canvas.getBoundingClientRect();
    var mx = e.clientX - rect.left;
    var pw = rect.width - margins.l - margins.r;
    var tAt = view.x0 + (mx - margins.l) / pw * (view.x1 - view.x0);
    var f = e.deltaY > 0 ? 1.2 : 1 / 1.2;
    view.x0 = tAt - (tAt - view.x0) * f;
    view.x1 = view.x0 + (view.x1 - view.x0) * f;
    draw();
  }, {passive: false});

  return {resetView: resetView, draw: draw, fitY: fitY, canvas: canvas};
}

// ---- 按 (量纲, 单位) 分组动态创建图表 ----
var chartsEl = document.getElementById('charts');
var charts = [];
groupKeys.forEach(function(key){
  var g = groups[key];
  var wrap = document.createElement('div');
  wrap.className = 'chart-wrap';
  var title = document.createElement('div');
  title.className = 'chart-title';
  title.textContent = groupLabel(g.kind, g.unit) + '曲线';
  var canvas = document.createElement('canvas');
  canvas.className = 'chart';
  var tt = document.createElement('div');
  tt.className = 'tt';
  wrap.appendChild(title);
  wrap.appendChild(canvas);
  wrap.appendChild(tt);
  chartsEl.appendChild(wrap);
  charts.push(makeChart(canvas, tt, g.idxs, yLabelOf(g.kind, g.unit)));
});

// ---- 工具栏按钮 ----
var autoY = true;

document.getElementById('btnReset').onclick = function(){
  charts.forEach(function(c){ c.resetView(); });
};

document.getElementById('btnAutoY').onclick = function(){
  autoY = !autoY;
  this.textContent = autoY ? '自动适配 Y' : '自动适配 Y: 关';
  if (autoY) charts.forEach(function(c){ c.fitY(); });
  charts.forEach(function(c){ c.draw(); });
};

document.getElementById('btnTable').onclick = function(){
  var tw = document.getElementById('tableWrap');
  var show = tw.style.display === 'none';
  tw.style.display = show ? 'block' : 'none';
  if (show && !document.getElementById('datatable').rows.length) buildTable();
};

document.getElementById('btnCsv').onclick = function(){
  var lines = [headers.join(',')];
  for (var r = 0; r < time.length; r++){
    var cells = [String(time[r])];
    for (var c = 0; c < nSer; c++) cells.push(String(series[c][r]));
    lines.push(cells.join(','));
  }
  var blob = new Blob([lines.join('\r\n')], {type: 'text/csv;charset=utf-8'});
  var a = document.createElement('a');
  a.href = URL.createObjectURL(blob);
  a.download = (meta.title || 'circuit') + '.csv';
  a.click();
  setTimeout(function(){ URL.revokeObjectURL(a.href); }, 1000);
};

// ---- 数据表（最多渲染 5000 行，完整数据用 CSV 导出） ----
function buildTable(){
  var table = document.getElementById('datatable');
  var thead = document.createElement('thead');
  var hr = document.createElement('tr');
  for (var i = 0; i < headers.length; i++){
    var th = document.createElement('th');
    th.textContent = headers[i];
    hr.appendChild(th);
  }
  thead.appendChild(hr);
  table.appendChild(thead);

  var MAXROWS = 5000;
  var total = time.length;
  var note = document.getElementById('tableNote');
  note.textContent = total > MAXROWS
    ? '数据共 ' + total + ' 行，表格仅显示前 ' + MAXROWS + ' 行；完整数据请下载 CSV。'
    : '共 ' + total + ' 行数据。';

  var n = Math.min(total, MAXROWS);
  var tbody = document.createElement('tbody');
  for (var r = 0; r < n; r++){
    var tr = document.createElement('tr');
    if (r % 2 === 1) tr.style.background = '#fafbfc';
    var td = document.createElement('td');
    td.textContent = fmtT(time[r]);
    tr.appendChild(td);
    for (var c = 0; c < nSer; c++){
      td = document.createElement('td');
      td.textContent = fmtNum(series[c][r], 8);
      tr.appendChild(td);
    }
    tbody.appendChild(tr);
  }
  table.appendChild(tbody);
}

// ---- 启动 ----
charts.forEach(function(c){ c.resetView(); });
window.addEventListener('resize', function(){ charts.forEach(function(c){ c.draw(); }); });
</script>
</body>
</html>
`
