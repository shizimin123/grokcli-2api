import sys
import unittest
from pathlib import Path
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "grok-build-auth"))

from xconsole_client.solver import YesCaptchaSolver


class TurnstileProxyTest(unittest.TestCase):
    def test_local_task_carries_registration_proxy(self):
        solver = YesCaptchaSolver(
            "local",
            endpoint="http://127.0.0.1:5072",
            auto_fallback_endpoint=False,
        )
        captured = {}

        def create_task(task):
            captured.update(task)
            return "task-1"

        with (
            patch.object(solver, "_create_task", side_effect=create_task),
            patch.object(
                solver,
                "_get_result",
                return_value={"status": "ready", "solution": {"token": "ok"}},
            ),
        ):
            token = solver.solve_turnstile(
                "https://accounts.x.ai/sign-up",
                "0x-test",
                proxy="http://192.0.2.10:7890",
            )

        self.assertEqual(token, "ok")
        self.assertEqual(captured["proxy"], "http://192.0.2.10:7890")


if __name__ == "__main__":
    unittest.main()
