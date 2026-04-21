// Repo-owned shim for templui popovers/selectboxes.
// Templui portals popovers outside the swapped subtree, which can leave an old
// open dropdown visually covering freshly swapped HTMX content. This script
// closes popovers before HTMX requests/cleanup/swap and removes orphaned open
// popovers after DOM mutations, without patching templui-owned assets.
(function () {
	"use strict";

	if (window.diwisePopoverHTMXFix) {
		return;
	}
	window.diwisePopoverHTMXFix = true;

	function closePopover(id) {
		if (!id) {
			return;
		}

		if (window.tui && window.tui.popover && typeof window.tui.popover.close === "function") {
			window.tui.popover.close(id);
			return;
		}

		if (typeof window.closePopover === "function") {
			window.closePopover(id);
		}
	}

	function removePopoverContents(id) {
		if (!id) {
			return;
		}

		document.querySelectorAll(`[data-tui-popover-id="${id}"]`).forEach((popover) => {
			popover.remove();
		});
	}

	function openPopovers() {
		return document.querySelectorAll('[data-tui-popover-open="true"][data-tui-popover-id]');
	}

	function hasAnyTrigger(id) {
		return document.querySelector(`[data-tui-popover-trigger="${id}"]`) !== null;
	}

	function elementTouchesPopover(id, element) {
		if (!id || !element || typeof element.contains !== "function") {
			return false;
		}

		const content = document.getElementById(id);
		if (content && (element === content || element.contains(content) || content.contains(element))) {
			return true;
		}

		return Array.from(document.querySelectorAll(`[data-tui-popover-trigger="${id}"]`)).some((trigger) => {
			return element === trigger || element.contains(trigger) || trigger.contains(element);
		});
	}

	function closeOrphanedPopovers() {
		openPopovers().forEach((popover) => {
			const id = popover.id;
			if (!id) {
				return;
			}

			if (!popover.isConnected || !hasAnyTrigger(id)) {
				closePopover(id);
			}
		});
	}

	function closePopoversForElement(element) {
		if (!element) {
			return;
		}

		openPopovers().forEach((popover) => {
			if (popover.id && elementTouchesPopover(popover.id, element)) {
				closePopover(popover.id);
			}
		});
	}

	function canQueryWithin(element) {
		return element && typeof element.querySelectorAll === "function";
	}

	function matchesSelector(element, selector) {
		return element && typeof element.matches === "function" && element.matches(selector);
	}

	function removePopoversForElement(element) {
		if (!element) {
			return;
		}

		const ids = new Set();

		if (matchesSelector(element, "[data-tui-popover-trigger]")) {
			ids.add(element.getAttribute("data-tui-popover-trigger") || "");
		}

		if (canQueryWithin(element)) {
			element.querySelectorAll("[data-tui-popover-trigger]").forEach((trigger) => {
				ids.add(trigger.getAttribute("data-tui-popover-trigger") || "");
			});
		}

		if (matchesSelector(element, "[data-tui-popover-id]")) {
			ids.add(element.getAttribute("data-tui-popover-id") || "");
		}

		if (canQueryWithin(element)) {
			element.querySelectorAll("[data-tui-popover-id]").forEach((popover) => {
				ids.add(popover.getAttribute("data-tui-popover-id") || "");
			});
		}

		ids.forEach((id) => {
			if (!id) {
				return;
			}
			closePopover(id);
			removePopoverContents(id);
		});
	}

	function swapTargetFromEvent(event) {
		if (!event || !event.detail) {
			return null;
		}

		return event.detail.target || event.detail.elt || null;
	}

	function resetSelectboxTarget(element) {
		if (!element || typeof element.querySelector !== "function") {
			return;
		}

		if (!element.querySelector("[data-tui-popover-trigger]")) {
			return;
		}

		element.innerHTML = "";
	}

	function init() {
		const body = document.body;
		if (!body) {
			return;
		}

		body.addEventListener("htmx:beforeSwap", function (event) {
			openPopovers().forEach((popover) => closePopover(popover.id));
			const target = swapTargetFromEvent(event);
			removePopoversForElement(target);
			resetSelectboxTarget(target);
		});

		body.addEventListener("htmx:beforeRequest", function () {
			// Close any open dropdown immediately for visual feedback.
			openPopovers().forEach((popover) => closePopover(popover.id));
		});

		body.addEventListener("htmx:beforeCleanupElement", function (event) {
			const element = event.detail && event.detail.elt;
			closePopoversForElement(element);
			removePopoversForElement(element);
		});

		body.addEventListener("htmx:afterSwap", closeOrphanedPopovers);

		new MutationObserver(closeOrphanedPopovers).observe(body, {
			childList: true,
			subtree: true,
		});

		closeOrphanedPopovers();
	}

	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", init);
	} else {
		init();
	}
})();
