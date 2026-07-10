#!/usr/bin/env python3
"""Regression tests for the public workspace boundary checker."""

from __future__ import annotations

import unittest

from check_public_contracts import ALLOWED_INTERNAL_EDGES, WORKSPACE_PACKAGES, validate_workspace_edges


class WorkspaceBoundaryTests(unittest.TestCase):
    def test_approved_layering_is_accepted(self) -> None:
        self.assertEqual(validate_workspace_edges(set(WORKSPACE_PACKAGES), set(ALLOWED_INTERNAL_EDGES)), [])

    def test_inverted_report_to_engine_edge_is_rejected(self) -> None:
        inverted = set(ALLOWED_INTERNAL_EDGES)
        inverted.add(("steward-report", "steward-engine"))
        errors = validate_workspace_edges(set(WORKSPACE_PACKAGES), inverted)
        self.assertTrue(any("steward-report" in error and "steward-engine" in error for error in errors))


if __name__ == "__main__":
    unittest.main()
