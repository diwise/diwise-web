(function () {
	"use strict";

	if (window.diwiseCustomSelectboxRemote) {
		return;
	}

	const stateByContentID = new Map();

	function splitValues(value, multiple) {
		if (!value) {
			return [];
		}

		if (!multiple) {
			return [String(value)];
		}

		return String(value)
			.split(",")
			.map((part) => part.trim())
			.filter(Boolean);
	}

	function joinValues(values) {
		return values.join(",");
	}

	function getInputValueSetter() {
		return Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set || null;
	}

	function setHiddenInputValue(hiddenInput, value) {
		const setter = getInputValueSetter();
		if (setter) {
			setter.call(hiddenInput, value);
			return;
		}

		hiddenInput.value = value;
	}

	function dispatchHiddenInputEvents(hiddenInput) {
		hiddenInput.dispatchEvent(new Event("input", { bubbles: true }));
		hiddenInput.dispatchEvent(new Event("change", { bubbles: true }));
	}

	function findTriggerFromContentID(contentID) {
		if (!contentID) {
			return null;
		}

		const selector = `button.select-trigger[data-tui-selectbox-content-id="${contentID}"]`;
		return document.querySelector(selector);
	}

	function getContentFromTrigger(trigger) {
		if (!(trigger instanceof HTMLElement)) {
			return null;
		}

		const contentID = trigger.getAttribute("data-tui-selectbox-content-id") || "";
		if (!contentID) {
			return null;
		}

		const content = document.getElementById(contentID);
		return content instanceof HTMLElement ? content : null;
	}

	function getOptionsRootFromContent(content) {
		if (!(content instanceof HTMLElement)) {
			return null;
		}

		const root = content.querySelector("[data-diwise-selectbox-options-root]");
		return root instanceof HTMLElement ? root : null;
	}

	function findRemoteSearchInputFromTrigger(trigger) {
		const content = getContentFromTrigger(trigger);
		if (!(content instanceof HTMLElement)) {
			return null;
		}

		if (content.getAttribute("data-diwise-selectbox-remote") !== "true") {
			return null;
		}

		const searchInput = content.querySelector("[data-diwise-selectbox-remote-input]");
		return searchInput instanceof HTMLInputElement ? searchInput : null;
	}

	function focusRemoteSearchInput(trigger) {
		const searchInput = findRemoteSearchInputFromTrigger(trigger);
		if (!(searchInput instanceof HTMLInputElement)) {
			return;
		}

		requestAnimationFrame(function () {
			searchInput.focus();
		});
	}

	function getRemoteContextFromNode(node) {
		const element = node instanceof HTMLElement ? node : null;
		const item = element ? element.closest(".select-item") : null;
		const trigger = element ? element.closest("button.select-trigger") : null;
		const optionsRoot = element?.matches?.("[data-diwise-selectbox-options-root]")
			? element
			: element?.closest?.("[data-diwise-selectbox-options-root]");
		const content =
			item?.closest("[data-diwise-selectbox-remote='true']") ||
			optionsRoot?.closest("[data-diwise-selectbox-remote='true']") ||
			getContentFromTrigger(trigger);

		if (!(content instanceof HTMLElement) || content.getAttribute("data-diwise-selectbox-remote") !== "true") {
			return null;
		}

		const contentID = content.id || "";
		const resolvedTrigger = trigger instanceof HTMLElement ? trigger : findTriggerFromContentID(contentID);
		if (!(resolvedTrigger instanceof HTMLElement)) {
			return null;
		}

		const hiddenInput = resolvedTrigger.querySelector("[data-tui-selectbox-hidden-input]");
		if (!(hiddenInput instanceof HTMLInputElement)) {
			return null;
		}

		const resolvedOptionsRoot = optionsRoot instanceof HTMLElement ? optionsRoot : getOptionsRootFromContent(content);
		if (!(resolvedOptionsRoot instanceof HTMLElement)) {
			return null;
		}

		return {
			content: content,
			contentID: contentID,
			hiddenInput: hiddenInput,
			item: item instanceof HTMLElement ? item : null,
			multiple: resolvedTrigger.getAttribute("data-tui-selectbox-multiple") === "true",
			optionsRoot: resolvedOptionsRoot,
			trigger: resolvedTrigger,
		};
	}

	function getState(contentID) {
		let state = stateByContentID.get(contentID);
		if (!state) {
			state = {
				labels: new Map(),
				values: [],
			};
			stateByContentID.set(contentID, state);
		}
		return state;
	}

	function getItemPrimaryLabel(item) {
		if (!(item instanceof HTMLElement)) {
			return "";
		}

		const text = item.querySelector(".select-item-text")?.textContent || "";
		return text.trim();
	}

	function seedLabelsFromOptions(optionsRoot, state) {
		optionsRoot.querySelectorAll(".select-item").forEach(function (item) {
			if (!(item instanceof HTMLElement)) {
				return;
			}

			const value = item.getAttribute("data-tui-selectbox-value") || "";
			const label = getItemPrimaryLabel(item);
			if (!value || !label) {
				return;
			}

			state.labels.set(value, label);
		});
	}

	function removePreservedItems(optionsRoot) {
		optionsRoot.querySelectorAll("[data-diwise-selectbox-preserved='true']").forEach(function (item) {
			item.remove();
		});
	}

	function createPreservedItem(value, label) {
		const item = document.createElement("div");
		item.className = "select-item";
		item.setAttribute("data-diwise-selectbox-preserved", "true");
		item.setAttribute("data-tui-selectbox-disabled", "false");
		item.setAttribute("data-tui-selectbox-selected", "true");
		item.setAttribute("data-tui-selectbox-value", value);
		item.setAttribute("role", "option");
		item.setAttribute("tabindex", "-1");
		item.style.display = "none";

		const text = document.createElement("div");
		text.className = "select-item-text";
		text.textContent = label || value;
		item.appendChild(text);

		return item;
	}

	function syncRemoteMultiSelect(context) {
		const state = getState(context.contentID);
		const nextValues = splitValues(context.hiddenInput.value, true);
		const nextValueSet = new Set(nextValues);

		state.values = nextValues;
		seedLabelsFromOptions(context.optionsRoot, state);

		for (const value of Array.from(state.labels.keys())) {
			if (!nextValueSet.has(value)) {
				state.labels.delete(value);
			}
		}

		removePreservedItems(context.optionsRoot);

		const visibleValues = new Set();
		context.optionsRoot.querySelectorAll(".select-item").forEach(function (item) {
			if (!(item instanceof HTMLElement)) {
				return;
			}

			const value = item.getAttribute("data-tui-selectbox-value") || "";
			if (!value) {
				return;
			}

			visibleValues.add(value);
			item.setAttribute("data-tui-selectbox-selected", nextValueSet.has(value).toString());
		});

		nextValues.forEach(function (value) {
			if (visibleValues.has(value)) {
				return;
			}

			context.optionsRoot.appendChild(createPreservedItem(value, state.labels.get(value) || value));
		});
	}

	function syncRemoteSingleSelect(context) {
		const selectedValues = new Set(splitValues(context.hiddenInput.value, false));

		context.optionsRoot.querySelectorAll(".select-item").forEach(function (item) {
			if (!(item instanceof HTMLElement)) {
				return;
			}

			const value = item.getAttribute("data-tui-selectbox-value") || "";
			item.setAttribute("data-tui-selectbox-selected", selectedValues.has(value).toString());
		});
	}

	function syncOptionSelection(root) {
		const context = getRemoteContextFromNode(root);
		if (!context) {
			return;
		}

		if (context.multiple) {
			syncRemoteMultiSelect(context);
			return;
		}

		syncRemoteSingleSelect(context);
	}

	function syncWithin(root) {
		if (!root || typeof root.querySelectorAll !== "function") {
			return;
		}

		if (root.matches?.("[data-diwise-selectbox-options-root]")) {
			syncOptionSelection(root);
		}

		root.querySelectorAll("[data-diwise-selectbox-options-root]").forEach(syncOptionSelection);
	}

	function updateRemoteMultiSelectValue(context, nextValues) {
		const state = getState(context.contentID);
		const nextValueSet = new Set(nextValues);

		state.values = nextValues.slice();
		for (const value of Array.from(state.labels.keys())) {
			if (!nextValueSet.has(value)) {
				state.labels.delete(value);
			}
		}

		setHiddenInputValue(context.hiddenInput, joinValues(nextValues));
		syncRemoteMultiSelect(context);
		dispatchHiddenInputEvents(context.hiddenInput);

		const finalValue = joinValues(state.values);
		if (context.hiddenInput.value !== finalValue) {
			setHiddenInputValue(context.hiddenInput, finalValue);
		}
	}

	function handleRemoteMultiSelectItemClick(event, item) {
		const context = getRemoteContextFromNode(item);
		if (!context || !context.multiple || !(context.item instanceof HTMLElement)) {
			return false;
		}

		event.preventDefault();
		event.stopImmediatePropagation();

		const value = context.item.getAttribute("data-tui-selectbox-value") || "";
		if (!value || context.item.getAttribute("data-tui-selectbox-disabled") === "true") {
			return true;
		}

		const state = getState(context.contentID);
		const label = getItemPrimaryLabel(context.item);
		if (label) {
			state.labels.set(value, label);
		}

		const currentValues = splitValues(context.hiddenInput.value, true);
		const nextValues = currentValues.filter(function (currentValue) {
			return currentValue !== value;
		});

		if (nextValues.length === currentValues.length) {
			nextValues.push(value);
		}

		updateRemoteMultiSelectValue(context, nextValues);
		return true;
	}

	function handleRemoteMultiSelectPillRemove(event, button) {
		const context = getRemoteContextFromNode(button);
		if (!context || !context.multiple) {
			return false;
		}

		const value = button.getAttribute("data-tui-selectbox-value") || "";
		if (!value) {
			return false;
		}

		event.preventDefault();
		event.stopImmediatePropagation();

		const currentValues = splitValues(context.hiddenInput.value, true);
		const nextValues = currentValues.filter(function (currentValue) {
			return currentValue !== value;
		});

		updateRemoteMultiSelectValue(context, nextValues);
		return true;
	}

	function init() {
		document.querySelectorAll("[data-diwise-selectbox-options-root]").forEach(syncOptionSelection);

		document.addEventListener(
			"click",
			function (event) {
				const pillRemove = event.target.closest("[data-tui-selectbox-pill-remove]");
				if (pillRemove instanceof HTMLElement && handleRemoteMultiSelectPillRemove(event, pillRemove)) {
					return;
				}

				const item = event.target.closest(".select-item");
				if (item instanceof HTMLElement) {
					handleRemoteMultiSelectItemClick(event, item);
				}
			},
			true,
		);

		document.addEventListener("click", function (event) {
			const trigger = event.target.closest("button.select-trigger");
			if (!(trigger instanceof HTMLElement)) {
				return;
			}

			focusRemoteSearchInput(trigger);
		});

		document.addEventListener("input", function (event) {
			if (!(event.target instanceof HTMLInputElement) || !event.target.matches("[data-tui-selectbox-hidden-input]")) {
				return;
			}

			const context = getRemoteContextFromNode(event.target);
			if (!context || !context.multiple) {
				return;
			}

			syncRemoteMultiSelect(context);
		});

		document.body.addEventListener("htmx:afterSwap", function (event) {
			const target = event.detail?.target;
			syncWithin(target);

			if (!(target instanceof HTMLElement) || !target.matches("[data-diwise-selectbox-options-root]")) {
				return;
			}

			const contentID = target.getAttribute("data-diwise-selectbox-content-id") || "";
			focusRemoteSearchInput(findTriggerFromContentID(contentID));
		});
	}

	window.diwiseCustomSelectboxRemote = {
		syncWithin: syncWithin,
	};

	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", init);
	} else {
		init();
	}
})();
