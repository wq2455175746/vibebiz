(function (global) {
  var api = typeof wx !== 'undefined' ? wx : tt;
  var canvas = api.createCanvas();
  var ctx = canvas.getContext('2d');
  global.window = global;
  global.canvas = canvas;
  global.document = {
    getElementById: function () { return canvas; },
    querySelector: function () { return canvas; },
    querySelectorAll: function () { return [canvas]; },
    createElement: function (tag) {
      if (tag === 'canvas') return canvas;
      return { style: {}, appendChild: function () {}, addEventListener: function () {} };
    },
    addEventListener: function () {},
    body: { appendChild: function () {}, style: {} }
  };
  global.navigator = { userAgent: 'minigame' };
  global.devicePixelRatio = api.getSystemInfoSync().pixelRatio;
  global.innerWidth = canvas.width;
  global.innerHeight = canvas.height;
  global.requestAnimationFrame = api.requestAnimationFrame || function (cb) {
    return setTimeout(function () { cb(Date.now()); }, 16);
  };
  global.addEventListener = function () {};
  global.localStorage = {
    getItem: function (k) { return api.getStorageSync(k) || null; },
    setItem: function (k, v) { api.setStorageSync(k, v); },
    removeItem: function (k) { api.removeStorageSync(k); }
  };
  global.Image = function () { return api.createImage(); };
  global.fetch = function (url, options) {
    return new Promise(function (resolve, reject) {
      api.request({
        url: url, method: (options && options.method) || 'GET',
        data: options && options.body, success: resolve, fail: reject
      });
    });
  };
  window.__channelAdapter = true;
})(typeof GameGlobal !== 'undefined' ? GameGlobal : globalThis);
