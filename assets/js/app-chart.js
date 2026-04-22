(function () {
  "use strict";

  var resizeObservers = new WeakMap();

  function parseJSONNode(id) {
    if (!id) {
      return null;
    }

    var node = document.getElementById(id);
    if (!node) {
      return null;
    }

    try {
      return JSON.parse(node.textContent || "{}");
    } catch (_err) {
      return null;
    }
  }

  function canvasReady(canvas) {
    if (!canvas || !canvas.isConnected) {
      return false;
    }

    var rect = canvas.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  }

  function ensureResizeObserver(canvas) {
    if (!canvas || typeof Chart === "undefined" || typeof Chart.getChart !== "function") {
      return;
    }

    if (resizeObservers.has(canvas) || typeof ResizeObserver === "undefined") {
      return;
    }

    var resizeObserver = new ResizeObserver(function () {
      var chart = Chart.getChart(canvas);
      if (chart && canvasReady(canvas)) {
        chart.resize();
        chart.update("none");
        return;
      }

      initChart(canvas, 0);
    });

    resizeObserver.observe(canvas);
    if (canvas.parentElement) {
      resizeObserver.observe(canvas.parentElement);
    }
    resizeObservers.set(canvas, resizeObserver);
  }

  function initChart(canvas, attempt) {
    if (!canvas || !canvas.dataset || typeof Chart === "undefined") {
      return;
    }

    var config = parseJSONNode(canvas.dataset.appChartId);
    if (!config) {
      return;
    }

    if (typeof Chart.getChart !== "function") {
      if ((attempt || 0) < 10) {
        setTimeout(function () {
          initChart(canvas, (attempt || 0) + 1);
        }, 50);
      }
      return;
    }

    if (!canvasReady(canvas)) {
      ensureResizeObserver(canvas);
      return;
    }

    var existing = Chart.getChart(canvas);
    if (existing) {
      existing.destroy();
    }

    try {
      new Chart(canvas, config);
      ensureResizeObserver(canvas);
    } catch (err) {
      var needsDateAdapter =
        err &&
        typeof err.message === "string" &&
        err.message.indexOf("date adapter") >= 0;

      if (needsDateAdapter && (attempt || 0) < 10) {
        setTimeout(function () {
          initChart(canvas, (attempt || 0) + 1);
        }, 50);
        return;
      }

      throw err;
    }
  }

  function canvases(root) {
    if (!root) {
      return [];
    }

    if (root.matches && root.matches("canvas[data-app-chart-id]")) {
      return [root];
    }

    if (!root.querySelectorAll) {
      return [];
    }

    return Array.prototype.slice.call(root.querySelectorAll("canvas[data-app-chart-id]"));
  }

  function refresh(root) {
    canvases(root).forEach(function (canvas) {
      requestAnimationFrame(function () {
        initChart(canvas, 0);
      });
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    refresh(document);
  });

  document.body.addEventListener("htmx:afterSettle", function (event) {
    var target = event && event.detail ? event.detail.target : document;
    refresh(target || document);
  });
})();
