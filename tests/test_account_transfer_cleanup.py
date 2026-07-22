import sys
import types
import unittest
from unittest.mock import MagicMock, patch

try:
    import httpx  # noqa: F401
except ModuleNotFoundError:
    httpx_stub = types.ModuleType("httpx")
    httpx_stub.Client = object
    httpx_stub.HTTPError = Exception
    sys.modules["httpx"] = httpx_stub

from grok2api.pool import accounts
from grok2api.store import accounts_pg


class AccountTransferCleanupTest(unittest.TestCase):
    def test_pg_cleanup_deletes_exact_id_missing_from_cached_map(self):
        backend = MagicMock()
        backend.enabled.return_value = True
        backend.delete_accounts.return_value = {
            "removed": ["new-account"],
            "missing": [],
        }
        with (
            patch.object(accounts, "read_auth_map", return_value={}),
            patch.object(accounts, "_cleanup_account_side_state") as cleanup,
            patch("grok2api.store.accounts_pg", backend),
        ):
            result = accounts.remove_accounts(["new-account"])

        backend.delete_accounts.assert_called_once_with(["new-account"])
        cleanup.assert_called_once_with(["new-account"])
        self.assertEqual(result["removed"], ["new-account"])
        self.assertEqual(result["missing"], [])

    def test_pg_delete_accounts_deduplicates_and_reports_missing(self):
        cursor = MagicMock()
        rowcounts = iter([1, 0])

        def execute(query, _params):
            if query.startswith("DELETE FROM accounts"):
                cursor.rowcount = next(rowcounts)

        cursor.execute.side_effect = execute
        connection = MagicMock()
        connection.__enter__.return_value.cursor.return_value.__enter__.return_value = cursor
        with (
            patch.object(accounts_pg, "connection", return_value=connection),
            patch.object(accounts_pg, "invalidate_auth_map_cache") as invalidate,
        ):
            result = accounts_pg.delete_accounts(["acc-1", "acc-1", "acc-2", ""])

        self.assertEqual(result["removed"], ["acc-1"])
        self.assertEqual(result["missing"], ["acc-2"])
        self.assertEqual(result["requested"], 2)
        connection.__enter__.return_value.commit.assert_called_once_with()
        invalidate.assert_called_once_with()


if __name__ == "__main__":
    unittest.main()
