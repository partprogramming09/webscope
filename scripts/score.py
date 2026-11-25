#!/usr/bin/env python3
"""Score of a WEBSCOPE_ACTIONLOG file: how many calls worked and why the others failed.

Usage: python3 -I scripts/score.py <actionlog> [more logs]
Reads only the lines "webscope: call <tool> <action> ok|failed <n>ms [bytes=<n>] [err=<slug>]" (older lines without bytes or err count too).
Prints the success rate, the failures by cause, the share of failures a setup or a wrong call causes (everything but `other`),
the bytes of an ok answer and the calls to help. A failure inside the page (a helper that cannot load) runs in the browser and is not in the log.
"""
import collections
import re
import sys

LINE = re.compile(r"webscope: call (\S+) (\S+) (ok|failed) (\d+)ms(?: bytes=(\d+))?(?: err=(\w+))?")


def score(lines):
    total = ok = helps = 0
    kinds = collections.Counter()
    ok_bytes = []
    for line in lines:
        m = LINE.search(line)
        if not m:
            continue
        _, action, status, _, size, kind = m.groups()
        total += 1
        helps += action == "help"
        if status == "ok":
            ok += 1
            if size is not None:
                ok_bytes.append(int(size))
        else:
            kinds[kind or "unclassified"] += 1
    return total, ok, kinds, ok_bytes, helps


def main(paths):
    lines = []
    for p in paths:
        with open(p, encoding="utf-8", errors="replace") as f:
            lines += f.readlines()
    total, ok, kinds, ok_bytes, helps = score(lines)
    if total == 0:
        print("no call lines found: set WEBSCOPE_ACTIONLOG to a file and use webscope first")
        return 1
    failed = total - ok
    avoidable = sum(n for k, n in kinds.items() if k not in ("other", "unclassified"))
    print(f"calls: {total}  ok: {ok}  failed: {failed}  success rate: {100 * ok / total:.0f} %")
    print(f"failures a setup or a wrong call causes: {avoidable} of {total} ({100 * avoidable / total:.0f} %)")
    for k, n in kinds.most_common():
        print(f"  {k:15} {n}")
    if ok_bytes:
        print(f"bytes of an ok answer: {sum(ok_bytes) // len(ok_bytes)} on average over {len(ok_bytes)} calls")
    print(f"calls to help: {helps}")
    return 0


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(2)
    sys.exit(main(sys.argv[1:]))
