from app.services.chat_turns import TurnCollector


def test_collector_sums_assistant_usage():
    collector = TurnCollector()
    collector.consume(
        {
            "type": "agent",
            "payload": {
                "type": "assistant",
                "message": {"content": [{"type": "text", "text": "a"}]},
                "usage": {"input_tokens": 10, "output_tokens": 2},
            },
        }
    )
    collector.consume(
        {
            "type": "agent",
            "payload": {
                "type": "assistant",
                "message": {"content": [{"type": "text", "text": "b"}]},
                "usage": {"inputTokens": 5, "outputTokens": 3},
            },
        }
    )

    assert collector.usage() == {"inputTokens": 15, "outputTokens": 5, "modelRequests": 2}


def test_collector_falls_back_to_result_usage():
    collector = TurnCollector()
    collector.consume(
        {
            "type": "agent",
            "payload": {
                "type": "result",
                "usage": {"input_tokens": 7, "output_tokens": 4},
            },
        }
    )

    assert collector.usage() == {"inputTokens": 7, "outputTokens": 4, "modelRequests": 0}


def test_collector_falls_back_to_model_usage():
    collector = TurnCollector()
    collector.consume(
        {
            "type": "agent",
            "payload": {
                "type": "result",
                "modelUsage": {
                    "a": {"inputTokens": 3, "outputTokens": 2},
                    "b": {"inputTokens": 4, "outputTokens": 1},
                },
            },
        }
    )

    assert collector.usage() == {"inputTokens": 7, "outputTokens": 3, "modelRequests": 0}
