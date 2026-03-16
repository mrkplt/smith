if (typeof window !== 'undefined' && typeof window.matchMedia !== 'function') {
	Object.defineProperty(window, 'matchMedia', {
		writable: true,
		value: (query: string) => ({
			matches: false,
			media: query,
			onchange: null,
			addListener: () => undefined,
			removeListener: () => undefined,
			addEventListener: () => undefined,
			removeEventListener: () => undefined,
			dispatchEvent: () => false
		})
	});
}

if (typeof window !== 'undefined' && typeof HTMLDialogElement !== 'undefined') {
	if (!HTMLDialogElement.prototype.showModal) {
		HTMLDialogElement.prototype.showModal = function showModal() {
			this.open = true;
		};
	}
	if (!HTMLDialogElement.prototype.show) {
		HTMLDialogElement.prototype.show = function show() {
			this.open = true;
		};
	}
	if (!HTMLDialogElement.prototype.close) {
		HTMLDialogElement.prototype.close = function close() {
			this.open = false;
		};
	}
}

if (typeof Element !== 'undefined' && !Element.prototype.animate) {
	Element.prototype.animate = function animate() {
		return {
			finished: Promise.resolve(),
			cancel: () => undefined,
			play: () => undefined,
			pause: () => undefined,
			reverse: () => undefined
		} as unknown as Animation;
	};
}
