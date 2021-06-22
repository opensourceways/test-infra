var __values = this && this.__values || function (e) {
    var t = "function" == typeof Symbol && e[Symbol.iterator], o = 0;
    return t ? t.call(e) : {
        next: function () {
            return e && o >= e.length && (e = void 0), {value: e && e[o++], done: !e}
        }
    }
}, __read = this && this.__read || function (e, t) {
    var o = "function" == typeof Symbol && e[Symbol.iterator];
    if (!o) return e;
    var n, i, a = o.call(e), r = [];
    try {
        for (; (void 0 === t || t-- > 0) && !(n = a.next()).done;) r.push(n.value)
    } catch (e) {
        i = {error: e}
    } finally {
        try {
            n && !n.done && (o = a.return) && o.call(a)
        } finally {
            if (i) throw i.error
        }
    }
    return r
}, __spread = this && this.__spread || function () {
    for (var e = [], t = 0; t < arguments.length; t++) e = e.concat(__read(arguments[t]));
    return e
};
!function () {
    "use strict";
    var e = window.CustomEvent;

    function t(e) {
        for (; e;) {
            if ("dialog" === e.localName) return e;
            e = e.parentElement
        }
        return null
    }

    function o(e) {
        e && e.blur && e !== document.body && e.blur()
    }

    function n(e, t) {
        for (var o = 0; o < e.length; ++o) if (e[o] === t) return !0;
        return !1
    }

    function i(e) {
        return !(!e || !e.hasAttribute("method")) && "dialog" === e.getAttribute("method").toLowerCase()
    }

    function a(e) {
        if (this.dialog_ = e, this.replacedStyleTop_ = !1, this.openAsModal_ = !1, e.hasAttribute("role") || e.setAttribute("role", "dialog"), e.show = this.show.bind(this), e.showModal = this.showModal.bind(this), e.close = this.close.bind(this), "returnValue" in e || (e.returnValue = ""), "MutationObserver" in window) new MutationObserver(this.maybeHideModal.bind(this)).observe(e, {
            attributes: !0,
            attributeFilter: ["open"]
        }); else {
            var t, o = !1, n = function () {
                o ? this.downgradeModal() : this.maybeHideModal(), o = !1
            }.bind(this), i = function (i) {
                if (i.target === e) {
                    var a = "DOMNodeRemoved";
                    o |= i.type.substr(0, a.length) === a, window.clearTimeout(t), t = window.setTimeout(n, 0)
                }
            };
            ["DOMAttrModified", "DOMNodeRemoved", "DOMNodeRemovedFromDocument"].forEach(function (t) {
                e.addEventListener(t, i)
            })
        }
        Object.defineProperty(e, "open", {
            set: this.setOpen.bind(this),
            get: e.hasAttribute.bind(e, "open")
        }), this.backdrop_ = document.createElement("div"), this.backdrop_.className = "backdrop", this.backdrop_.addEventListener("click", this.backdropClick_.bind(this))
    }

    e && "object" != typeof e || ((e = function e(t, o) {
        o = o || {};
        var n = document.createEvent("CustomEvent");
        return n.initCustomEvent(t, !!o.bubbles, !!o.cancelable, o.detail || null), n
    }).prototype = window.Event.prototype), a.prototype = {
        get dialog() {
            return this.dialog_
        }, maybeHideModal: function () {
            this.dialog_.hasAttribute("open") && document.body.contains(this.dialog_) || this.downgradeModal()
        }, downgradeModal: function () {
            this.openAsModal_ && (this.openAsModal_ = !1, this.dialog_.style.zIndex = "", this.replacedStyleTop_ && (this.dialog_.style.top = "", this.replacedStyleTop_ = !1), this.backdrop_.parentNode && this.backdrop_.parentNode.removeChild(this.backdrop_), r.dm.removeDialog(this))
        }, setOpen: function (e) {
            e ? this.dialog_.hasAttribute("open") || this.dialog_.setAttribute("open", "") : (this.dialog_.removeAttribute("open"), this.maybeHideModal())
        }, backdropClick_: function (e) {
            if (this.dialog_.hasAttribute("tabindex")) this.dialog_.focus(); else {
                var t = document.createElement("div");
                this.dialog_.insertBefore(t, this.dialog_.firstChild), t.tabIndex = -1, t.focus(), this.dialog_.removeChild(t)
            }
            var o = document.createEvent("MouseEvents");
            o.initMouseEvent(e.type, e.bubbles, e.cancelable, window, e.detail, e.screenX, e.screenY, e.clientX, e.clientY, e.ctrlKey, e.altKey, e.shiftKey, e.metaKey, e.button, e.relatedTarget), this.dialog_.dispatchEvent(o), e.stopPropagation()
        }, focus_: function () {
            var e = this.dialog_.querySelector("[autofocus]:not([disabled])");
            if (!e && this.dialog_.tabIndex >= 0 && (e = this.dialog_), !e) {
                var t = ["button", "input", "keygen", "select", "textarea"].map(function (e) {
                    return e + ":not([disabled])"
                });
                t.push('[tabindex]:not([disabled]):not([tabindex=""])'), e = this.dialog_.querySelector(t.join(", "))
            }
            o(document.activeElement), e && e.focus()
        }, updateZIndex: function (e, t) {
            if (e < t) throw new Error("dialogZ should never be < backdropZ");
            this.dialog_.style.zIndex = e, this.backdrop_.style.zIndex = t
        }, show: function () {
            this.dialog_.open || (this.setOpen(!0), this.focus_())
        }, showModal: function () {
            if (this.dialog_.hasAttribute("open")) throw new Error("Failed to execute 'showModal' on dialog: The element is already open, and therefore cannot be opened modally.");
            if (!document.body.contains(this.dialog_)) throw new Error("Failed to execute 'showModal' on dialog: The element is not in a Document.");
            if (!r.dm.pushDialog(this)) throw new Error("Failed to execute 'showModal' on dialog: There are too many open modal dialogs.");
            (function e(t) {
                for (; t && t !== document.body;) {
                    var o = window.getComputedStyle(t), n = function (e, t) {
                        return !(void 0 === o[e] || o[e] === t)
                    };
                    if (o.opacity < 1 || n("zIndex", "auto") || n("transform", "none") || n("mixBlendMode", "normal") || n("filter", "none") || n("perspective", "none") || "isolate" === o.isolation || "fixed" === o.position || "touch" === o.webkitOverflowScrolling) return !0;
                    t = t.parentElement
                }
                return !1
            })(this.dialog_.parentElement) && console.warn("A dialog is being shown inside a stacking context. This may cause it to be unusable. For more information, see this link: https://github.com/GoogleChrome/dialog-polyfill/#stacking-context"), this.setOpen(!0), this.openAsModal_ = !0, r.needsCentering(this.dialog_) ? (r.reposition(this.dialog_), this.replacedStyleTop_ = !0) : this.replacedStyleTop_ = !1, this.dialog_.parentNode.insertBefore(this.backdrop_, this.dialog_.nextSibling), this.focus_()
        }, close: function (t) {
            if (!this.dialog_.hasAttribute("open")) throw new Error("Failed to execute 'close' on dialog: The element does not have an 'open' attribute, and therefore cannot be closed.");
            this.setOpen(!1), void 0 !== t && (this.dialog_.returnValue = t);
            var o = new e("close", {bubbles: !1, cancelable: !1});
            this.dialog_.dispatchEvent(o)
        }
    };
    var r = {
        reposition: function (e) {
            var t = document.body.scrollTop || document.documentElement.scrollTop,
                o = t + (window.innerHeight - e.offsetHeight) / 2;
            e.style.top = Math.max(t, o) + "px"
        }, isInlinePositionSetByStylesheet: function (e) {
            for (var t = 0; t < document.styleSheets.length; ++t) {
                var o = document.styleSheets[t], i = null;
                try {
                    i = o.cssRules
                } catch (e) {
                }
                if (i) for (var a = 0; a < i.length; ++a) {
                    var r = i[a], l = null;
                    try {
                        l = document.querySelectorAll(r.selectorText)
                    } catch (e) {
                    }
                    if (l && n(l, e)) {
                        var d = r.style.getPropertyValue("top"), s = r.style.getPropertyValue("bottom");
                        if (d && "auto" !== d || s && "auto" !== s) return !0
                    }
                }
            }
            return !1
        }, needsCentering: function (e) {
            return !("absolute" !== window.getComputedStyle(e).position || "auto" !== e.style.top && "" !== e.style.top || "auto" !== e.style.bottom && "" !== e.style.bottom || r.isInlinePositionSetByStylesheet(e))
        }, forceRegisterDialog: function (e) {
            if ((window.HTMLDialogElement || e.showModal) && console.warn("This browser already supports <dialog>, the polyfill may not work correctly", e), "dialog" !== e.localName) throw new Error("Failed to register dialog: The element is not a dialog.");
            new a(e)
        }, registerDialog: function (e) {
            e.showModal || r.forceRegisterDialog(e)
        }, DialogManager: function () {
            this.pendingDialogStack = [];
            var e = this.checkDOM_.bind(this);
            this.overlay = document.createElement("div"), this.overlay.className = "_dialog_overlay", this.overlay.addEventListener("click", function (t) {
                this.forwardTab_ = void 0, t.stopPropagation(), e([])
            }.bind(this)), this.handleKey_ = this.handleKey_.bind(this), this.handleFocus_ = this.handleFocus_.bind(this), this.zIndexLow_ = 1e5, this.zIndexHigh_ = 100150, this.forwardTab_ = void 0, "MutationObserver" in window && (this.mo_ = new MutationObserver(function (t) {
                var o = [];
                t.forEach(function (e) {
                    for (var t, n = 0; t = e.removedNodes[n]; ++n) t instanceof Element && ("dialog" === t.localName && o.push(t), o = o.concat(t.querySelectorAll("dialog")))
                }), o.length && e(o)
            }))
        }
    };
    if (r.DialogManager.prototype.blockDocument = function () {
        document.documentElement.addEventListener("focus", this.handleFocus_, !0), document.addEventListener("keydown", this.handleKey_), this.mo_ && this.mo_.observe(document, {
            childList: !0,
            subtree: !0
        })
    }, r.DialogManager.prototype.unblockDocument = function () {
        document.documentElement.removeEventListener("focus", this.handleFocus_, !0), document.removeEventListener("keydown", this.handleKey_), this.mo_ && this.mo_.disconnect()
    }, r.DialogManager.prototype.updateStacking = function () {
        for (var e, t = this.zIndexHigh_, o = 0; e = this.pendingDialogStack[o]; ++o) e.updateZIndex(--t, --t), 0 === o && (this.overlay.style.zIndex = --t);
        var n = this.pendingDialogStack[0];
        n ? (n.dialog.parentNode || document.body).appendChild(this.overlay) : this.overlay.parentNode && this.overlay.parentNode.removeChild(this.overlay)
    }, r.DialogManager.prototype.containedByTopDialog_ = function (e) {
        for (; e = t(e);) {
            for (var o, n = 0; o = this.pendingDialogStack[n]; ++n) if (o.dialog === e) return 0 === n;
            e = e.parentElement
        }
        return !1
    }, r.DialogManager.prototype.handleFocus_ = function (e) {
        if (!this.containedByTopDialog_(e.target) && document.activeElement !== document.documentElement && (e.preventDefault(), e.stopPropagation(), o(e.target), void 0 !== this.forwardTab_)) {
            var t = this.pendingDialogStack[0];
            return t.dialog.compareDocumentPosition(e.target) & Node.DOCUMENT_POSITION_PRECEDING && (this.forwardTab_ ? t.focus_() : e.target !== document.documentElement && document.documentElement.focus()), !1
        }
    }, r.DialogManager.prototype.handleKey_ = function (t) {
        if (this.forwardTab_ = void 0, 27 === t.keyCode) {
            t.preventDefault(), t.stopPropagation();
            var o = new e("cancel", {bubbles: !1, cancelable: !0}), n = this.pendingDialogStack[0];
            n && n.dialog.dispatchEvent(o) && n.dialog.close()
        } else 9 === t.keyCode && (this.forwardTab_ = !t.shiftKey)
    }, r.DialogManager.prototype.checkDOM_ = function (e) {
        this.pendingDialogStack.slice().forEach(function (t) {
            -1 !== e.indexOf(t.dialog) ? t.downgradeModal() : t.maybeHideModal()
        })
    }, r.DialogManager.prototype.pushDialog = function (e) {
        return !(this.pendingDialogStack.length >= (this.zIndexHigh_ - this.zIndexLow_) / 2 - 1 || (1 === this.pendingDialogStack.unshift(e) && this.blockDocument(), this.updateStacking(), 0))
    }, r.DialogManager.prototype.removeDialog = function (e) {
        var t = this.pendingDialogStack.indexOf(e);
        -1 !== t && (this.pendingDialogStack.splice(t, 1), 0 === this.pendingDialogStack.length && this.unblockDocument(), this.updateStacking())
    }, r.dm = new r.DialogManager, r.formSubmitter = null, r.useValue = null, void 0 === window.HTMLDialogElement) {
        var l = document.createElement("form");
        if (l.setAttribute("method", "dialog"), "dialog" !== l.method) {
            var d = Object.getOwnPropertyDescriptor(HTMLFormElement.prototype, "method");
            if (d) {
                var s = d.get;
                d.get = function () {
                    return i(this) ? "dialog" : s.call(this)
                };
                var c = d.set;
                d.set = function (e) {
                    return "string" == typeof e && "dialog" === e.toLowerCase() ? this.setAttribute("method", e) : c.call(this, e)
                }, Object.defineProperty(HTMLFormElement.prototype, "method", d)
            }
        }
        document.addEventListener("click", function (e) {
            if (r.formSubmitter = null, r.useValue = null, !e.defaultPrevented) {
                var o = e.target;
                if (o && i(o.form)) {
                    if (!("submit" === o.type && ["button", "input"].indexOf(o.localName) > -1)) {
                        if ("input" !== o.localName || "image" !== o.type) return;
                        r.useValue = e.offsetX + "," + e.offsetY
                    }
                    t(o) && (r.formSubmitter = o)
                }
            }
        }, !1);
        var u = HTMLFormElement.prototype.submit;
        HTMLFormElement.prototype.submit = function () {
            if (!i(this)) return u.call(this);
            var e = t(this);
            e && e.close()
        }, document.addEventListener("submit", function (e) {
            var o = e.target;
            if (i(o)) {
                e.preventDefault();
                var n = t(o);
                if (n) {
                    var a = r.formSubmitter;
                    a && a.form === o ? n.close(r.useValue || a.value) : n.close(), r.formSubmitter = null
                }
            }
        }, !0)
    }

    function h() {
        for (var e = allHelp.AllRepos.sort(), t = document.getElementById("repo"); t.length > 1;) t.removeChild(t.lastChild);
        var o = function n(e) {
            return function t(e) {
                var t, o, n = {};
                try {
                    for (var i = __values(e.split("&").map(function (e) {
                        return e.split("=").map(unescape)
                    })), a = i.next(); !a.done; a = i.next()) {
                        var r = __read(a.value, 2);
                        n[r[0]] = r[1]
                    }
                } catch (e) {
                    t = {error: e}
                } finally {
                    try {
                        a && !a.done && (o = i.return) && o.call(i)
                    } finally {
                        if (t) throw t.error
                    }
                }
                return n
            }(location.search.substr(1))[e]
        }("repo");
        e.forEach(function (e) {
            var n = document.createElement("option");
            n.text = e, n.selected = !(!o || e !== o), t.appendChild(n)
        })
    }

    function p(e, t) {
        if ("" === e) {
            var o = t[""];
            return o ? o.sort() : []
        }
        var n = t[e.split("/")[0]], i = [], a = t[e];
        a && a !== [] && (i = i.concat(a));
        return n && n.forEach(function (e) {
            i.includes(e) || i.push(e)
        }), i.sort()
    }

    function m(e, t, o) {
        var n;
        void 0 === t && (t = []), void 0 === o && (o = !1);
        var i, a = document.createElement("td");
        return a.classList.add("mdl-data-table__cell--non-numeric"), o || a.classList.add("table-cell"), Array.isArray(e) ? ((i = document.createElement("ul")).classList.add("command-example-list"), e.forEach(function (e) {
            var o, n = document.createElement("li"), a = document.createElement("span");
            a.innerHTML = e, (o = a.classList).add.apply(o, __spread(t)), n.appendChild(a), i.appendChild(n)
        })) : ((n = (i = document.createElement("div")).classList).add.apply(n, __spread(t)), i.innerHTML = e), a.appendChild(i), a
    }

    function f(e, t, o, n, i) {
        var a, r;
        void 0 === o && (o = []), void 0 === n && (n = ""), void 0 === i && (i = !1);
        var l = document.createElement("i");
        l.id = "icon-" + t + "-" + e, l.classList.add("material-icons"), (a = l.classList).add.apply(a, __spread(o)), l.innerHTML = t;
        var d = i ? document.createElement("button") : document.createElement("div");
        if (d.appendChild(l), i && (r = d.classList).add.apply(r, __spread(["mdl-button", "mdl-js-button", "mdl-button--icon"])), "" === n) return d;
        var s = document.createElement("div");
        return s.setAttribute("for", l.id), s.classList.add("mdl-tooltip"), s.innerHTML = n, d.appendChild(s), d
    }

    function g(e, t) {
        var o = document.createElement("div"), n = document.createElement("h5"), i = document.createElement("p");
        return i.classList.add("dialog-section-body"), i.innerHTML = t, n.classList.add("dialog-section-title"), n.innerHTML = e, o.classList.add("dialog-section"), o.appendChild(n), o.appendChild(i), o
    }


    function v() {
        var e = function getRepo(e) {
            var repo;
            e && (repo = unescape(e.split('=')[1]));
            return repo
        }(location.search.substr(1));
        var o = new Map;
        p(e, allHelp.RepoPlugins).forEach(function (e) {
            allHelp.PluginHelp[e] && allHelp.PluginHelp[e].Commands && o.set(e, {
                isExternal: !1,
                plugin: allHelp.PluginHelp[e]
            })
        }), p(e, allHelp.RepoExternalPlugins).forEach(function (e) {
            allHelp.ExternalPluginHelp[e] && allHelp.ExternalPluginHelp[e].Commands && o.set(e, {
                isExternal: !0,
                plugin: allHelp.ExternalPluginHelp[e]
            })
        }), function n(e, t) {
            var o, n, i = document.getElementById("command-table"), a = document.querySelector("tbody");
            if (0 !== t.size) {
                for (i.style.display = "table"; 0 !== a.childElementCount;) a.removeChild(a.firstChild);
                var r = Array.from(t.keys()), l = [], d = function (e) {
                    t.get(e).plugin.Commands.forEach(function (t) {
                        l.push({command: t, pluginName: e})
                    })
                };
                try {
                    for (var s = __values(r), c = s.next(); !c.done; c = s.next()) d(c.value)
                } catch (e) {
                    o = {error: e}
                } finally {
                    try {
                        c && !c.done && (n = s.return) && n.call(s)
                    } finally {
                        if (o) throw o.error
                    }
                }
                l.sort(function (e, t) {
                    return e.command.Featured ? -1 : t.command.Featured ? 1 : 0
                }).forEach(function (o, n) {
                    var i = o.pluginName, r = t.get(i), l = function d(e, t, o, n, i, a) {
                        var r = document.createElement("tr"), l = function d(e) {
                            var t = e.split(" ");
                            if (!t || 0 === t.length) throw new Error("Cannot extract command name.");
                            return t[0].slice(1).split("-").join("_")
                        }(n.Examples[0]);
                        return r.id = l, r.appendChild(function s(e, t, o) {
                            var n = document.createElement("td");
                            return n.classList.add("mdl-data-table__cell--non-numeric"), e && n.appendChild(f(o, "stars", ["featured-icon"], "Featured command")), t && n.appendChild(f(o, "open_in_new", ["external-icon"], "External plugin")), n
                        }(n.Featured, i, a)), r.appendChild(m(n.Usage, ["command-usage"])), r.appendChild(m(n.Examples, ["command-examples"], !0)), r.appendChild(m(n.Description, ["command-desc-text"])), r.appendChild(m(n.WhoCanUse, ["command-desc-text"])), r.appendChild(function c(e, t, o) {
                            var n = document.createElement("td"), i = document.createElement("button");
                            n.classList.add("mdl-data-table__cell--non-numeric"), i.classList.add("mdl-button", "mdl-button--js", "mdl-button--primary"), i.innerHTML = t;
                            var a = document.querySelector("dialog");
                            return i.addEventListener("click", function () {
                                for (var n = a.querySelector(".mdl-dialog__title"), i = a.querySelector(".mdl-dialog__content"); i.firstChild;) i.removeChild(i.firstChild);
                                if (n.innerHTML = t, o.Description && i.appendChild(g("Description", o.Description)), o.Events) {
                                    var r = "[" + o.Events.sort().join(", ") + "]";
                                    i.appendChild(g("Events handled", r))
                                }
                                o.Config && (r = o.Config ? o.Config[e] : "") && "" !== r && i.appendChild(g("" === e ? "Configuration(global)" : "Configuration(" + e + ")", r)), a.showModal()
                            }), n.appendChild(i), n
                        }(e, t, o)), r.appendChild(function u(e, t) {
                            var o = document.createElement("td"), n = f(t, "link", ["link-icon"], "", !0);
                            return n.addEventListener("click", function () {
                                var t = document.createElement("input"), o = window.location.href, n = o.indexOf("#");
                                -1 !== n && (o = o.slice(0, n)), o += "#" + e, t.style.zIndex = "-99999", t.style.background = "transparent", t.value = o, document.body.appendChild(t), t.select(), document.execCommand("copy"), document.body.removeChild(t), document.body.querySelector("#toast").MaterialSnackbar.showSnackbar({message: "Copied to clipboard"})
                            }), o.appendChild(n), o.classList.add("mdl-data-table__cell--non-numeric"), o
                        }(l, a)), r
                    }(e, i, r.plugin, o.command, r.isExternal, n);
                    a.appendChild(l)
                })
            } else i.style.display = "none"
        }(e, o)
    }

    window.onload = function () {
        var e = window.location.hash;
        v();
        var t = document.querySelector("dialog");
        if (r.registerDialog(t), t.querySelector(".close").addEventListener("click", function () {
            t.close()
        }), "" !== e) {
            var o = document.body.querySelector(e), n = document.body.querySelector(".mdl-layout__content");
            o && n && (setTimeout(function () {
                n.scrollTop = o.getBoundingClientRect().top, window.location.hash = e
            }, 32), o.querySelector(".mdl-button--primary").click())
        }
    }, window.redraw = v
}();
