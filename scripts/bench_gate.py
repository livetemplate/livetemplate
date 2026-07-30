#!/usr/bin/env python3
"""CI benchmark regression gate.

Reads `benchstat -format csv` output (the comparison of the committed
baseline against the current run) and gates on the machine-independent
metrics — B/op and allocs/op — for the critical benchmark families.

sec/op is deliberately NOT gated: the committed baseline may come from a
different machine (the current one is arm64; CI is amd64), and wall-time
deltas across machines are meaningless. sec/op comparisons still appear in
the posted PR comment as informational.

Deltas are computed from benchstat's median-value columns directly rather
than its "vs base" column: benchstat prints "~" for deltas below its
significance threshold, and with the run counts CI can afford (n≤3) the
Mann-Whitney test can NEVER reach significance — every delta would read
"~" and a grep-based gate passes vacuously. B/op and allocs/op are
deterministic, so comparing medians directly is exact.

Usage: bench_gate.py <comparison.csv>

Prints `status=(pass|warn|fail)` and `message=...` lines for the workflow
to forward into $GITHUB_OUTPUT, plus a human-readable offender table.
Exits 1 on fail (including the fail-closed case: a parse that yields no
gated critical rows means the gate is broken, not that the code is fast).
"""

import csv
import re
import sys

CRITICAL = re.compile(
    r"^(E2E|Template|CompareTrees|RangeDiff|PrepareTree|Composite|TopicFanout"
    r"|TriggerAction|Upload_|Redis|ChatAppend|WideTable|LargeDoc)"
)
# Honest-variant benches share family tokens but must not gate:
# Loopback = real-socket fidelity checks, EnqueueOnly = deliberately hollow
# contrast benches, RealRedis = env-gated integration variant.
EXCLUDED = re.compile(r"Loopback|EnqueueOnly|RealRedis")
GATED_METRICS = ("B/op", "allocs/op")
WARN_PCT = 10.0
FAIL_PCT = 20.0

CPU_SUFFIX = re.compile(r"-\d+$")


def parse(path):
    """Yield (name, metric, old, new) for every gated-metric benchmark row."""
    metric = None
    with open(path, newline="") as f:
        for rec in csv.reader(f):
            if len(rec) < 2 or not rec[1]:
                continue
            if rec[0] == "":  # header row: either file names or a metric header
                # Reset on every header so rows of an unrecognized metric
                # section can never be mis-attributed to a gated one.
                metric = rec[1] if rec[1] in GATED_METRICS else None
                continue
            if rec[0] == "geomean" or metric is None or len(rec) < 4:
                continue
            try:
                old, new = float(rec[1]), float(rec[3])
            except ValueError:
                continue  # bench present on only one side, or malformed
            yield CPU_SUFFIX.sub("", rec[0]), metric, old, new


def main():
    if len(sys.argv) != 2:
        print("usage: bench_gate.py <comparison.csv>", file=sys.stderr)
        return 2

    offenders = []  # (delta_pct, name, metric, old, new)
    gated_rows = 0
    for name, metric, old, new in parse(sys.argv[1]):
        if not CRITICAL.match(name) or EXCLUDED.search(name):
            continue
        if old <= 0:
            continue
        gated_rows += 1
        delta = (new - old) / old * 100
        if delta > WARN_PCT:
            offenders.append((delta, name, metric, old, new))

    if gated_rows == 0:
        # Fail closed: a gate that matches nothing is a broken gate
        # (format drift, renamed benchmarks without a baseline regen, or an
        # empty comparison), not a green build.
        print("status=fail")
        print("message=Benchmark gate parsed 0 critical rows — gate is broken, not passing (regenerate baseline or fix the parse)")
        return 1

    offenders.sort(reverse=True)
    for delta, name, metric, old, new in offenders:
        print(f"  {name} {metric}: {old:.0f} -> {new:.0f} ({delta:+.1f}%)")

    worst = offenders[0][0] if offenders else 0.0
    if worst > FAIL_PCT:
        print("status=fail")
        print(f"message=Critical allocation regression >{FAIL_PCT:.0f}% "
              f"(worst {worst:+.1f}%: {offenders[0][1]} {offenders[0][2]}) across {gated_rows} gated rows")
        return 1
    if worst > WARN_PCT:
        print("status=warn")
        print(f"message=Allocation regression >{WARN_PCT:.0f}% "
              f"(worst {worst:+.1f}%: {offenders[0][1]} {offenders[0][2]}) across {gated_rows} gated rows")
        return 0
    print("status=pass")
    print(f"message=No significant allocation regressions across {gated_rows} gated rows (B/op, allocs/op)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
