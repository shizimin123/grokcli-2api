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
            patch.object(
                sub2,
                "upsert_grok_oauth_account",
                return_value={
                    "account": {"id": 99},
                    "action": "created",
                    "credentials_updated": True,
                    "deduplicated": 0,
                },
            ) as upsert,
        ):
            result = sub2.push_account("acc-1", cfg=cfg)
        self.assertTrue(result["ok"])
        self.assertEqual(result["group_id"], 12)
        self.assertEqual(result["proxy_id"], 7)
        self.assertEqual(upsert.call_args.kwargs["proxy_id"], 7)

    def test_upsert_preserves_fresher_remote_and_deactivates_duplicate(self):
        cfg = {"account_concurrency": 1}
        matches = [
            {
                "id": 20,
                "name": "user@example.com",
                "platform": "grok",
                "status": "active",
                "credentials": {"expires_at": "2026-07-22T20:00:00Z"},
            },
            {
                "id": 10,
                "name": "user@example.com",
                "platform": "grok",
                "status": "active",
                "credentials": {"expires_at": "2026-07-22T18:00:00Z"},
            },
        ]
        with (
            patch.object(sub2, "list_grok_accounts_by_name", return_value=matches),
            patch.object(
                sub2, "_write_grok_account", return_value={"id": 20}
            ) as write,
        ):
            result = sub2.upsert_grok_oauth_account(
                name="user@example.com",
                group_id=12,
                access_token="older-access",
                refresh_token="older-refresh",
                email="user@example.com",
                expires_at="2026-07-22T19:00:00Z",
                proxy_id=7,
                cfg=cfg,
            )

        self.assertEqual(result["action"], "updated")
        self.assertFalse(result["credentials_updated"])
        self.assertEqual(result["deduplicated"], 1)
        update_body = write.call_args_list[0].args[2]
        self.assertNotIn("credentials", update_body)
        self.assertEqual(write.call_args_list[0].args[1], "/api/v1/admin/accounts/20")
        self.assertEqual(
            write.call_args_list[1].args,
            (
                "PUT",
                "/api/v1/admin/accounts/10",
                {"status": "inactive"},
                cfg,
            ),
        )

    def test_upsert_recovers_error_only_with_newer_credentials(self):
        cfg = {"account_concurrency": 1}
        matches = [
            {
                "id": 30,
                "name": "user@example.com",
                "platform": "grok",
                "status": "error",
                "credentials": {"expires_at": "2026-07-22T18:00:00Z"},
            }
        ]
        with (
            patch.object(sub2, "list_grok_accounts_by_name", return_value=matches),
            patch.object(
                sub2, "_write_grok_account", return_value={"id": 30}
            ) as write,
        ):
            result = sub2.upsert_grok_oauth_account(
                name="user@example.com",
                group_id=12,
                access_token="new-access",
                refresh_token="new-refresh",
                email="user@example.com",
                expires_at="2026-07-22T20:00:00Z",
                proxy_id=7,
                cfg=cfg,
            )

        self.assertTrue(result["credentials_updated"])
        self.assertEqual(write.call_args_list[0].args[2]["status"], "active")
        self.assertIn("credentials", write.call_args_list[0].args[2])
        self.assertEqual(
            write.call_args_list[1].args[1],
            "/api/v1/admin/accounts/30/clear-error",
        )


if __name__ == "__main__":
    unittest.main()
