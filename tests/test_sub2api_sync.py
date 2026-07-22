import sys
import types
import unittest
from pathlib import Path
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

import grok2api.pool as pool_package

accounts_stub = types.ModuleType("grok2api.pool.accounts")
accounts_stub.read_auth_map = lambda: {}
previous_accounts_module = sys.modules.get("grok2api.pool.accounts")
previous_accounts_attr = getattr(pool_package, "accounts", None)
sys.modules["grok2api.pool.accounts"] = accounts_stub
pool_package.accounts = accounts_stub
try:
    from grok2api.upstream import sub2api_client as sub2
finally:
    if previous_accounts_module is None:
        sys.modules.pop("grok2api.pool.accounts", None)
    else:
        sys.modules["grok2api.pool.accounts"] = previous_accounts_module
    if previous_accounts_attr is None:
        delattr(pool_package, "accounts")
    else:
        pool_package.accounts = previous_accounts_attr


class Sub2APISyncTest(unittest.TestCase):
    def test_resolve_group_creates_missing_default(self):
        cfg = {"group_name": "grokcli-2api", "auto_create_group": True}
        with (
            patch.object(sub2, "list_groups", return_value=[]),
            patch.object(sub2, "create_group", return_value={"id": 12}) as create,
            patch.object(sub2, "set_sub2api_config"),
        ):
            self.assertEqual(sub2.resolve_group_id(cfg), 12)
        create.assert_called_once_with("grokcli-2api", platform="grok", cfg=cfg)

    def test_push_account_matches_proxy_and_assigns_group(self):
        cfg = {"group_id": 12, "notes_prefix": "g2a"}
        entry = {
            "email": "user@example.com",
            "access_token": "token",
            "proxy": "http://192.0.2.10:7890",
        }
        with (
            patch.object(sub2, "_local_account_entry", return_value=("acc-1", entry)),
            patch.object(
                sub2,
                "list_proxies",
                return_value=[{"id": 7, "protocol": "http", "host": "192.0.2.10", "port": 7890}],
            ),
            patch.object(sub2, "create_grok_oauth_account", return_value={"id": 99}) as create,
        ):
            result = sub2.push_account("acc-1", cfg=cfg)
        self.assertTrue(result["ok"])
        self.assertEqual(result["group_id"], 12)
        self.assertEqual(result["proxy_id"], 7)
        self.assertEqual(create.call_args.kwargs["proxy_id"], 7)


if __name__ == "__main__":
    unittest.main()
