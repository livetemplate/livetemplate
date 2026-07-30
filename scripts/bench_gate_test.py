#!/usr/bin/env python3
"""Self-test for bench_gate.py against a captured benchstat CSV fixture.

The gate's correctness rests on the exact row/column layout of
`benchstat -format csv` (pinned in .github/workflows/benchmark.yml). This
fixture was captured from that pinned benchstat
(golang.org/x/perf v0.0.0-20260709024250-82a0b07e230d), so a format change
in a future benchstat bump breaks THIS test with a clear message instead of
silently starving the gate of rows mid-PR. The CI workflow runs this file
before every gated comparison.

Run directly: python3 scripts/bench_gate_test.py
"""

import os
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import bench_gate

# Captured `benchstat -format csv` output. Embeds three scenarios at once:
# a +28.9% allocs/op regression on a gated bench (CompositeUpdate 97→125),
# a large regression on an EXCLUDED bench (TopicFanout_LoopbackWS), and
# unchanged gated rows. sec/op and wireB/op sections must be ignored.
FIXTURE_FAIL = """\
,old.txt,,new.txt,,,
,sec/op,CI,sec/op,CI,vs base,P
CompositeUpdate-8,3.1e-05,∞,3.1e-05,∞,~,p=1.000 n=1
TopicFanout_FullPipeline/N=100-8,0.0024,∞,0.0024,∞,~,p=1.000 n=1
geomean,5.34e-05,,5.34e-05,,+0.00%,

,old.txt,,new.txt,,,
,wireB/op,CI,wireB/op,CI,vs base,P
CompositeUpdate-8,78.7,∞,78.7,∞,~,p=1.000 n=1
TopicFanout_FullPipeline/N=100-8,5977,∞,5977,∞,~,p=1.000 n=1
geomean,685.8,,685.8,,+0.00%,

,old.txt,,new.txt,,,
,B/op,CI,B/op,CI,vs base,P
CompositeUpdate-8,6300,∞,6300,∞,~,p=1.000 n=1
TopicFanout_FullPipeline/N=100-8,574000,∞,574000,∞,~,p=1.000 n=1
TopicFanout_LoopbackWS-8,70000,∞,70000,∞,~,p=1.000 n=1
TemplateExecute/subsequent-render-8,3000,∞,3000,∞,~,p=1.000 n=1
Parse/simple-8,5248,∞,5248,∞,~,p=1.000 n=1
geomean,20897.4,,20897.4,,+0.00%,

,old.txt,,new.txt,,,
,allocs/op,CI,allocs/op,CI,vs base,P
CompositeUpdate-8,97,∞,125,∞,~,p=1.000 n=1
TopicFanout_FullPipeline/N=100-8,9229,∞,9229,∞,~,p=1.000 n=1
TopicFanout_LoopbackWS-8,1072,∞,9999,∞,~,p=1.000 n=1
TemplateExecute/subsequent-render-8,61,∞,61,∞,~,p=1.000 n=1
Parse/simple-8,47,∞,47,∞,~,p=1.000 n=1
geomean,307.5,,505.7,,+64.43%,
"""

FIXTURE_PASS = FIXTURE_FAIL.replace(",97,∞,125,", ",97,∞,97,")
FIXTURE_WARN = FIXTURE_FAIL.replace(",97,∞,125,", ",97,∞,108,")


class BenchGateTest(unittest.TestCase):
    def run_gate(self, csv_text):
        with tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False) as f:
            f.write(csv_text)
            path = f.name
        try:
            proc = subprocess.run(
                [sys.executable, bench_gate.__file__, path],
                capture_output=True, text=True)
        finally:
            os.unlink(path)
        status = ""
        for line in proc.stdout.splitlines():
            if line.startswith("status="):
                status = line.split("=", 1)[1]
        return status, proc.returncode, proc.stdout

    def test_parse_extracts_only_gated_metric_rows(self):
        with tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False) as f:
            f.write(FIXTURE_FAIL)
            path = f.name
        try:
            rows = list(bench_gate.parse(path))
        finally:
            os.unlink(path)
        metrics = {metric for _, metric, _, _ in rows}
        self.assertEqual(metrics, {"B/op", "allocs/op"},
                         "sec/op and custom metrics must not be yielded")
        names = {name for name, _, _, _ in rows}
        self.assertIn("CompositeUpdate", names, "-8 suffix must be stripped")
        self.assertNotIn("geomean", names)
        self.assertEqual(len(rows), 10)

    def test_gated_regression_fails(self):
        status, code, out = self.run_gate(FIXTURE_FAIL)
        self.assertEqual(status, "fail", out)
        self.assertEqual(code, 1)
        self.assertIn("CompositeUpdate", out, "offender must be named")

    def test_warn_band(self):
        status, code, _ = self.run_gate(FIXTURE_WARN)
        self.assertEqual(status, "warn")
        self.assertEqual(code, 0)

    def test_unchanged_passes_and_excluded_regression_is_ignored(self):
        # FIXTURE_PASS still contains the Loopback 1072→9999 regression;
        # the excluded variant must not trip the gate.
        status, code, _ = self.run_gate(FIXTURE_PASS)
        self.assertEqual(status, "pass")
        self.assertEqual(code, 0)

    def test_empty_input_fails_closed(self):
        status, code, _ = self.run_gate("")
        self.assertEqual(status, "fail")
        self.assertEqual(code, 1)


if __name__ == "__main__":
    unittest.main(verbosity=1)
