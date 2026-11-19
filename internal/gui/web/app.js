// htt gui: draws the time log from htt's JSON API (api/week, api/month).
// htt computes every figure; this only lays them out. Routes are hashes:
// #week, #week/2026-11-16 (any day of the week), #month, #month/2026-10.
(function () {
  "use strict";

  // ---- Formatting ----
  const pad = (n) => String(n).padStart(2, "0");
  const clock = (minute) => {
    const m = Math.round(minute);
    return pad(Math.floor(m / 60)) + ":" + pad(m % 60);
  };
  const dur = (seconds) => {
    const m = Math.round(seconds / 60);
    const h = Math.floor(m / 60);
    return h > 0 ? h + "h " + (m % 60) + "m" : m + "m";
  };
  const hours = (seconds) => Math.round(seconds / 36) / 100;
  const shortHours = (seconds) => Math.round(seconds / 360) / 10 + "h";
  const parseDate = (s) => {
    const [y, m, d] = s.split("-").map(Number);
    return new Date(y, m - 1, d || 1);
  };
  const fmt = (s, opts) => parseDate(s).toLocaleDateString("en-GB", opts);
  const dayName = (s) => fmt(s, { weekday: "short" }) + " " + parseDate(s).getDate();
  const longDay = (s) => fmt(s, { weekday: "long", day: "numeric", month: "long" });
  const monthName = (s) => fmt(s, { month: "long" });
  const shortMonth = (s) => fmt(s, { month: "short" });
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const percent = (x) => Math.round(x * 100) + "%";

  function weekLabel(start, end) {
    const a = parseDate(start), b = parseDate(end);
    if (a.getFullYear() !== b.getFullYear()) {
      return `${a.getDate()} ${monthName(start)} ${a.getFullYear()} – ${b.getDate()} ${monthName(end)} ${b.getFullYear()}`;
    }
    if (a.getMonth() !== b.getMonth()) {
      return `${a.getDate()} ${monthName(start)} – ${b.getDate()} ${monthName(end)} ${b.getFullYear()}`;
    }
    return `${a.getDate()}–${b.getDate()} ${monthName(end)} ${b.getFullYear()}`;
  }
  const monthLabel = (start) => monthName(start) + " " + parseDate(start).getFullYear();
  // "1–4 Oct", "31 Oct": a run of days within a month.
  const spanLabel = (start, end) => (start === end ? "" : parseDate(start).getDate() + "–") + parseDate(end).getDate() + " " + shortMonth(end);

  // ---- Categories ----
  // Stacks and legends: the categories that count as work, in the
  // config's order, uncategorised among them when it counts as work,
  // then the rest; so a bar's worked time is one run from its base.
  function seriesOrder(categories) {
    const uncat = categories.find((c) => c.uncategorised);
    const named = categories.filter((c) => !c.uncategorised);
    const work = named.filter((c) => c.work), rest = named.filter((c) => !c.work);
    return uncat && uncat.work ? [...work, uncat, ...rest] : [...work, ...rest, ...(uncat ? [uncat] : [])];
  }
  const colourToken = (c) => c.uncategorised ? "--uncategorised" : c.work ? "--work-" + (c.colour % 8) : "--rest-" + (c.colour % 4);
  const seriesOf = (iv) => iv.uncategorised ? "uncategorised" : iv.category;
  // Anything keyed by a category's name is looked up as its own: a
  // category may be called constructor, toString or __proto__.
  const timeIn = (totals, name) => (totals && Object.hasOwn(totals.categories, name) && totals.categories[name]) || 0;
  const workText = (c) => c.uncategorised
    ? (c.work ? "yes" : "no") + ", as " + c.countedAs
    : c.work ? "yes" : "no";

  // ---- Theme ----
  const css = (n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim();
  let P; // the palette in effect
  function palette(categories) {
    const cat = Object.create(null); // by name, so no name is Object.prototype's
    categories.forEach((c) => { cat[c.name] = css(colourToken(c)); });
    return {
      fg: css("--fg"), muted: css("--muted"), line: css("--line"), surface: css("--surface"), bg: css("--bg"),
      accent: css("--accent"), onAccent: css("--on-accent"), warn: css("--warn"), heatLo: css("--heat-lo"), heatHi: css("--heat-hi"),
      cat,
    };
  }
  const FONT = () => css("--font-body"), MONO = () => css("--font-mono");
  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const tip = (extra) => Object.assign({
    backgroundColor: P.surface, borderColor: P.line, borderWidth: 1, textStyle: { color: P.fg },
    extraCssText: "box-shadow:none;border-radius:6px;",
  }, extra);
  const axisText = () => ({ color: P.muted, fontFamily: MONO(), fontSize: 11 });
  const tipRow = (label, value) => `<div style="display:flex;justify-content:space-between;gap:16px"><span>${label}</span><span style="font-family:${MONO()}">${value}</span></div>`;
  const tipWrap = (body) => `<div style="font-family:${FONT()};font-size:13px;line-height:1.5;min-width:170px">${body}</div>`;
  const base = () => ({ backgroundColor: "transparent", animation: !reduced, textStyle: { fontFamily: FONT(), color: P.fg } });
  const legend = (keys) => ({
    top: 0, left: 0, icon: "roundRect", itemWidth: 12, itemHeight: 12, itemGap: 14,
    textStyle: { color: P.fg, fontFamily: FONT(), fontSize: 13 }, data: keys.map((c) => c.name),
  });
  const swatch = (c) => `<span class="swatch"><i style="background: ${P.cat[c.name]}"></i>${esc(c.name)}</span>`;
  const shadowPointer = { type: "shadow", shadowStyle: { color: "rgba(127,140,150,0.12)" } };

  // A tooltip's breakdown: the time per category, then worked and breaks.
  const breakdown = (title, totals, keys, extra) => tipWrap(`<div style="font-weight:600">${title}</div>`
    + (keys.filter((c) => timeIn(totals, c.name) > 0).map((c) => tipRow(esc(c.name), dur(timeIn(totals, c.name)))).join("")
      || `<div style="color:${P.muted}">nothing logged</div>`)
    + `<div style="border-top:1px solid ${P.line};margin-top:4px;padding-top:4px">${tipRow("worked", dur(totals.worked))}${tipRow("breaks", dur(totals.breaks))}</div>`
    + (extra || ""));

  // Stacked bars, one per row or column, with a label at each stack's end.
  function stackedSeries(rows, keys, totalsOf, opts) {
    const series = keys.map((c) => ({
      type: "bar", name: c.name, stack: "all", barWidth: opts.barWidth || "54%",
      itemStyle: { color: P.cat[c.name] }, emphasis: { disabled: true },
      data: rows.map((r) => hours(timeIn(totalsOf(r), c.name))),
    }));
    series.push({
      type: "bar", name: "label", stack: "all", silent: true, barWidth: opts.barWidth || "54%",
      itemStyle: { color: "transparent" }, tooltip: { show: false },
      data: rows.map(() => 0),
      label: {
        show: true, position: opts.position, formatter: (p) => opts.label(rows[p.dataIndex]),
        rich: {
          v: { color: P.fg, fontFamily: MONO(), fontSize: 12 },
          b: { color: P.muted, fontFamily: MONO(), fontSize: 11 },
          w: { color: P.warn, fontFamily: FONT(), fontSize: 11 },
          p: { color: P.accent, fontFamily: FONT(), fontSize: 11 },
          m: { color: P.muted, fontFamily: FONT(), fontSize: 12 },
        },
      },
    });
    return series;
  }

  // ---- Week: the day-by-day timeline (a custom series, after ECharts'
  // "Profile" example) ----
  function renderItem(params, api) {
    const row = api.value(0), s = api.value(1), e = api.value(2), flag = api.value(3);
    const a = api.coord([s, row]), b = api.coord([e, row]);
    const h = api.size([0, 1])[1] * 0.56;
    const cs = params.coordSys;
    const rect = echarts.graphic.clipRectByRect(
      { x: a[0], y: a[1] - h / 2, width: Math.max(b[0] - a[0], 1), height: h },
      { x: cs.x, y: cs.y, width: cs.width, height: cs.height });
    if (!rect) return;
    const children = [{
      type: "rect",
      shape: Object.assign({ r: 2 }, rect),
      style: flag === 1
        ? { fill: api.visual("color"), opacity: 0.45, stroke: P.warn, lineWidth: 1.5, lineDash: [4, 3] }
        : { fill: api.visual("color") },
    }];
    if (flag === 2 && b[0] >= cs.x && b[0] <= cs.x + cs.width) {
      children.push({ type: "line", shape: { x1: b[0], y1: a[1] - h / 2 - 5, x2: b[0], y2: a[1] + h / 2 + 5 }, style: { stroke: P.accent, lineWidth: 2 } });
    }
    return { type: "group", children };
  }

  const statusNote = (iv, data) => iv.open === "midnight" ? "No @end that day: counted to 24:00"
    : iv.open === "now" ? "In progress (now " + clock(data.nowMinute) + ")" : "";
  const statusTag = {
    unended: "{w|no @end}", inProgress: "{p|in progress}", empty: "{e|nothing logged}", future: "{e|not yet}",
  };

  function timelineOption(data, size, zoom) {
    const narrow = size !== "wide";
    const keys = seriesOrder(data.categories);
    const items = (name) => data.days.flatMap((day, row) => day.intervals.filter((iv) => seriesOf(iv) === name).map((iv) => ({
      // Drawn to drawnEndMinute: where the clocks go back, an entry's
      // wall clock can run backwards. The tooltip keeps its clock times.
      value: [row, iv.startMinute, iv.drawnEndMinute, iv.open === "midnight" ? 1 : iv.open === "now" ? 2 : 0],
      iv, date: day.date,
    })));
    const byDate = Object.fromEntries(data.days.map((d) => [d.date, d]));
    const span = zoom || [data.hours.from, data.hours.to];
    return Object.assign(base(), {
      legend: legend(keys),
      tooltip: tip({
        trigger: "item",
        formatter: (p) => {
          const iv = p.data.iv, note = statusNote(iv, data);
          const cat = iv.uncategorised ? "uncategorised (counted as " + esc(iv.category) + ")" : esc(iv.category);
          return tipWrap(`<div style="font-family:${MONO()};font-weight:500">${esc(iv.entry)}</div>
            <div style="color:${P.muted}">${dayName(p.data.date)} · ${clock(iv.startMinute)}–${clock(iv.endMinute)} · ${dur(iv.seconds)}</div>
            <div>${cat}${iv.work ? "" : " · not work"}</div>
            ${note ? `<div style="color:${P.warn}">${note}</div>` : ""}`);
        },
      }),
      grid: { left: narrow ? 78 : 96, right: 14, top: narrow ? 64 : 40, bottom: 58 },
      xAxis: {
        type: "value", min: 0, max: 1440, interval: narrow ? 180 : 60,
        axisLabel: Object.assign(axisText(), { formatter: (v) => clock(v) }),
        splitLine: { lineStyle: { color: P.line } }, axisLine: { show: false }, axisTick: { show: false },
      },
      yAxis: {
        type: "category", inverse: true, data: data.days.map((d) => d.date),
        axisLine: { lineStyle: { color: P.line } }, axisTick: { show: false },
        axisLabel: {
          lineHeight: 15,
          formatter: (date) => {
            const tag = statusTag[byDate[date].status];
            return "{d|" + dayName(date) + "}" + (tag ? "\n" + tag : "");
          },
          rich: {
            d: { color: P.fg, fontFamily: FONT(), fontSize: 13, fontWeight: 500 },
            w: { color: P.warn, fontFamily: FONT(), fontSize: 11 },
            p: { color: P.accent, fontFamily: FONT(), fontSize: 11 },
            e: { color: P.muted, fontFamily: FONT(), fontSize: 11 },
          },
        },
      },
      dataZoom: [
        {
          type: "slider", xAxisIndex: 0, filterMode: "weakFilter", height: 18, bottom: 10,
          startValue: span[0], endValue: span[1],
          labelFormatter: (v) => clock(v), textStyle: axisText(),
          borderColor: P.line, backgroundColor: "transparent", fillerColor: "rgba(127,140,150,0.18)",
          dataBackground: { lineStyle: { opacity: 0 }, areaStyle: { opacity: 0 } },
          handleStyle: { color: P.surface, borderColor: P.muted }, moveHandleStyle: { color: P.line },
        },
        { type: "inside", xAxisIndex: 0, filterMode: "weakFilter", zoomOnMouseWheel: "ctrl", moveOnMouseWheel: false, moveOnMouseMove: true },
      ],
      series: keys.map((c) => ({
        type: "custom", name: c.name, renderItem, encode: { x: [1, 2], y: 0 },
        itemStyle: { color: P.cat[c.name] }, data: items(c.name),
      })),
    });
  }

  function weekTotalsOption(data, size) {
    const narrow = size !== "wide";
    const keys = seriesOrder(data.categories);
    return Object.assign(base(), {
      tooltip: tip({
        trigger: "axis", axisPointer: shadowPointer,
        formatter: (ps) => {
          const d = data.days[ps[0].dataIndex];
          if (d.status === "future") return tipWrap(`<div style="font-weight:600">${dayName(d.date)}</div><div style="color:${P.muted}">not yet</div>`);
          return breakdown(dayName(d.date), d, keys, d.status === "unended" ? `<div style="color:${P.warn}">includes ${dur(d.toMidnight)} counted to 24:00</div>` : "");
        },
      }),
      grid: { left: narrow ? 64 : 92, right: narrow ? 120 : 200, top: 6, bottom: 26 },
      xAxis: {
        type: "value", axisLabel: Object.assign(axisText(), { formatter: (v) => v + "h" }),
        splitLine: { lineStyle: { color: P.line } }, axisLine: { show: false }, axisTick: { show: false },
      },
      yAxis: {
        type: "category", inverse: true, data: data.days.map((d) => dayName(d.date)),
        axisLine: { lineStyle: { color: P.line } }, axisTick: { show: false },
        axisLabel: { color: P.fg, fontFamily: FONT(), fontSize: 13, fontWeight: 500 },
      },
      series: stackedSeries(data.days, keys, (d) => d, {
        position: "right",
        label: (d) => {
          if (d.status === "future") return "{m|not yet}";
          if (d.worked + d.breaks === 0) return "{m|—}";
          const brk = d.breaks ? "{b| · " + dur(d.breaks) + (narrow ? "" : " break") + "}" : "";
          const tag = d.status === "unended" ? " {w|no @end}" : d.status === "inProgress" ? " {p|so far}" : "";
          return "{v|" + dur(d.worked) + "}" + brk + (narrow ? "" : tag);
        },
      }),
    });
  }

  // ---- Month: calendar, weeks, average week, trends ----
  function calendarOption(data, size) {
    const narrow = size !== "wide";
    const keys = seriesOrder(data.categories);
    const byDate = Object.fromEntries(data.days.map((d) => [d.date, d]));
    const past = data.days.filter((d) => d.status !== "future");
    const max = Math.max(1, Math.ceil(Math.max(0, ...past.map((d) => d.worked)) / 3600));
    const cellLabel = (p) => {
      const d = byDate[p.data.value[0]];
      return parseDate(d.date).getDate() + (d.worked > 0 ? "\n" + (narrow ? shortHours(d.worked) : dur(d.worked)) : "");
    };
    const label = { show: true, fontFamily: MONO(), fontSize: narrow ? 10 : 12, lineHeight: narrow ? 13 : 16, formatter: cellLabel };
    return Object.assign(base(), {
      tooltip: tip({
        trigger: "item",
        formatter: (p) => {
          const d = byDate[p.data.value[0]];
          if (d.status === "future") return tipWrap(`<div style="font-weight:600">${longDay(d.date)}</div><div style="color:${P.muted}">not yet</div>`);
          const extra = d.status === "unended" ? `<div style="color:${P.warn}">No @end: ${dur(d.toMidnight)} counted to 24:00</div>`
            : d.status === "inProgress" ? `<div style="color:${P.accent}">in progress</div>` : "";
          return breakdown(longDay(d.date), d, keys, extra);
        },
      }),
      visualMap: {
        show: true, type: "continuous", min: 0, max, seriesIndex: 0, orient: "horizontal", left: "center", bottom: 0,
        itemWidth: 12, itemHeight: narrow ? 160 : 220, calculable: false,
        text: [max + "h", "0h"], textStyle: axisText(), inRange: { color: [P.heatLo, P.heatHi] },
      },
      calendar: {
        orient: "vertical", range: data.period.key, top: 28, left: 8, right: 8, bottom: 48,
        cellSize: ["auto", "auto"],
        dayLabel: { firstDay: 1, nameMap: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"], color: P.muted, fontFamily: FONT(), fontSize: 12, margin: 8 },
        monthLabel: { show: false }, yearLabel: { show: false },
        splitLine: { show: false },
        itemStyle: { color: P.surface, borderColor: P.surface, borderWidth: 3 },
      },
      series: [{
        type: "heatmap", coordinateSystem: "calendar",
        data: past.map((d) => ({
          value: [d.date, hours(d.worked)],
          itemStyle: d.status === "unended" ? { borderColor: P.warn, borderWidth: 2.5, borderRadius: 4 } : { borderColor: P.surface, borderWidth: 3, borderRadius: 4 },
          label: { color: d.worked / 3600 > max * 0.55 ? P.onAccent : P.fg },
        })),
        label,
        emphasis: { itemStyle: { borderColor: P.fg, borderWidth: 2 } },
      }, {
        // The days to come: drawn, numbered, without a value.
        type: "heatmap", coordinateSystem: "calendar",
        data: data.days.filter((d) => d.status === "future").map((d) => ({ value: [d.date, 0] })),
        itemStyle: { color: P.surface, borderColor: P.surface, borderWidth: 3, borderRadius: 4 },
        label: Object.assign({}, label, { color: P.muted }),
        emphasis: { disabled: true },
      }],
    });
  }

  // roundTop is a round number at or above x for a value axis to end on,
  // one ECharts' own ticks land on: a multiple of the step it picks for
  // an axis from 0 to it, a round number near a fifth of it.
  function roundTop(x) {
    const step = (top) => {
      const e = Math.pow(10, Math.floor(Math.log10(top / 5))), f = top / 5 / e;
      return (f < 1.5 ? 1 : f < 2.5 ? 2 : f < 4 ? 3 : f < 7 ? 5 : 10) * e;
    };
    let top = x;
    for (let i = 0; i < 3; i++) {
      const s = step(top);
      top = +(Math.ceil(top / s - 1e-9) * s).toPrecision(12); // 1.2, not 1.2000000000000002
    }
    return top;
  }

  function weeksOption(data, size) {
    const narrow = size !== "wide", tiny = size === "tiny";
    const keys = seriesOrder(data.categories);
    const avg = data.summary.averageWeek;
    const avgHours = avg != null && avg > 0 ? hours(avg) : null;
    const series = stackedSeries(data.weeks, keys, (w) => w.totals, {
      position: "top", barWidth: "52%",
      label: (w) => w.totals.worked > 0 ? "{v|" + (tiny ? shortHours(w.totals.worked) : dur(w.totals.worked)) + "}" : "",
    });
    if (avgHours != null) {
      series[0].markLine = {
        silent: true, symbol: "none", data: [{ yAxis: avgHours }],
        lineStyle: { color: P.muted, type: "dashed", width: 1 },
        label: { show: !tiny, formatter: "avg " + dur(avg), position: "insideStartTop", color: P.muted, fontFamily: MONO(), fontSize: 11 },
      };
    }
    return Object.assign(base(), {
      tooltip: tip({
        trigger: "axis", axisPointer: shadowPointer,
        formatter: (ps) => {
          const w = data.weeks[ps[0].dataIndex];
          return breakdown(spanLabel(w.start, w.end) + (w.days < 7 ? ` <span style="color:${P.muted};font-weight:400">(${w.days} ${w.days === 1 ? "day" : "days"})</span>` : ""), w.totals, keys);
        },
      }),
      grid: { left: 40, right: 8, top: 24, bottom: 44 },
      xAxis: {
        type: "category", data: data.weeks.map((w) => (tiny ? spanLabel(w.start, w.end).replace(/ .*/, "") : spanLabel(w.start, w.end))
          + (w.days < 7 ? "\n" + w.days + (tiny ? "d" : w.days === 1 ? " day" : " days") : "\n ")),
        axisLine: { lineStyle: { color: P.line } }, axisTick: { show: false },
        axisLabel: { color: P.fg, fontFamily: FONT(), fontSize: narrow ? 11 : 12, lineHeight: 15, interval: 0 },
      },
      yAxis: {
        type: "value", axisLabel: Object.assign(axisText(), { formatter: (v) => v + "h" }),
        splitLine: { lineStyle: { color: P.line } },
        // ECharts doesn't stretch an axis to a markLine, and leaves out
        // one beyond it: when the average week is above every bar, the
        // axis goes past it, with room for its label. null is ECharts'
        // own top.
        max: (extent) => avgHours != null && avgHours > extent.max ? roundTop(avgHours * 1.05) : null,
      },
      series,
    });
  }

  function weekdayOption(data, size) {
    const narrow = size !== "wide";
    const keys = seriesOrder(data.categories);
    const zero = { worked: 0, breaks: 0, toMidnight: 0, categories: {} };
    const avg = (w) => w.average || zero;
    return Object.assign(base(), {
      tooltip: tip({
        trigger: "axis", axisPointer: shadowPointer,
        formatter: (ps) => {
          const w = data.weekdays[ps[0].dataIndex];
          if (!w.days) return tipWrap(`<div style="font-weight:600">${w.weekday}</div><div style="color:${P.muted}">none yet</div>`);
          return breakdown(w.weekday + ` <span style="color:${P.muted};font-weight:400">(average of ${w.days})</span>`, avg(w), keys);
        },
      }),
      grid: { left: 40, right: 8, top: 24, bottom: 28 },
      xAxis: {
        type: "category", data: data.weekdays.map((w) => w.weekday),
        axisLine: { lineStyle: { color: P.line } }, axisTick: { show: false },
        axisLabel: { color: P.fg, fontFamily: FONT(), fontSize: 12 },
      },
      yAxis: {
        type: "value", axisLabel: Object.assign(axisText(), { formatter: (v) => v + "h" }),
        splitLine: { lineStyle: { color: P.line } },
      },
      series: stackedSeries(data.weekdays, keys, avg, {
        position: "top", barWidth: "56%",
        label: (w) => avg(w).worked > 0 ? "{v|" + (narrow ? shortHours(avg(w).worked) : dur(avg(w).worked)) + "}" : "",
      }),
    });
  }

  function trendsOption(data, size) {
    const narrow = size !== "wide";
    const keys = seriesOrder(data.categories);
    const rolling = Object.fromEntries(data.rolling ? data.rolling.map((r) => [r.date, r.totals]) : []);
    const dates = data.days.map((d) => d.date);
    return Object.assign(base(), {
      legend: legend(keys),
      tooltip: tip({
        trigger: "axis", axisPointer: { type: "line", lineStyle: { color: P.muted, type: "dashed" } },
        formatter: (ps) => {
          const date = dates[ps[0].dataIndex];
          if (!rolling[date]) return tipWrap(`<div style="font-weight:600">${dayName(date)} ${shortMonth(date)}</div><div style="color:${P.muted}">not yet</div>`);
          return tipWrap(`<div style="font-weight:600">7 days to ${dayName(date)} ${shortMonth(date)}</div>`
            + ps.map((p) => tipRow(`<span style="color:${p.color}">●</span> ${esc(p.seriesName)}`, dur(timeIn(rolling[date], p.seriesName)))).join(""));
        },
      }),
      grid: { left: 44, right: 12, top: narrow ? 76 : 58, bottom: 28 },
      xAxis: {
        type: "category", boundaryGap: false, data: dates.map((d) => parseDate(d).getDate() + " " + shortMonth(d)),
        axisLine: { lineStyle: { color: P.line } }, axisTick: { show: false },
        axisLabel: Object.assign(axisText(), { interval: narrow ? 9 : 4 }),
      },
      yAxis: {
        type: "value", name: "h / week", nameTextStyle: axisText(), nameGap: 12,
        axisLabel: Object.assign(axisText(), { formatter: (v) => v + "h" }),
        splitLine: { lineStyle: { color: P.line } },
      },
      series: keys.map((c, i) => ({
        type: "line", name: c.name, smooth: 0.3, showSymbol: false, symbol: "circle", symbolSize: 6,
        lineStyle: { width: i === 0 && c.work ? 3 : 2, type: c.work ? "solid" : "dashed", color: P.cat[c.name] },
        itemStyle: { color: P.cat[c.name] },
        emphasis: { focus: "series" },
        data: dates.map((d) => rolling[d] ? hours(timeIn(rolling[d], c.name)) : null),
      })),
    });
  }

  // ---- Figures and tables ----
  const $ = (id) => document.getElementById(id);
  const stats = (rows) => rows.map(([k, v, cls]) => `<div class="${cls || ""}"><dt>${esc(k)}</dt><dd>${esc(v)}</dd></div>`).join("");
  const dayList = (dates) => dates.length ? dates.map(dayName).join(", ") : "none";

  function weekFigures(data) {
    const s = data.summary, t = s.totals;
    const range = weekLabel(data.period.start, data.period.end);
    $("title").textContent = range;
    document.title = "htt · " + range;
    $("week-intro").textContent = data.period.includesToday
      ? "This week so far: " + s.days + " of 7 days, to " + clock(data.nowMinute) + " today."
      : s.days ? "Monday to Sunday: " + s.workingDays + (s.workingDays === 1 ? " day" : " days") + " with worked time." : "This week is still to come.";
    $("week-stats").innerHTML = stats([
      ["Worked", dur(t.worked)],
      ["Breaks", dur(t.breaks)],
      ["Average working day", s.averageWorkingDay != null ? dur(s.averageWorkingDay) : "–"],
      ["Counted to midnight", dur(t.toMidnight), t.toMidnight > 0 ? "flag" : ""],
      ["Days without @end", dayList(s.unended), s.unended.length ? "flag" : ""],
    ]);
    $("week-totals").innerHTML = seriesOrder(data.categories).map((c) =>
      `<tr><td>${swatch(c)}</td><td>${esc(workText(c))}</td><td class="num">${dur(timeIn(t, c.name))}</td></tr>`).join("");
  }

  function monthFigures(data) {
    const s = data.summary, t = s.totals, p = data.period;
    const label = monthLabel(p.start);
    $("title").textContent = label;
    document.title = "htt · " + label;
    const days = data.days.length;
    $("month-intro").textContent = p.includesToday
      ? label + " so far: " + s.days + " of " + days + " days. The averages and trends are over the days so far."
      : s.days ? days + " days, " + s.workingDays + " with worked time." : label + " is still to come.";
    const uncat = data.categories.find((c) => c.uncategorised);
    $("month-stats").innerHTML = stats([
      ["Worked", dur(t.worked)],
      ["Average week", s.averageWeek != null ? dur(s.averageWeek) : "–"],
      ["Average working day", s.averageWorkingDay != null ? dur(s.averageWorkingDay) : "–"],
      ["Breaks", dur(t.breaks)],
      ["Uncategorised", s.uncategorisedShare != null ? percent(s.uncategorisedShare) + " of worked"
        : dur(timeIn(t, "uncategorised")) + (uncat && !uncat.work ? " (as " + uncat.countedAs + ")" : "")],
      ["Days without @end", dayList(s.unended), s.unended.length ? "flag" : ""],
    ]);
    $("weeks-hint").textContent = "Dashed: the average week" + (s.averageWeek ? ", " + dur(s.averageWeek) : "") + ".";
    $("weekday-hint").textContent = "Average time per weekday in " + monthName(p.start) + (p.includesToday ? ", so far." : ".");
    const [a, b] = data.halves;
    const halfLabel = (h, from, to) => "Per week, " + (h.days ? spanLabel(h.start, h.end) : from + "–" + to + " " + shortMonth(p.start));
    $("month-head").innerHTML = `<tr><th>Category</th><th class="num">${esc(monthName(p.start))}</th><th class="num">Share of worked</th>`
      + `<th class="num">${esc(halfLabel(a, 1, 15))}</th><th class="num">${esc(halfLabel(b, 16, parseDate(p.end).getDate()))}</th><th class="num">Change</th></tr>`;
    const rows = new Map(data.table.map((r) => [r.category, r]));
    $("month-totals").innerHTML = seriesOrder(data.categories).map((c) => {
      const r = rows.get(c.name);
      const change = r.change == null ? "–" : (r.change > 0 ? "+" : "") + percent(r.change);
      const cls = r.change == null ? "" : r.change >= 0 ? "up" : "down";
      return `<tr><td>${swatch(c)}</td><td class="num">${dur(r.total)}</td><td class="num">${r.shareOfWorked == null ? "–" : percent(r.shareOfWorked)}</td>`
        + `<td class="num">${r.perWeek[0] == null ? "–" : dur(r.perWeek[0])}</td><td class="num">${r.perWeek[1] == null ? "–" : dur(r.perWeek[1])}</td>`
        + `<td class="num ${cls}">${change}</td></tr>`;
    }).join("");
  }

  // ---- Charts: made when their view is first shown ----
  const CHARTS = {
    week: [["timeline", timelineOption, true], ["summary", weekTotalsOption]],
    month: [["calendar", calendarOption], ["weeks", weeksOption], ["weekday", weekdayOption], ["trends", trendsOption]],
  };
  const live = {}; // id -> echarts instance
  // A chart's size class: wide, narrow (under 560px) or tiny (under 420px).
  const sizeOf = (el) => el.clientWidth < 420 ? "tiny" : el.clientWidth < 560 ? "narrow" : "wide";
  let zoomed = null; // the timeline's zoom, kept while the same week is shown: [period key, from, to]

  const unpinned = $("detail").textContent; // what's below the timeline with no entry pinned

  function pinEntry(p) {
    if (!p.data || !p.data.iv || !state.data) return;
    const iv = p.data.iv, note = statusNote(iv, state.data);
    const c = state.data.categories.find((x) => x.name === seriesOf(iv));
    $("detail").innerHTML = `<strong>${esc(iv.entry)}</strong> · ${dayName(p.data.date)}, ${clock(iv.startMinute)}–${clock(iv.endMinute)} (${dur(iv.seconds)}) · ${c ? swatch(c) : esc(iv.category)}`
      + (note ? ` · <span class="flagtext">${note}</span>` : "");
  }

  function zoomWindow(chart) {
    const opt = chart.getOption();
    const z = opt && opt.dataZoom && opt.dataZoom[0];
    return z && z.startValue != null ? [z.startValue, z.endValue] : null;
  }

  function updateZoomButton() {
    const chart = live.timeline;
    if (!chart || !state.data || state.data.period.kind !== "week") return;
    const w = zoomWindow(chart);
    const whole = w && w[0] <= 0 && w[1] >= 1440;
    $("zoom").textContent = whole ? "Hours logged" : "Whole day";
  }

  function renderChart(id, build, keepZoom) {
    const el = $(id);
    if (!live[id]) {
      live[id] = echarts.init(el, null, { renderer: "svg" });
      if (id === "timeline") {
        live[id].on("click", pinEntry);
        live[id].on("datazoom", () => {
          const w = zoomWindow(live.timeline);
          if (w && state.data) zoomed = [state.data.period.key, w[0], w[1]];
          updateZoomButton();
        });
      }
    }
    const chart = live[id];
    chart.resize();
    const zoom = keepZoom && zoomed && zoomed[0] === state.data.period.key ? [zoomed[1], zoomed[2]] : null;
    chart.setOption(build(state.data, sizeOf(el), zoom), true);
    chart.__size = sizeOf(el);
  }

  function renderCharts() {
    if (!state.data) return;
    P = palette(state.data.categories);
    CHARTS[state.data.period.kind].forEach(([id, build, keepZoom]) => renderChart(id, build, keepZoom));
    updateZoomButton();
  }

  // ---- State, routes and loading ----
  // shown is the period on the page, kind/key: refreshing it keeps the
  // pinned entry, another clears it.
  const state = { data: null, seq: 0, last: { week: "#week", month: "#month" }, shown: "" };

  function route() {
    const [view, key] = location.hash.replace(/^#/, "").split("/");
    if (view === "month") return { view, key: /^\d{4}-\d{2}$/.test(key || "") ? key : "" };
    return { view: "week", key: /^\d{4}-\d{2}-\d{2}$/.test(key || "") ? key : "" };
  }

  function showError(title, text) {
    $("error-title").textContent = title;
    $("error-text").textContent = text;
    $("error").hidden = false;
    ["week", "month"].forEach((v) => { $("view-" + v).hidden = true; });
    $("no-categories").hidden = true;
    ["previous", "current", "next"].forEach((id) => { $(id).disabled = true; });
  }

  function showTabs(view) {
    ["week", "month"].forEach((v) => $("tab-" + v).setAttribute("aria-selected", String(v === view)));
    const unit = view === "week" ? "week" : "month";
    $("current").textContent = "This " + unit;
    $("previous").setAttribute("aria-label", "Previous " + unit);
    $("next").setAttribute("aria-label", "Next " + unit);
  }

  async function load() {
    const { view, key } = route();
    showTabs(view);
    state.last[view] = location.hash || "#" + view;
    const seq = ++state.seq;
    const url = view === "week" ? "api/week" + (key ? "?date=" + key : "") : "api/month" + (key ? "?month=" + key : "");
    let resp, body;
    try {
      resp = await fetch(url, { cache: "no-store" });
      body = await resp.json();
    } catch (e) {
      if (seq === state.seq) showError("htt gui isn't answering", "Is it still running? Start it again with htt gui, and open the address it prints. (" + e.message + ")");
      return;
    }
    if (seq !== state.seq) return; // a newer load is under way
    if (!resp.ok) {
      showError("htt can't show the time log", body.error || resp.status + " " + resp.statusText);
      return;
    }
    state.data = body;
    show();
  }

  function show() {
    const data = state.data, p = data.period;
    if (state.shown !== p.kind + "/" + p.key) {
      state.shown = p.kind + "/" + p.key;
      $("detail").textContent = unpinned;
    }
    $("error").hidden = true;
    $("no-categories").hidden = data.categories.some((c) => !c.uncategorised);
    ["week", "month"].forEach((v) => { $("view-" + v).hidden = v !== p.kind; });
    $("previous").disabled = false;
    $("current").disabled = p.includesToday;
    $("next").disabled = p.includesToday || p.start > data.today;
    P = palette(data.categories);
    if (p.kind === "week") weekFigures(data); else monthFigures(data);
    renderCharts();
    $("updated").textContent = "Updated at " + new Date().toLocaleTimeString("en-GB")
      + (p.includesToday ? "; it refreshes every minute while it shows today." : ".");
  }

  function go(hash) {
    if (location.hash === hash) load(); else location.hash = hash;
  }
  $("tab-week").addEventListener("click", () => go(state.last.week));
  $("tab-month").addEventListener("click", () => go(state.last.month));
  $("previous").addEventListener("click", () => state.data && go("#" + state.data.period.kind + "/" + state.data.period.previous));
  $("next").addEventListener("click", () => state.data && go("#" + state.data.period.kind + "/" + state.data.period.next));
  $("current").addEventListener("click", () => go("#" + route().view));
  $("zoom").addEventListener("click", () => {
    const chart = live.timeline;
    if (!chart || !state.data) return;
    const w = zoomWindow(chart);
    const whole = w && w[0] <= 0 && w[1] >= 1440;
    const [from, to] = whole ? [state.data.hours.from, state.data.hours.to] : [0, 1440];
    chart.dispatchAction({ type: "dataZoom", dataZoomIndex: 0, startValue: from, endValue: to });
  });
  window.addEventListener("hashchange", load);

  // Fresh data: when the page is seen again, and every minute while the
  // period shown includes today.
  document.addEventListener("visibilitychange", () => { if (!document.hidden) load(); });
  setInterval(() => {
    if (!document.hidden && state.data && state.data.period.includesToday) load();
  }, 60000);

  new ResizeObserver(() => {
    if (!state.data) return;
    let redo = false;
    CHARTS[state.data.period.kind].forEach(([id]) => {
      const chart = live[id];
      if (!chart) return;
      if (chart.__size !== sizeOf($(id))) redo = true;
      chart.resize();
    });
    if (redo) renderCharts();
  }).observe(document.querySelector(".page"));

  // The charts read the colours when drawn: draw again when they change.
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", renderCharts);
  new MutationObserver(renderCharts).observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

  if (typeof echarts === "undefined") {
    showError("The page couldn't load its charts", "vendor/echarts/echarts.min.js didn't load.");
    return;
  }
  // Text is measured as it's laid out: wait for the fonts.
  (document.fonts ? document.fonts.ready : Promise.resolve()).then(load);
})();
