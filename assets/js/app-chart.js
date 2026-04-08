(function () {
  "use strict";

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

    var existing = Chart.getChart(canvas);
    if (existing) {
      existing.destroy();
    }

    try {
      new Chart(canvas, config);
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
      initChart(canvas, 0);
    });
  }

  document.addEventListener("DOMContentLoaded", function () {
    setTimeout(function () {
      refresh(document);
    }, 0);
  });

  document.body.addEventListener("htmx:afterSwap", function (event) {
    var target = event && event.detail ? event.detail.target : document;
    setTimeout(function () {
      refresh(target || document);
    }, 0);
  });
})();
