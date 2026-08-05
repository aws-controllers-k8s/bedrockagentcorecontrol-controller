"""Pytest configuration for isolated Harness configuration tests."""

import sys
from pathlib import Path


sys.path.insert(0, str(Path(__file__).parents[1] / "e2e"))
