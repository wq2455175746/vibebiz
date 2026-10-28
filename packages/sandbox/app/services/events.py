import json

DONE = "data: [DONE]\n\n"


def sse(payload: dict) -> str:
    return f"data: {json.dumps(payload, ensure_ascii=False, separators=(',', ':'))}\n\n"
