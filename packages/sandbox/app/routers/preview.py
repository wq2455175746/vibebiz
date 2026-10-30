from fastapi import APIRouter, Request
from fastapi.responses import FileResponse, HTMLResponse, Response

from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["preview"])

PREVIEW_CONSOLE_BRIDGE = """<script>
(function () {
  if (window.parent === window) return;
  var sent = 0;
  function text(value) {
    if (typeof value === "string") return value;
    if (value instanceof Error) return value.stack || value.message;
    try {
      return JSON.stringify(value);
    } catch (_) {
      return String(value);
    }
  }
  function send(level, args) {
    if (sent++ > 1000) return;
    var message = Array.prototype.slice.call(args).map(text).join(" ");
    if (message.length > 4000) message = message.slice(0, 4000) + "...";
    try {
      window.parent.postMessage(
        { type: "vibebiz-preview-log", level: level, message: message },
        "*",
      );
    } catch (_) {}
  }
  var originalGetContext = HTMLCanvasElement.prototype.getContext;
  HTMLCanvasElement.prototype.getContext = function (type, attributes) {
    var options = attributes ? Object.assign({}, attributes) : {};
    if (type === "webgl" || type === "webgl2" || type === "experimental-webgl") {
      options.preserveDrawingBuffer = true;
    }
    return originalGetContext.call(this, type, options);
  };
  ["log", "info", "warn", "error", "debug"].forEach(function (method) {
    var original = console[method];
    if (typeof original !== "function") return;
    console[method] = function () {
      send(method === "debug" ? "info" : method, arguments);
      return original.apply(console, arguments);
    };
  });
  window.addEventListener("error", function (event) {
    send("error", [event.message + " (" + (event.filename || "") + ":" + (event.lineno || 0) + ")"]);
  });
  window.addEventListener("unhandledrejection", function (event) {
    send("error", ["Unhandled rejection: " + text(event.reason)]);
  });
})();
</script>"""


def _inject_console_bridge(content: str) -> str:
    marker = "</head>"
    if marker in content:
        return content.replace(marker, PREVIEW_CONSOLE_BRIDGE + marker, 1)
    marker = "<body"
    if marker in content:
        return content.replace(marker, PREVIEW_CONSOLE_BRIDGE + marker, 1)
    return PREVIEW_CONSOLE_BRIDGE + content


@router.get("/{project_id}/preview")
@router.get("/{project_id}/preview/{path:path}")
def preview(project_id: str, request: Request, path: str = "") -> Response:
    workspace: Workspace = request.app.state.workspace
    target = workspace.preview_path(project_id, path)
    headers = {"Cache-Control": "no-store"}
    if target.suffix.lower() in {".html", ".htm"}:
        content = target.read_text(encoding="utf-8", errors="replace")
        return HTMLResponse(_inject_console_bridge(content), headers=headers)
    return FileResponse(target, headers=headers)
