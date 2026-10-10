import unittest
from unittest.mock import patch
from subprocess import CompletedProcess

from release_tag import reserve_tag, select_tag


class ReleaseTagTests(unittest.TestCase):
    def test_initial_version(self):
        self.assertEqual(select_tag([], "new"), ("v1.0.0", False))

    def test_increment_highest_stable_version(self):
        tags = [("v1.0.9", "old"), ("v1.0.10", "old"),
                ("v2.0.0-rc.1", "old"), ("main-42", "old")]
        self.assertEqual(select_tag(tags, "new"), ("v1.0.11", False))
        self.assertEqual(select_tag([("v2.3.4", "old")], "new"), ("v2.3.5", False))

    def test_reuse_same_commit(self):
        self.assertEqual(select_tag([("v1.0.0", "same")], "same"), ("v1.0.0", True))

    @patch("release_tag.subprocess.run")
    def test_retry_concurrent_reservation(self, run):
        run.side_effect = [
            CompletedProcess([], 0, "", ""),
            CompletedProcess([], 1, "", "HTTP 422"),
            CompletedProcess([], 0, "v1.0.0\tother\n", ""),
            CompletedProcess([], 0, "", ""),
        ]
        self.assertEqual(reserve_tag("owner/repo", "new"), "v1.0.1")
        self.assertIn("ref=refs/tags/v1.0.1", run.call_args.args[0])


if __name__ == "__main__":
    unittest.main()
