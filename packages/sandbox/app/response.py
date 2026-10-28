from typing import Any


def envelope(result: Any = None, code: int = 0, message: str = "ok") -> dict:
    return {"code": code, "message": message, "result": result}
